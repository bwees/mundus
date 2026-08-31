package webapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-fuego/fuego"
	"github.com/golang-jwt/jwt/v5"

	"github.com/bwees/mundus/server/internal/auth"
	"github.com/bwees/mundus/server/internal/config"
	"github.com/bwees/mundus/server/internal/robot"
	"github.com/bwees/mundus/server/internal/robotapi"
	"github.com/bwees/mundus/server/internal/system"
	"github.com/bwees/mundus/server/internal/update"
)

type stubMQTT struct{}

func (stubMQTT) CurrentMQTT() config.MQTTSettings { return config.MQTTSettings{} }
func (stubMQTT) MQTTConnected() bool              { return false }
func (stubMQTT) Reconfigure(config.MQTTSettings)  {}
func (stubMQTT) RoomsChanged()                    {}

// stubProps stands in for the local-cloud settings backend, which is the only
// thing that can deliver a property change to the robot. It records writes so
// tests can round-trip a setting.
type stubProps struct{ values map[int]any }

func newStubProps() stubProps { return stubProps{values: map[int]any{}} }

func (p stubProps) GetInt(id, def int) int {
	if v, ok := p.values[id].(int); ok {
		return v
	}
	if v, ok := p.values[id].(bool); ok {
		if v {
			return 1
		}
		return 0
	}
	return def
}

func (p stubProps) GetBool(id int, def bool) bool {
	if v, ok := p.values[id].(bool); ok {
		return v
	}
	if v, ok := p.values[id].(int); ok {
		return v != 0
	}
	return def
}

func (p stubProps) Set(id int, v any) error { p.values[id] = v; return nil }

func testServer(t *testing.T) (*fuego.Server, Deps) {
	t.Helper()
	store, err := auth.Load(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	d := Deps{
		Auth:     store,
		Security: fuego.NewSecurity(),
		API:      &robotapi.API{},
		Props:    newStubProps(),
		Robot:    &robot.Robot{},
		Rooms:    func() []robot.Room { return nil },
		MQTT:     stubMQTT{},
		System:   &system.System{},
		Update:   &update.Updater{},
		MapDir:   t.TempDir(),
		Log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	return NewServer(d), d
}

// routesFromSpec reads the generated spec so a newly added route is covered by
// this test automatically. Routes registered with OptionHide() are absent from
// the spec, so they must be listed here by hand.
func routesFromSpec(t *testing.T) map[string][]string {
	t.Helper()
	data, err := os.ReadFile("../../openapi.json")
	if err != nil {
		t.Fatalf("read spec (run `go run . -openapi`): %v", err)
	}
	var spec struct {
		Paths map[string]map[string]any `json:"paths"`
	}
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatal(err)
	}
	out := map[string][]string{}
	for path, methods := range spec.Paths {
		for m := range methods {
			out[path] = append(out[path], strings.ToUpper(m))
		}
	}
	// Hidden from the spec, and the most dangerous route in the server.
	out["/api/update/upload"] = []string{"POST"}
	return out
}

func TestEveryAPIRouteRequiresAuth(t *testing.T) {
	srv, _ := testServer(t)
	routes := routesFromSpec(t)
	// Guards against a truncated or stale spec silently passing this test.
	if len(routes) < 20 {
		t.Fatalf("only %d routes found; spec looks stale", len(routes))
	}

	for path, methods := range routes {
		for _, method := range methods {
			t.Run(method+" "+path, func(t *testing.T) {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(method, path, strings.NewReader("{}"))
				req.Header.Set("Content-Type", "application/json")
				srv.Mux.ServeHTTP(rec, req)

				// A public route may still answer 401 from its own handler
				// (a bad password); only the guard's message means it was
				// blocked before reaching the handler.
				blocked := rec.Code == http.StatusUnauthorized &&
					strings.Contains(rec.Body.String(), "authentication required")
				if publicRoutes[path] {
					if blocked {
						t.Errorf("public route blocked by the auth guard")
					}
					return
				}
				if !blocked {
					t.Errorf("reachable without a token: got %d %s", rec.Code, rec.Body.String())
				}
			})
		}
	}
}

// A form-encoded POST is CORS-simple, so a browser will send it cross-origin
// without a preflight. It must still be rejected without a token.
func TestFormEncodedPostRequiresAuth(t *testing.T) {
	srv, _ := testServer(t)
	for _, path := range []string{"/api/system/ssh/keys", "/api/update/apply", "/api/update/upload"} {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("POST", path, strings.NewReader("Key=ssh-ed25519+AAAA+pwn"))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			srv.Mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("got %d, want 401", rec.Code)
			}
		})
	}
}

func TestValidTokenPassesTheGuard(t *testing.T) {
	srv, d := testServer(t)
	token, err := d.Security.GenerateToken(jwt.MapClaims{"sub": "admin"})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/rooms", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	srv.Mux.ServeHTTP(rec, req)
	if rec.Code == http.StatusUnauthorized {
		t.Errorf("valid token rejected: %s", rec.Body.String())
	}
}

func TestGarbageTokenRejected(t *testing.T) {
	srv, _ := testServer(t)
	for _, header := range []string{"Bearer ", "Bearer not.a.token", "Basic abc", "nonsense"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/rooms", nil)
		req.Header.Set("Authorization", header)
		srv.Mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("header %q: got %d, want 401", header, rec.Code)
		}
	}
}

func post(t *testing.T, srv *fuego.Server, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	srv.Mux.ServeHTTP(rec, req)
	return rec
}

func tokenOf(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var out TokenDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode token from %q: %v", rec.Body.String(), err)
	}
	if out.Token == "" {
		t.Fatalf("no token in %q", rec.Body.String())
	}
	return out.Token
}

func TestSetupThenLoginFlow(t *testing.T) {
	srv, _ := testServer(t)

	rec := httptest.NewRecorder()
	srv.Mux.ServeHTTP(rec, httptest.NewRequest("GET", "/api/auth/status", nil))
	var status AuthStatusDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if !status.SetupRequired {
		t.Fatal("fresh device did not report setup_required")
	}

	if rec := post(t, srv, "/api/auth/setup", `{"password":"short"}`, ""); rec.Code < 400 {
		t.Errorf("short password accepted: %d", rec.Code)
	}

	token := tokenOf(t, post(t, srv, "/api/auth/setup", `{"password":"correct-horse"}`, ""))

	rec = httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/rooms", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	srv.Mux.ServeHTTP(rec, req)
	if rec.Code == http.StatusUnauthorized {
		t.Error("token from setup was not accepted")
	}

	// The setup route is unauthenticated; it must not re-key a live device.
	if rec := post(t, srv, "/api/auth/setup", `{"password":"attacker-pw"}`, ""); rec.Code < 400 {
		t.Errorf("second setup accepted: %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	srv.Mux.ServeHTTP(rec, httptest.NewRequest("GET", "/api/auth/status", nil))
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.SetupRequired {
		t.Error("still reports setup_required after setup")
	}

	if rec := post(t, srv, "/api/auth/login", `{"password":"wrong"}`, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong password: got %d, want 401", rec.Code)
	}
	tokenOf(t, post(t, srv, "/api/auth/login", `{"password":"correct-horse"}`, ""))
}

// An unconfigured device must not be drivable. Setup is the only thing an
// anonymous caller can do, and it is a one-shot claim rather than a way in.
func TestNothingIsControllableBeforeSetup(t *testing.T) {
	srv, d := testServer(t)
	if d.Auth.Configured() {
		t.Fatal("a fresh store reports itself configured")
	}
	for _, c := range []struct{ method, path string }{
		{"GET", "/api/state"},
		{"POST", "/api/clean"},
		{"POST", "/api/dock"},
		{"POST", "/api/stop"},
		{"PUT", "/api/settings"},
		{"GET", "/api/config/mqtt"},
		{"PUT", "/api/config/mqtt"},
		{"PUT", "/api/system/ssh"},
		{"POST", "/api/system/ssh/keys"},
		{"POST", "/api/update/apply"},
		{"POST", "/api/update/upload"},
		{"POST", "/api/map/split"},
	} {
		rec := post(t, srv, c.path, "{}", "")
		if c.method != "POST" {
			rec = httptest.NewRecorder()
			req := httptest.NewRequest(c.method, c.path, strings.NewReader("{}"))
			req.Header.Set("Content-Type", "application/json")
			srv.Mux.ServeHTTP(rec, req)
		}
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s answered %d before setup, want 401", c.method, c.path, rec.Code)
		}
	}
}

// The web UI calls this on load to decide whether a stored token is still good,
// so it has to answer 401 for a stale one rather than succeeding.
func TestSessionEndpointReflectsTokenValidity(t *testing.T) {
	srv, d := testServer(t)

	rec := httptest.NewRecorder()
	srv.Mux.ServeHTTP(rec, httptest.NewRequest("GET", "/api/auth/session", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("no token: got %d, want 401", rec.Code)
	}

	rec = httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/auth/session", nil)
	req.Header.Set("Authorization", "Bearer not.a.real.token")
	srv.Mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("garbage token: got %d, want 401", rec.Code)
	}

	token, err := d.Security.GenerateToken(jwt.MapClaims{"sub": "admin"})
	if err != nil {
		t.Fatal(err)
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/auth/session", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	srv.Mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("valid token: got %d, want 200", rec.Code)
	}

	// A token signed by a different server instance, which is what every mundus
	// restart produces, since the signing key only lives in memory.
	other := fuego.NewSecurity()
	stale, err := other.GenerateToken(jwt.MapClaims{"sub": "admin"})
	if err != nil {
		t.Fatal(err)
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/auth/session", nil)
	req.Header.Set("Authorization", "Bearer "+stale)
	srv.Mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("token from a previous run: got %d, want 401", rec.Code)
	}
}

func cookieNamed(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range (&http.Response{Header: rec.Header()}).Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// The token now travels as a cookie the browser attaches by itself. That is
// only safe because SameSite=Strict stops it being attached cross-site, which
// is what previously made CSRF impossible when it rode in a header.
func TestLoginSetsHardenedSessionCookie(t *testing.T) {
	srv, _ := testServer(t)
	if rec := post(t, srv, "/api/auth/setup", `{"password":"owner-password"}`, ""); rec.Code != http.StatusOK {
		t.Fatalf("setup: %d", rec.Code)
	}

	rec := post(t, srv, "/api/auth/login", `{"password":"owner-password"}`, "")
	c := cookieNamed(rec, "mundus_session")
	if c == nil {
		t.Fatal("login set no session cookie")
	}
	if c.Value == "" {
		t.Error("session cookie is empty")
	}
	if !c.HttpOnly {
		t.Error("cookie is readable from JavaScript")
	}
	if c.SameSite != http.SameSiteStrictMode {
		t.Errorf("SameSite is %v, want Strict; CSRF is open without it", c.SameSite)
	}
	if c.Path != "/" {
		t.Errorf("cookie path %q, want /", c.Path)
	}
	// The device serves plain HTTP, so a Secure cookie would never be stored.
	if c.Secure {
		t.Error("cookie is Secure, which stops it being stored over plain HTTP")
	}
}

func TestCookieAuthenticatesRequests(t *testing.T) {
	srv, _ := testServer(t)
	setup := post(t, srv, "/api/auth/setup", `{"password":"owner-password"}`, "")
	c := cookieNamed(setup, "mundus_session")
	if c == nil {
		t.Fatal("setup set no cookie")
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/auth/session", nil)
	req.AddCookie(c)
	srv.Mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("cookie was not accepted: %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/auth/session", nil)
	req.AddCookie(&http.Cookie{Name: "mundus_session", Value: "not.a.token"})
	srv.Mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("garbage cookie accepted: %d", rec.Code)
	}
}

func TestLogoutExpiresTheCookie(t *testing.T) {
	srv, _ := testServer(t)
	rec := post(t, srv, "/api/auth/logout", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("logout without a session: %d, want 200", rec.Code)
	}
	c := cookieNamed(rec, "mundus_session")
	if c == nil {
		t.Fatal("logout set no cookie")
	}
	if c.MaxAge >= 0 {
		t.Errorf("MaxAge is %d, want negative so the browser drops it", c.MaxAge)
	}
}
