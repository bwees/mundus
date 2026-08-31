// Package settings is the one definition of the device settings mundus exposes.
// The Home Assistant entities and the web API are both built from this table, so
// a name, option label or stored value can only be wrong in one place.

package settings

import "fmt"

type Kind string

const (
	Toggle Kind = "toggle"
	Choice Kind = "choice"
	Number Kind = "number"
)

type Option struct {
	Label string `json:"label"`
	Value int    `json:"value"`
}

type Setting struct {
	Key     string   `json:"key"`
	Name    string   `json:"name"`
	Icon    string   `json:"icon"`
	Kind    Kind     `json:"kind"`
	Prop    int      `json:"prop"`
	Options []Option `json:"options,omitempty"`
	Min     int      `json:"min,omitempty"`
	Max     int      `json:"max,omitempty"`
	Default int      `json:"default"`

	// inverted marks a property stored as the opposite of what it displays.
	// Read and Write hide it, so Default is always the displayed value and no
	// caller has to think about which way round the property is stored.
	inverted bool
}

// Props is the part of robotapi.Properties this package needs.
type Props interface {
	GetInt(id, def int) int
	GetBool(id int, def bool) bool
	Set(id int, v any) error
}

// Read returns the current value, with a toggle as 0 or 1.
func (s Setting) Read(p Props) int {
	if s.Kind != Toggle {
		return p.GetInt(s.Prop, s.Default)
	}
	stored := p.GetBool(s.Prop, (s.Default != 0) != s.inverted)
	if stored != s.inverted {
		return 1
	}
	return 0
}

func (s Setting) Write(p Props, v int) error {
	if s.Kind == Toggle {
		return p.Set(s.Prop, (v != 0) != s.inverted)
	}
	if s.Kind == Choice && !s.HasValue(v) {
		return fmt.Errorf("%s: %d is not one of its options", s.Key, v)
	}
	if s.Kind == Number && (v < s.Min || v > s.Max) {
		return fmt.Errorf("%s: %d is outside %d-%d", s.Key, v, s.Min, s.Max)
	}
	return p.Set(s.Prop, v)
}

func (s Setting) HasValue(v int) bool {
	for _, o := range s.Options {
		if o.Value == v {
			return true
		}
	}
	return false
}

// Labels lists the option labels in order, for the Home Assistant select.
func (s Setting) Labels() []string {
	out := make([]string, len(s.Options))
	for i, o := range s.Options {
		out[i] = o.Label
	}
	return out
}

func (s Setting) LabelOf(value int) string {
	for _, o := range s.Options {
		if o.Value == value {
			return o.Label
		}
	}
	if len(s.Options) > 0 {
		return s.Options[0].Label
	}
	return ""
}

func (s Setting) ValueOf(label string) (int, bool) {
	for _, o := range s.Options {
		if o.Label == label {
			return o.Value, true
		}
	}
	return 0, false
}

func toggle(key, name, icon string, prop int, def bool) Setting {
	s := Setting{Key: key, Name: name, Icon: icon, Kind: Toggle, Prop: prop}
	if def {
		s.Default = 1
	}
	return s
}

func choice(key, name, icon string, prop int, options ...Option) Setting {
	return Setting{
		Key: key, Name: name, Icon: icon, Kind: Choice, Prop: prop,
		Options: options, Default: options[0].Value,
	}
}

func All() []Setting {
	childLock := toggle("child_lock", "Child Lock", "mdi:lock", 1057, true)
	childLock.inverted = true

	return []Setting{
		toggle("auto_empty", "Auto-Empty Dust", "mdi:delete-empty", 1062, false),
		toggle("auto_dry_after_wash", "Auto-Dry After Wash", "mdi:hair-dryer", 1068, false),
		toggle("dampen_mop", "Dampen Mop First", "mdi:water-percent", 1112, false),
		toggle("auto_resume", "Auto-Resume After Charge", "mdi:play-pause", 1074, false),
		toggle("carpet_boost", "Carpet Boost", "mdi:rug", 1060, false),
		toggle("ai_obstacle", "AI Obstacle Avoidance", "mdi:robot", 1070, false),
		childLock,

		choice("smart_dust", "Smart Dust Collection", "mdi:fan", 1063,
			Option{"Normal", 0}, Option{"Fast", 1}, Option{"Super", 2}),
		choice("dust_freq", "Dust Collection Frequency", "mdi:timer-sand", 1103,
			Option{"Default", 0}, Option{"15 min", 15}, Option{"20 min", 20}),
		choice("dry_duration", "Drying Duration", "mdi:timer", 1069,
			Option{"2 h", 2}, Option{"3 h", 3}, Option{"4 h", 4}),
		choice("carpet_clean", "Carpet Behavior", "mdi:rug", 1061,
			Option{"Adapt", 0}, Option{"Avoid", 1}),
		choice("ai_mode", "AI Avoidance Mode", "mdi:robot-outline", 1113,
			Option{"Standard", 0}, Option{"Agile", 1}),

		{
			Key: "volume", Name: "Volume", Icon: "mdi:volume-high", Kind: Number,
			Prop: 1039, Min: 0, Max: 100, Default: 50,
		},
	}
}
