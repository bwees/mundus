package robotapi

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// The on-disk shape is the vendor's: a bare array of
// [x, y, theta, ms, segment, action] entries (read off device W1106000).
const trackFixture = `[
[0.065,-0.028,1.704,12906,0,0],
[0.021,-0.049,1.789,24488,1,8],
[0.021,-0.049,1.789,25059,2,20],
[0.025,-0.075,1.819,25156,2,20]
]`

func TestReadTrackFlattensPositions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "track.json")
	if err := os.WriteFile(path, []byte(trackFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadTrack(path)
	if err != nil {
		t.Fatal(err)
	}
	// The third entry repeats the second position and is dropped.
	want := []float64{0.065, -0.028, 0.021, -0.049, 0.025, -0.075}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ReadTrack = %v, want %v", got, want)
	}
}

// No task has run yet: the file is simply absent, and the map view must still load.
func TestReadTrackMissingFileIsEmpty(t *testing.T) {
	got, err := ReadTrack(filepath.Join(t.TempDir(), "track.json"))
	if err != nil {
		t.Fatalf("missing track reported an error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("ReadTrack = %v, want empty", got)
	}
}
