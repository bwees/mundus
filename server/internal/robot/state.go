package robot

import (
	"encoding/json"
	"strings"
)

type State struct {
	HAState        string // cleaning|docked|paused|idle|returning|error
	Battery        int    // 0-100, -1 if unknown
	FanSpeed       string // min|medium|high|max, "" if unknown
	RunState       string // curr_running_state verbatim, e.g. "Sleeping" (attribute)
	WorkingStatus  int    // srv.state numeric WorkingStatus enum
	CloudConnected bool   // cloud_conn.is_connected
	Charging       bool
	Docked         bool
	ErrorCode      int // 0 if none/unknown
}

type brainStatus struct {
	BatteryPercent   *int   `json:"battery_percent"`
	FanLevel         *int   `json:"fan_level"`
	CurrRunningState string `json:"curr_running_state"`
	IsCharging       bool   `json:"is_charging"`
	IsOnBase         bool   `json:"is_on_base"`
	IsOnChargeBase   bool   `json:"is_on_charge_base"`
	IsFanFault       bool   `json:"is_fan_fault"`
}

type iotState struct {
	Srv struct {
		State int `json:"state"`
	} `json:"srv"`
	CloudConn struct {
		IsConnected bool `json:"is_connected"`
	} `json:"cloud_conn"`
}

// WorkingStatus (property 1010) -> HA vacuum state. srv.state==21
// (Sleeping->idle) is the only value confirmed on device.
func workingStatusToHA(ws int) string {
	switch ws {
	case 2, 3, 36:
		return "docked"
	case 4, 5, 6, 8, 9, 10, 26, 35:
		return "cleaning"
	case 11, 32:
		return "paused"
	case 14, 15, 24, 29, 37:
		return "returning"
	case 13:
		return "error"
	default:
		return "idle"
	}
}

// runStateToHA classifies the curr_running_state string. Used only as a fallback
// when the numeric WorkingStatus is unavailable; substring-based so it tolerates
// state names we have not observed. "Sleeping" is confirmed; the rest are inferred
// from the brain state-machine vocabulary.
func runStateToHA(s string) string {
	l := strings.ToLower(s)
	switch {
	case strings.Contains(l, "fault"), strings.Contains(l, "error"):
		return "error"
	case strings.Contains(l, "pause"):
		return "paused"
	case strings.Contains(l, "back"), strings.Contains(l, "return"), strings.Contains(l, "recharg"):
		return "returning"
	case strings.Contains(l, "sweep"), strings.Contains(l, "mop"), strings.Contains(l, "clean"),
		strings.Contains(l, "explor"), strings.Contains(l, "wetting"), strings.Contains(l, "mark"):
		return "cleaning"
	case strings.Contains(l, "charg"), strings.Contains(l, "dock"):
		return "docked"
	default:
		return "idle"
	}
}

// WORK_STATUS_DRYING_MOP: srv.state while the base is drying the mop.
const WorkStatusDryingMop = 20

var FanSpeedOptions = []string{"Quiet", "Standard", "Strong", "Max"}

func FanLevelToHA(level int) string {
	switch level {
	case 1:
		return "Quiet"
	case 2:
		return "Standard"
	case 3:
		return "Strong"
	case 4:
		return "Max"
	default:
		return ""
	}
}

func FanSpeedToLevel(speed string) (int, bool) {
	switch speed {
	case "Quiet":
		return 1, true
	case "Standard":
		return 2, true
	case "Strong":
		return 3, true
	case "Max":
		return 4, true
	default:
		return 0, false
	}
}

// decodeJSON finds the first JSON object in the terminal output and decodes it.
// The terminal may emit log/whitespace around the payload; a Decoder tolerates
// trailing bytes after the object.
func decodeJSON(out string, v any) bool {
	start := strings.IndexByte(out, '{')
	if start < 0 {
		return false
	}
	dec := json.NewDecoder(strings.NewReader(out[start:]))
	return dec.Decode(v) == nil
}

func parseBrainStatus(out string) (brainStatus, bool) {
	var b brainStatus
	if !decodeJSON(out, &b) {
		return b, false
	}
	return b, true
}

func parseIotState(out string) (iotState, bool) {
	var s iotState
	if !decodeJSON(out, &s) {
		return s, false
	}
	return s, true
}
