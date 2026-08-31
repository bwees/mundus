package robot

import "fmt"

type CleanMode struct {
	Type       string `json:"type"`        // sweep|mop|sweep_mop|first_sweep_then_mop
	FanLevel   int    `json:"fan_level"`   // 1-4
	WaterLevel int    `json:"water_level"` // 0-3
	Times      int    `json:"times"`       // 1-2
}

func DefaultCleanMode() CleanMode {
	return CleanMode{Type: "sweep_mop", FanLevel: 2, WaterLevel: 1, Times: 1}
}

// Validate rejects a mode the firmware would not accept, so a bad value is
// refused at the edge rather than persisted as the robot's clean mode.
func (m CleanMode) Validate() error {
	if _, ok := cleanTypeLabel[m.Type]; !ok {
		return fmt.Errorf("unknown clean type %q", m.Type)
	}
	if m.FanLevel < 1 || m.FanLevel > 4 {
		return fmt.Errorf("fan level %d is outside 1-4", m.FanLevel)
	}
	if m.WaterLevel < 1 || m.WaterLevel > 3 {
		return fmt.Errorf("water level %d is outside 1-3", m.WaterLevel)
	}
	if m.Times < 1 || m.Times > 2 {
		return fmt.Errorf("passes %d is outside 1-2", m.Times)
	}
	return nil
}

// Clean-mode selects: option labels shown in HA <-> the wire values the robot
// expects in a CleanMode. Labels use the user-facing "vacuum" vocabulary; wire
// values are the firmware's sweep/mop enum.
var (
	CleanTypeOptions = []string{"Vacuum", "Mop", "Vacuum and Mop", "Vacuum then Mop"}
	cleanTypeWire    = map[string]string{
		"Vacuum":          "sweep",
		"Mop":             "mop",
		"Vacuum and Mop":  "sweep_mop",
		"Vacuum then Mop": "first_sweep_then_mop",
	}
	cleanTypeLabel = map[string]string{
		"sweep":                "Vacuum",
		"mop":                  "Mop",
		"sweep_mop":            "Vacuum and Mop",
		"first_sweep_then_mop": "Vacuum then Mop",
	}

	WaterLevelOptions = []string{"Low", "Medium", "High"}
	waterLevelValue   = map[string]int{"Low": 1, "Medium": 2, "High": 3}
	waterLevelLabel   = map[int]string{1: "Low", 2: "Medium", 3: "High"}

	PassesOptions = []string{"1", "2"}
)

func CleanTypeWire(label string) (string, bool) { v, ok := cleanTypeWire[label]; return v, ok }

func CleanTypeLabel(wire string) string {
	if l, ok := cleanTypeLabel[wire]; ok {
		return l
	}
	return "Vacuum and Mop"
}

func WaterLevelValue(label string) (int, bool) { v, ok := waterLevelValue[label]; return v, ok }

func WaterLevelLabel(v int) string {
	if l, ok := waterLevelLabel[v]; ok {
		return l
	}
	return "Low"
}

func PassesValue(o string) (int, bool) {
	switch o {
	case "1":
		return 1, true
	case "2":
		return 2, true
	default:
		return 0, false
	}
}

func PassesLabel(times int) string {
	if times == 2 {
		return "2"
	}
	return "1"
}
