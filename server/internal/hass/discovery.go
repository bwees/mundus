package hass

import (
	"fmt"

	"github.com/bwees/mundus/server/internal/robot"
	"github.com/bwees/mundus/server/internal/robotapi"
)

type topics struct {
	base          string
	command       string
	state         string
	setFanSpeed   string
	sendCommand   string
	cleanSegments string
	availability  string
	attributes    string
}

func newTopics(baseTopic, deviceID string) topics {
	base := fmt.Sprintf("%s/%s", baseTopic, deviceID)
	return topics{
		base:          base,
		command:       base + "/command",
		state:         base + "/state",
		setFanSpeed:   base + "/set_fan_speed",
		sendCommand:   base + "/send_command",
		cleanSegments: base + "/clean_segments",
		availability:  base + "/availability",
		attributes:    base + "/attributes",
	}
}

func (t topics) buttonCmd(slug string) string   { return t.base + "/button/" + slug }
func (t topics) selectCmd(slug string) string   { return t.base + "/select/" + slug + "/set" }
func (t topics) selectState(slug string) string { return t.base + "/select/" + slug + "/state" }
func (t topics) switchCmd(slug string) string   { return t.base + "/switch/" + slug + "/set" }
func (t topics) switchState(slug string) string { return t.base + "/switch/" + slug + "/state" }

func (b *Bridge) deviceBlock() map[string]any {
	return map[string]any{
		"identifiers":  []string{b.cfg.DeviceID},
		"name":         b.cfg.DeviceName,
		"manufacturer": "SwitchBot",
		"model":        "S20 (mundus)",
	}
}

func (b *Bridge) vacuumConfig() map[string]any {
	return map[string]any{
		"name":                         nil,
		"unique_id":                    b.cfg.DeviceID + "_vacuum",
		"schema":                       "state",
		"supported_features":           []string{"start", "stop", "pause", "return_home", "status", "locate", "clean_spot", "fan_speed", "send_command"},
		"command_topic":                b.t.command,
		"payload_start":                "start",
		"payload_stop":                 "stop",
		"payload_pause":                "pause",
		"payload_return_to_base":       "return_to_base",
		"payload_locate":               "locate",
		"payload_clean_spot":           "clean_spot",
		"state_topic":                  b.t.state,
		"set_fan_speed_topic":          b.t.setFanSpeed,
		"fan_speed_list":               robot.FanSpeedOptions,
		"send_command_topic":           b.t.sendCommand,
		"clean_segments_command_topic": b.t.cleanSegments,
		"availability_topic":           b.t.availability,
		"payload_available":            "online",
		"payload_not_available":        "offline",
		"json_attributes_topic":        b.t.attributes,
		"device":                       b.deviceBlock(),
	}
}

type buttonEntity struct {
	slug string
	name string
	run  func() error
}

// buttons enumerates the non-standard robot functions exposed as HA buttons.
// Self-clean actions go through the funcID API (control_center runs the full base
// routine); the remaining base ops fall back to their terminal commands.
func (b *Bridge) buttons() []buttonEntity {
	var out []buttonEntity
	if b.api != nil {
		out = append(out,
			buttonEntity{"mop_wash", "Mop Wash", func() error { return b.api.SelfClean(robotapi.SelfCleanMopWash) }},
			buttonEntity{"dust_collect", "Dust Collection", func() error { return b.api.SelfClean(robotapi.SelfCleanDustCollect) }},
		)
	}
	r := b.robot
	return append(out,
		buttonEntity{"explore", "Explore / Build Map", r.Explore},
		buttonEntity{"water_base_charge", "Water Base Charge", r.WaterBaseCharge},
		buttonEntity{"mark_water_base", "Mark Water Base", r.MarkWaterBase},
		buttonEntity{"fill_humidifier", "Fill Humidifier", r.FillHumidifier},
	)
}

func (b *Bridge) buttonConfig(e buttonEntity) map[string]any {
	return map[string]any{
		"name":                  e.name,
		"unique_id":             b.cfg.DeviceID + "_" + e.slug,
		"command_topic":         b.t.buttonCmd(e.slug),
		"availability_topic":    b.t.availability,
		"payload_available":     "online",
		"payload_not_available": "offline",
		"device":                b.deviceBlock(),
	}
}

type selectEntity struct {
	slug    string
	name    string
	icon    string
	options []string
	state   func() string
	apply   func(string) error
}

func (b *Bridge) selectConfig(e selectEntity) map[string]any {
	return map[string]any{
		"name":                  e.name,
		"unique_id":             b.cfg.DeviceID + "_" + e.slug,
		"command_topic":         b.t.selectCmd(e.slug),
		"state_topic":           b.t.selectState(e.slug),
		"options":               e.options,
		"icon":                  e.icon,
		"availability_topic":    b.t.availability,
		"payload_available":     "online",
		"payload_not_available": "offline",
		"device":                b.deviceBlock(),
	}
}

func (b *Bridge) switchConfig() map[string]any {
	return map[string]any{
		"name":                  "Mop Drying",
		"unique_id":             b.cfg.DeviceID + "_mop_drying",
		"command_topic":         b.t.switchCmd("mop_drying"),
		"state_topic":           b.t.switchState("mop_drying"),
		"payload_on":            "ON",
		"payload_off":           "OFF",
		"icon":                  "mdi:hair-dryer",
		"availability_topic":    b.t.availability,
		"payload_available":     "online",
		"payload_not_available": "offline",
		"device":                b.deviceBlock(),
	}
}
