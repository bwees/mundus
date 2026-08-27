package webapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-fuego/fuego"
	"github.com/golang-jwt/jwt/v5"

	"github.com/bwees/mundus/server/internal/settings"
)

func authed(t *testing.T, srv *fuego.Server, d Deps, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	token, err := d.Security.GenerateToken(jwt.MapClaims{"sub": "admin"})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	srv.Mux.ServeHTTP(rec, req)
	return rec
}

func getSettings(t *testing.T, srv *fuego.Server, d Deps) SettingsDTO {
	t.Helper()
	rec := authed(t, srv, d, "GET", "/api/settings", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/settings: %d %s", rec.Code, rec.Body.String())
	}
	var out SettingsDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// The web UI renders entirely from this payload, so every setting has to arrive
// with a value and every choice with its options.
func TestSettingsResponseCarriesSchemaAndValues(t *testing.T) {
	srv, d := testServer(t)
	got := getSettings(t, srv, d)

	if len(got.Schema) != len(settings.All()) {
		t.Fatalf("schema has %d entries, want %d", len(got.Schema), len(settings.All()))
	}
	for _, s := range got.Schema {
		if _, ok := got.Values[s.Key]; !ok {
			t.Errorf("%s has no value", s.Key)
		}
		if s.Name == "" {
			t.Errorf("%s has no display name", s.Key)
		}
		if s.Kind == settings.Choice && len(s.Options) == 0 {
			t.Errorf("%s is a choice with no options", s.Key)
		}
	}
}

func TestSettingsRoundTripThroughTheAPI(t *testing.T) {
	srv, d := testServer(t)

	body := `{"values":{"carpet_clean":1,"volume":73,"child_lock":0,"smart_dust":2}}`
	if rec := authed(t, srv, d, "PUT", "/api/settings", body); rec.Code != http.StatusOK {
		t.Fatalf("PUT /api/settings: %d %s", rec.Code, rec.Body.String())
	}

	values := getSettings(t, srv, d).Values
	for key, want := range map[string]int{"carpet_clean": 1, "volume": 73, "child_lock": 0, "smart_dust": 2} {
		if values[key] != want {
			t.Errorf("%s read back %d, want %d", key, values[key], want)
		}
	}
}

func TestSettingsRejectsBadInput(t *testing.T) {
	srv, d := testServer(t)
	for name, body := range map[string]string{
		"unknown key":         `{"values":{"not_a_setting":1}}`,
		"value not an option": `{"values":{"carpet_clean":7}}`,
		"number out of range": `{"values":{"volume":9000}}`,
	} {
		t.Run(name, func(t *testing.T) {
			if rec := authed(t, srv, d, "PUT", "/api/settings", body); rec.Code != http.StatusBadRequest {
				t.Errorf("got %d, want 400: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

// Home Assistant and the web UI now read the same table, so the labels that
// drifted apart before cannot drift again.
func TestServedLabelsMatchTheSharedTable(t *testing.T) {
	srv, d := testServer(t)
	served := map[string]settings.Setting{}
	for _, s := range getSettings(t, srv, d).Schema {
		served[s.Key] = s
	}
	for _, want := range settings.All() {
		got, ok := served[want.Key]
		if !ok {
			t.Errorf("%s missing from the API", want.Key)
			continue
		}
		if got.Name != want.Name {
			t.Errorf("%s served as %q, table says %q", want.Key, got.Name, want.Name)
		}
		for i, o := range want.Options {
			if got.Options[i] != o {
				t.Errorf("%s option %d served as %+v, table says %+v", want.Key, i, got.Options[i], o)
			}
		}
	}
}
