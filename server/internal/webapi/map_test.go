package webapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"

	"github.com/bwees/mundus/server/internal/robotapi"
)

// A room shaped like a real one: an irregular polygon read off the device.
var fixtureRoom = []float64{
	1.35, 4.425, 1.4, 4.375, 2.4, 4.375, 2.4, 3.775, 2.45, 3.725, 2.55, 3.725,
	2.55, 3.375, 2.7, 3.275, 2.7, 3.175, 2.5, 3.025, 2.3, 3.025, 2.15, 2.875,
	2.3, 2.775, 2.3, 2.575, 2.5, 2.475, 2.5, 0.875, 3.05, 0.775, 3.05, -1.125,
	2.5, -1.075, 2.5, -0.575, 1.85, -0.575, 1.7, -0.675, 1.7, -0.425, 0.15, -0.725,
	0.1, -1.075, -0.4, -0.925, -0.4, -0.775, -0.25, -0.575, -0.65, -0.525, -0.65, 0.625,
	-0.05, 0.575, 0.0, 0.225, 0.575, -0.2, 0.625, -0.6, 0.625, 1.325, -0.05, 1.375,
	0.0, 1.425, 2.175, 0.2, 2.225, 0.25, 2.225, 2.775, 2.975, 0.0, 3.025, -0.3,
	2.875, -0.55, 2.875, -0.6, 3.825, -0.2, 3.925, -0.15, 4.275, -0.1, 4.275, 0.45,
	3.825, 0.5, 3.825, 1.05, 4.375, 1.15, 4.425, 1.35,
}

func writeMapFixture(t *testing.T, dir string) {
	t.Helper()
	const w, h = 210, 219
	pgm := append([]byte(fmt.Sprintf("P5\n%d %d\n255\n", w, h)), make([]byte, w*h)...)
	write(t, filepath.Join(dir, "map.pgm"), pgm)
	write(t, filepath.Join(dir, "map.json"), []byte(`{"image":"map.pgm","resolution":0.05,"origin":[-4.525,-2.25,0.0]}`))

	labels := robotapi.Labels{
		Data: []robotapi.RoomLabel{{
			ColorType: 0, Geometry: fixtureRoom, Graph: []int{},
			ID: "ROOM_001", Name: "Living", Type: "polygon",
		}},
		UUID: "cd84ae5a-d6b4-4c2f-8157-f9f7bc8c547f", Version: "online_3.0.1",
	}
	data, err := json.Marshal(labels)
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "labels.json"), data)
}

func write(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// Every rejection the map routes can produce must reach the browser as a 4xx
// carrying its reason; a bare 500 with an empty body is what this guards against.
func TestMapInputErrorsAreReadable(t *testing.T) {
	srv, d := testServer(t)
	writeMapFixture(t, d.MapDir)
	token, err := d.Security.GenerateToken(jwt.MapClaims{"sub": "admin"})
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name, path, body, want string
	}{
		{"split line misses the room", "/api/map/split",
			`{"id":"ROOM_001","line":[40,40,40,50],"new_name":"K"}`,
			"split line does not divide the room"},
		{"split unknown room", "/api/map/split",
			`{"id":"ROOM_404","line":[1,0,1,4],"new_name":"K"}`,
			"not found"},
		{"merge needs two rooms", "/api/map/merge",
			`{"ids":["ROOM_001"],"name":"K"}`,
			"at least two rooms"},
		{"zone needs three points", "/api/map/zone",
			`{"kind":"no_go","geometry":[0,0,1,1],"name":"Z"}`,
			"at least 3 points"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := post(t, srv, c.path, c.body, token)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("got %d, want %d", rec.Code, http.StatusBadRequest)
			}
			if !strings.Contains(rec.Body.String(), c.want) {
				t.Errorf("body %q does not explain the failure (want %q)", rec.Body.String(), c.want)
			}
		})
	}
}

// An internal failure must still say what went wrong; fuego serializes an
// untyped error to "{}" unless the engine's error handler wraps it.
func TestInternalErrorsCarryTheirMessage(t *testing.T) {
	srv, d := testServer(t)
	token, err := d.Security.GenerateToken(jwt.MapClaims{"sub": "admin"})
	if err != nil {
		t.Fatal(err)
	}
	rec := post(t, srv, "/api/map/split", `{"id":"ROOM_001","line":[1,0,1,4],"new_name":"K"}`, token)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("got %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rec.Body.String(), "labels.json") {
		t.Errorf("body %q hides the cause", rec.Body.String())
	}
}
