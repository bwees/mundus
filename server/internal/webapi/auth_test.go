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
		Props:    robotapi.NewProperties(filepath.Join(t.TempDir(), "props.json")),
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
