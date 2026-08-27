package hass

import (
	"fmt"
	"strconv"

	"github.com/bwees/mundus/server/internal/settings"
)

func (t topics) settingCmd(key string) string   { return t.base + "/setting/" + key + "/set" }
func (t topics) settingState(key string) string { return t.base + "/setting/" + key + "/state" }

func (b *Bridge) settings() []settings.Setting {
	if b.props == nil {
		return nil
	}
	return settings.All()
}

// settingPayload renders a setting as the payload its HA entity expects.
func (b *Bridge) settingPayload(s settings.Setting) string {
	v := s.Read(b.props)
	switch s.Kind {
	case settings.Toggle:
		if v != 0 {
			return "ON"
		}
		return "OFF"
	case settings.Choice:
		return s.LabelOf(v)
	default:
		return strconv.Itoa(v)
	}
}

func (b *Bridge) applySetting(s settings.Setting, payload string) error {
	switch s.Kind {
	case settings.Toggle:
		return s.Write(b.props, boolToInt(payload == "ON"))
	case settings.Choice:
		v, ok := s.ValueOf(payload)
		if !ok {
			return fmt.Errorf("unknown option %q", payload)
		}
		return s.Write(b.props, v)
	default:
		v, err := strconv.Atoi(payload)
		if err != nil {
			return err
		}
		return s.Write(b.props, v)
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// component maps a setting onto the HA discovery component that renders it.
func component(k settings.Kind) string {
	switch k {
	case settings.Toggle:
		return "switch"
	case settings.Choice:
		return "select"
	default:
		return "number"
	}
}

func (b *Bridge) settingConfig(s settings.Setting) map[string]any {
	c := map[string]any{
		"name":                  s.Name,
		"unique_id":             b.cfg.DeviceID + "_" + s.Key,
		"command_topic":         b.t.settingCmd(s.Key),
		"state_topic":           b.t.settingState(s.Key),
		"icon":                  s.Icon,
		"availability_topic":    b.t.availability,
		"payload_available":     "online",
		"payload_not_available": "offline",
		"device":                b.deviceBlock(),
	}
	switch s.Kind {
	case settings.Toggle:
		c["payload_on"] = "ON"
		c["payload_off"] = "OFF"
	case settings.Choice:
		c["options"] = s.Labels()
	case settings.Number:
		c["min"] = s.Min
		c["max"] = s.Max
		c["mode"] = "slider"
	}
	return c
}
