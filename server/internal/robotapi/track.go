package robotapi

import (
	"encoding/json"
	"os"
)

// ReadTrack reads control_center's pose trail for the active map. On disk each
// entry is [x, y, theta, ms, segment, action]; only the position is returned,
// flattened to [x,y,x,y,…] world metres so it shares the frame with room and
// zone geometry. Consecutive duplicate positions (the robot holding still) are
// dropped. A missing file means no task has run yet, which is not an error.
func ReadTrack(path string) ([]float64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var poses [][]float64
	if err := json.Unmarshal(data, &poses); err != nil {
		return nil, err
	}
	out := make([]float64, 0, len(poses)*2)
	for _, p := range poses {
		if len(p) < 2 {
			continue
		}
		if n := len(out); n >= 2 && out[n-2] == p[0] && out[n-1] == p[1] {
			continue
		}
		out = append(out, p[0], p[1])
	}
	return out, nil
}
