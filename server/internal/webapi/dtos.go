package webapi

import "github.com/bwees/mundus/server/internal/settings"

type StateDTO struct {
	DeviceName     string `json:"device_name"`
	State          string `json:"state"`
	BatteryLevel   int    `json:"battery_level"`
	FanSpeed       string `json:"fan_speed"`
	WorkingStatus  int    `json:"working_status"`
	ErrorCode      int    `json:"error_code"`
	Charging       bool   `json:"charging"`
	Docked         bool   `json:"docked"`
	CloudConnected bool   `json:"cloud_connected"`
	RunState       string `json:"run_state"`
}

type RoomDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ModeDTO struct {
	Type       string `json:"type"`
	FanLevel   int    `json:"fan_level"`
	WaterLevel int    `json:"water_level"`
	Times      int    `json:"times"`
}

// Empty Rooms means whole-house.
type CleanRequest struct {
	Rooms []string `json:"rooms"`
	Mode  ModeDTO  `json:"mode"`
}

type SelfCleanRequest struct {
	Action int `json:"action"` // 1 wash, 2 dry, 3 stop-dry, 4 dust
}

// SettingsDTO carries the setting definitions alongside their current values, so
// the web UI renders the controls from the server rather than keeping its own
// copy of the labels. Toggles read and write as 0 or 1.
type SettingsDTO struct {
	Schema []settings.Setting `json:"schema"`
	Values map[string]int     `json:"values"`
}

type SettingsInput struct {
	Values map[string]int `json:"values"`
}

type OK struct {
	OK bool `json:"ok"`
}

type RoomGeomDTO struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	ColorType int       `json:"color_type"`
	Graph     []int     `json:"graph"`
	Geometry  []float64 `json:"geometry"`
}

// ZoneDTO is one map marker: a carpet, no-mop, or no-go region (or the
// read-only dock base). Kind is carpet|no_mop|no_go|base. Geometry is a flat
// [x,y,…] polygon in world metres (a [x,y,theta] pose for the base).
type ZoneDTO struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Kind     string    `json:"kind"`
	Geometry []float64 `json:"geometry"`
}

// MapDTO is the full map payload: raster (base64 PNG) + transform + rooms +
// zones. The transform lets the UI place world-metre polygons over the raster.
type MapDTO struct {
	UUID       string        `json:"uuid"`
	Resolution float64       `json:"resolution"`
	OriginX    float64       `json:"origin_x"`
	OriginY    float64       `json:"origin_y"`
	Width      int           `json:"width"`
	Height     int           `json:"height"`
	ImagePNG   string        `json:"image_png"`
	Rooms      []RoomGeomDTO `json:"rooms"`
	Zones      []ZoneDTO     `json:"zones"`
}

// TrackDTO is the robot's pose trail for the current task: a flat [x,y,…]
// polyline in world metres, the same frame as room and zone geometry. Empty
// when no task has run since the map was created.
type TrackDTO struct {
	Points []float64 `json:"points"`
}

// AddZoneInput adds a carpet/no_mop/no_go rectangle. Geometry is flat [x,y,…].
type AddZoneInput struct {
	Kind     string    `json:"kind"`
	Name     string    `json:"name"`
	Geometry []float64 `json:"geometry"`
}

type ZoneIDInput struct {
	ID string `json:"id"`
}

// UpdateZoneInput moves/resizes/renames a zone. Empty Geometry keeps the current
// shape; empty Name keeps the current name.
type UpdateZoneInput struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Geometry []float64 `json:"geometry"`
}

type RenameRoomInput struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type MergeRoomsInput struct {
	IDs  []string `json:"ids"`
	Name string   `json:"name"`
}

// SplitRoomInput splits room ID by an infinite line through two world-metre
// points; NewName names the freshly created half.
type SplitRoomInput struct {
	ID      string     `json:"id"`
	Line    [4]float64 `json:"line"`
	NewName string     `json:"new_name"`
}

// MQTTConfigDTO reports the current MQTT setup. The password is never returned;
// HasPassword tells the UI whether one is set. Connected is the live link state.
type MQTTConfigDTO struct {
	Broker          string `json:"broker"`
	Username        string `json:"username"`
	BaseTopic       string `json:"base_topic"`
	DiscoveryPrefix string `json:"discovery_prefix"`
	HasPassword     bool   `json:"has_password"`
	Connected       bool   `json:"connected"`
}

// MQTTConfigInput sets the MQTT connection. An empty Password keeps the existing
// one (so the UI can save other fields without echoing the secret back).
type MQTTConfigInput struct {
	Broker          string `json:"broker"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	BaseTopic       string `json:"base_topic"`
	DiscoveryPrefix string `json:"discovery_prefix"`
}
