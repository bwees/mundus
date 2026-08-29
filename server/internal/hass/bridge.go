package hass

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/bwees/mundus/server/internal/config"
	"github.com/bwees/mundus/server/internal/pending"
	"github.com/bwees/mundus/server/internal/robot"
	"github.com/bwees/mundus/server/internal/robotapi"
)

type Bridge struct {
	cfg   config.Config
	robot *robot.Robot
	api   *robotapi.API
	props *robotapi.Properties
	log   *slog.Logger

	t         topics
	client    mqtt.Client
	connected atomic.Bool
	reconfig  chan config.MQTTSettings

	roomsFn func() []robot.Room

	state  *pending.Value[string]
	drying *pending.Value[bool]

	mu   sync.Mutex
	mode robot.CleanMode

	bursting atomic.Bool
}

func New(cfg config.Config, r *robot.Robot, api *robotapi.API, props *robotapi.Properties, rooms func() []robot.Room, log *slog.Logger) *Bridge {
	return &Bridge{
		cfg:      cfg,
		robot:    r,
		api:      api,
		props:    props,
		roomsFn:  rooms,
		log:      log,
		t:        newTopics(cfg.BaseTopic, cfg.DeviceID),
		mode:     robot.DefaultCleanMode(),
		state:    pending.New[string](pendWindow(cfg)),
		drying:   pending.New[bool](pendWindow(cfg)),
		reconfig: make(chan config.MQTTSettings, 1),
	}
}

// RoomsChanged skips the poll wait so a rename reaches HA immediately.
func (b *Bridge) RoomsChanged() { b.publishState() }

func (b *Bridge) MQTTConnected() bool { return b.connected.Load() }

func (b *Bridge) CurrentMQTT() config.MQTTSettings {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.cfg.MQTT()
}

// Reconfigure requests the Run loop to reconnect with new MQTT settings. The
// actual client teardown/reconnect happens on the Run goroutine so all MQTT
// state stays single-threaded. A pending request is replaced by the newest.
func (b *Bridge) Reconfigure(s config.MQTTSettings) {
	select {
	case <-b.reconfig:
	default:
	}
	b.reconfig <- s
}

func (b *Bridge) apiMode() robotapi.CleanMode {
	m := b.getMode()
	return robotapi.CleanMode{Type: m.Type, FanLevel: m.FanLevel, WaterLevel: m.WaterLevel, Times: m.Times}
}

func (b *Bridge) persistMode() {
	if b.api == nil {
		return
	}
	if err := b.api.SetCleanMode(b.apiMode()); err != nil {
		b.log.Error("set clean mode failed", "err", err)
	}
}

func (b *Bridge) getMode() robot.CleanMode {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.mode
}

func (b *Bridge) updateMode(f func(*robot.CleanMode)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	f(&b.mode)
}

func (b *Bridge) selects() []selectEntity {
	return []selectEntity{
		{
			slug: "clean_type", name: "Clean Type", icon: "mdi:broom",
			options: robot.CleanTypeOptions,
			state:   func() string { return robot.CleanTypeLabel(b.getMode().Type) },
			apply: func(o string) error {
				wire, ok := robot.CleanTypeWire(o)
				if !ok {
					return fmt.Errorf("unknown clean type %q", o)
				}
				b.updateMode(func(m *robot.CleanMode) { m.Type = wire })
				return nil
			},
		},
		{
			slug: "water_level", name: "Water Level", icon: "mdi:water",
			options: robot.WaterLevelOptions,
			state:   func() string { return robot.WaterLevelLabel(b.getMode().WaterLevel) },
			apply: func(o string) error {
				v, ok := robot.WaterLevelValue(o)
				if !ok {
					return fmt.Errorf("unknown water level %q", o)
				}
				b.updateMode(func(m *robot.CleanMode) { m.WaterLevel = v })
				return nil
			},
		},
		{
			slug: "passes", name: "Passes", icon: "mdi:repeat",
			options: robot.PassesOptions,
			state:   func() string { return robot.PassesLabel(b.getMode().Times) },
			apply: func(o string) error {
				v, ok := robot.PassesValue(o)
				if !ok {
					return fmt.Errorf("unknown passes %q", o)
				}
				b.updateMode(func(m *robot.CleanMode) { m.Times = v })
				return nil
			},
		},
	}
}

// pendWindow bounds how long an optimistic value survives without the device
// confirming it: one poll, plus slack for a slow round trip.
func pendWindow(cfg config.Config) time.Duration { return cfg.PollInterval + 5*time.Second }

// refresh publishes state now and again over the next few seconds, so a
// command's real effect reaches Home Assistant without waiting for the poll
// tick. Overlapping refreshes collapse into the one already running.
func (b *Bridge) refresh() {
	b.publishState()
	if !b.bursting.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer b.bursting.Store(false)
		for _, d := range []time.Duration{500 * time.Millisecond, time.Second, 2 * time.Second, 3 * time.Second} {
			time.Sleep(d)
			b.publishState()
		}
	}()
}

func (b *Bridge) dryingConfigured() bool {
	return b.api != nil
}

func (b *Bridge) dryingState(ws int) string {
	if b.drying.Resolve(ws == robot.WorkStatusDryingMop) {
		return "ON"
	}
	return "OFF"
}

func (b *Bridge) Run(stop <-chan struct{}) error {
	b.connect()

	ticker := time.NewTicker(b.cfg.PollInterval)
	defer ticker.Stop()
	b.publishState()

	for {
		select {
		case <-stop:
			if b.client != nil {
				b.publish(b.t.availability, "offline", true)
				b.client.Disconnect(500)
			}
			return nil
		case <-ticker.C:
			b.publishState()
		case s := <-b.reconfig:
			b.applyMQTT(s)
		}
	}
}

// connect (re)establishes the broker link from the current config. An empty
// broker is not an error: the bridge idles until a broker is configured via the
// web UI. Called only from the Run goroutine (and its reconfig case).
func (b *Bridge) connect() {
	if b.cfg.MQTTBroker == "" {
		b.log.Info("mqtt broker not configured; awaiting web setup")
		return
	}
	opts := mqtt.NewClientOptions().
		AddBroker(b.cfg.MQTTBroker).
		SetClientID(b.cfg.MQTTClientID).
		SetUsername(b.cfg.MQTTUsername).
		SetPassword(b.cfg.MQTTPassword).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetWill(b.t.availability, "offline", 1, true).
		SetOnConnectHandler(b.onConnect).
		SetConnectionLostHandler(func(_ mqtt.Client, err error) {
			b.connected.Store(false)
			b.log.Warn("mqtt connection lost", "err", err)
		})

	b.client = mqtt.NewClient(opts)
	if tok := b.client.Connect(); tok.Wait() && tok.Error() != nil {
		b.log.Error("mqtt connect", "broker", b.cfg.MQTTBroker, "err", tok.Error())
	}
}

// applyMQTT tears down the current link and reconnects with new settings,
// rebuilding topics (base topic may have changed) so onConnect re-publishes
// discovery on the new prefix.
func (b *Bridge) applyMQTT(s config.MQTTSettings) {
	b.log.Info("reconfiguring mqtt", "broker", s.Broker)
	if b.client != nil {
		b.publish(b.t.availability, "offline", true)
		b.client.Disconnect(300)
		b.client = nil
	}
	b.connected.Store(false)
	b.mu.Lock()
	b.cfg.ApplyMQTT(s)
	b.mu.Unlock()
	b.t = newTopics(b.cfg.BaseTopic, b.cfg.DeviceID)
	b.connect()
}

func (b *Bridge) onConnect(_ mqtt.Client) {
	b.connected.Store(true)
	b.log.Info("mqtt connected", "broker", b.cfg.MQTTBroker)
	b.publishDiscovery()
	b.publish(b.t.availability, "online", true)
	b.subscribe()
	b.publishState()
}

func (b *Bridge) subscribe() {
	b.sub(b.t.command, b.handleCommand)
	b.sub(b.t.setFanSpeed, b.handleFanSpeed)
	b.sub(b.t.sendCommand, b.handleSendCommand)
	b.sub(b.t.cleanSegments, b.handleCleanSegments)
	for _, e := range b.buttons() {
		run := e.run
		name := e.name
		b.sub(b.t.buttonCmd(e.slug), func(_ mqtt.Client, _ mqtt.Message) {
			if err := run(); err != nil {
				b.log.Error("button failed", "button", name, "err", err)
			}
			b.refresh()
		})
	}
	for _, e := range b.selects() {
		apply := e.apply
		name := e.name
		slug := e.slug
		b.sub(b.t.selectCmd(slug), func(_ mqtt.Client, m mqtt.Message) {
			opt := string(m.Payload())
			if err := apply(opt); err != nil {
				b.log.Error("select failed", "select", name, "option", opt, "err", err)
				return
			}
			b.persistMode()
			b.publishState()
		})
	}
	if b.dryingConfigured() {
		b.sub(b.t.switchCmd("mop_drying"), b.handleMopDrying)
	}
	for _, e := range b.settings() {
		b.sub(b.t.settingCmd(e.Key), func(_ mqtt.Client, m mqtt.Message) {
			if err := b.applySetting(e, string(m.Payload())); err != nil {
				b.log.Error("setting failed", "setting", e.Name, "err", err)
				return
			}
			b.publishSettingsState()
		})
	}
}

func (b *Bridge) publishSettingsState() {
	for _, e := range b.settings() {
		b.publish(b.t.settingState(e.Key), b.settingPayload(e), true)
	}
}

func (b *Bridge) sub(topic string, h mqtt.MessageHandler) {
	if tok := b.client.Subscribe(topic, 1, h); tok.Wait() && tok.Error() != nil {
		b.log.Error("subscribe failed", "topic", topic, "err", tok.Error())
	}
}

func (b *Bridge) handleCommand(_ mqtt.Client, m mqtt.Message) {
	cmd := string(m.Payload())
	if b.api == nil {
		b.log.Error("command dropped: funcapi unavailable", "command", cmd)
		return
	}
	var err error
	var want string
	switch cmd {
	case "start":
		err, want = b.api.CleanAll(b.apiMode()), "cleaning"
	case "pause":
		err, want = b.api.Pause(), "paused"
	case "stop":
		err, want = b.api.Stop(), "idle"
	case "return_to_base":
		err, want = b.api.Dock(), "returning"
	case "locate":
		err = b.api.Locate()
	case "clean_spot":
		err, want = b.api.CleanSpot(b.apiMode()), "cleaning"
	default:
		err = fmt.Errorf("unknown command %q", cmd)
	}
	if err != nil {
		b.log.Error("command failed", "command", cmd, "err", err)
	} else if want != "" {
		b.state.Expect(want)
	}
	b.refresh()
}

func (b *Bridge) handleFanSpeed(_ mqtt.Client, m mqtt.Message) {
	speed := string(m.Payload())
	level, ok := robot.FanSpeedToLevel(speed)
	if !ok {
		b.log.Error("unknown fan speed", "speed", speed)
		return
	}
	b.updateMode(func(cm *robot.CleanMode) { cm.FanLevel = level })
	b.persistMode()
	b.publishState()
}

func (b *Bridge) handleMopDrying(_ mqtt.Client, m mqtt.Message) {
	on := string(m.Payload()) == "ON"
	if b.api == nil {
		b.log.Error("mop drying dropped: funcapi unavailable")
		return
	}
	action := robotapi.SelfCleanStopDrying
	if on {
		action = robotapi.SelfCleanStartDrying
	}
	if err := b.api.SelfClean(action); err != nil {
		b.log.Error("mop drying failed", "on", on, "err", err)
		return
	}
	b.drying.Expect(on)
	b.refresh()
}

// handleSendCommand runs an arbitrary terminal command line, or a structured
// room/zone command. Accepts a bare string, or HA's vacuum.send_command shape
// {"command": "...", "params": ...}. Structured commands:
//
//	segment_clean / room_clean : params = ["ROOM_001", ...]
//	clean_area                 : params = {"polygon": [[x,y],...]} or [[x,y],...]
//
// Anything else runs as a raw terminal line.
func (b *Bridge) handleSendCommand(_ mqtt.Client, m mqtt.Message) {
	var wrap struct {
		Command string          `json:"command"`
		Params  json.RawMessage `json:"params"`
	}
	if json.Unmarshal(m.Payload(), &wrap) == nil && wrap.Command != "" {
		switch wrap.Command {
		case "segment_clean", "room_clean", "clean_rooms":
			var ids []string
			if err := json.Unmarshal(wrap.Params, &ids); err != nil || len(ids) == 0 {
				b.log.Error("room clean params", "raw", string(wrap.Params), "err", err)
				return
			}
			if b.api != nil {
				if err := b.api.CleanRooms(ids, b.apiMode()); err != nil {
					b.log.Error("room clean failed", "ids", ids, "err", err)
				} else {
					b.state.Expect("cleaning")
				}
			}
			b.refresh()
			return
		case "clean_area", "clean_areas", "zone_clean":
			polys, err := parsePolygons(wrap.Params)
			if err != nil {
				b.log.Error("clean_area params", "raw", string(wrap.Params), "err", err)
				return
			}
			if b.api != nil {
				if err := b.api.CleanAreas(polys, b.apiMode()); err != nil {
					b.log.Error("clean_area failed", "err", err)
				} else {
					b.state.Expect("cleaning")
				}
			}
			b.refresh()
			return
		}
		b.runRaw(wrap.Command)
		return
	}
	b.runRaw(string(m.Payload()))
}

func (b *Bridge) runRaw(cmd string) {
	out, err := b.robot.Raw(cmd)
	if err != nil {
		b.log.Error("send_command failed", "command", cmd, "err", err)
		return
	}
	b.log.Info("send_command ok", "command", cmd, "output", out)
}

func parsePolygons(raw json.RawMessage) ([][][2]float64, error) {
	var wrapped struct {
		Polygon [][2]float64 `json:"polygon"`
	}
	if err := json.Unmarshal(raw, &wrapped); err == nil && len(wrapped.Polygon) > 0 {
		return [][][2]float64{wrapped.Polygon}, nil
	}
	var poly [][2]float64
	if err := json.Unmarshal(raw, &poly); err == nil && len(poly) > 0 {
		return [][][2]float64{poly}, nil
	}
	return nil, fmt.Errorf("no polygon in params")
}

// Payload is a JSON array of ROOM_ label ids, e.g. ["ROOM_001","ROOM_002"].
func (b *Bridge) handleCleanSegments(_ mqtt.Client, m mqtt.Message) {
	var ids []string
	if err := json.Unmarshal(m.Payload(), &ids); err != nil || len(ids) == 0 {
		b.log.Error("clean_segments payload", "raw", string(m.Payload()), "err", err)
		return
	}
	if b.api == nil {
		b.log.Error("clean_segments dropped: funcapi unavailable")
		return
	}
	if err := b.api.CleanRooms(ids, b.apiMode()); err != nil {
		b.log.Error("clean_segments failed", "ids", ids, "err", err)
	} else {
		b.state.Expect("cleaning")
	}
	b.refresh()
}

// roomList returns the current room list. It reads from the map labels (via the
// injected provider — the same source the map editor edits) so renames reach HA
// promptly, falling back to the robot's room query.
func (b *Bridge) roomList() []robot.Room {
	if b.roomsFn != nil {
		if rooms := b.roomsFn(); len(rooms) > 0 {
			return rooms
		}
	}
	rooms, err := b.robot.Rooms()
	if err != nil {
		b.log.Warn("room enumerate failed", "err", err)
	}
	return rooms
}

func (b *Bridge) publishState() {
	st, err := b.robot.Poll()
	if err != nil {
		b.log.Warn("poll failed", "err", err)
		b.publish(b.t.availability, "offline", true)
		return
	}
	b.publish(b.t.availability, "online", true)

	state := map[string]any{"state": b.state.Resolve(st.HAState)}
	if st.Battery >= 0 {
		state["battery_level"] = st.Battery
	}
	// fan_speed reflects the selected clean mode (what the set-fan command
	// persists via funcID 1043), not the polled level — otherwise each poll
	// would revert HA's selection to the idle/reported value.
	if fs := robot.FanLevelToHA(b.getMode().FanLevel); fs != "" {
		state["fan_speed"] = fs
	}
	if rooms := b.roomList(); len(rooms) > 0 {
		seg := make(map[string]string, len(rooms))
		for _, r := range rooms {
			seg[r.ID] = r.Name
		}
		state["segments"] = seg
	}
	b.publishJSON(b.t.state, state)

	attrs := map[string]any{
		"error_code":      st.ErrorCode,
		"working_status":  st.WorkingStatus,
		"cloud_connected": st.CloudConnected,
		"charging":        st.Charging,
	}
	if st.RunState != "" {
		attrs["run_state"] = st.RunState
	}
	b.publishJSON(b.t.attributes, attrs)

	for _, e := range b.selects() {
		b.publish(b.t.selectState(e.slug), e.state(), true)
	}
	if b.dryingConfigured() {
		b.publish(b.t.switchState("mop_drying"), b.dryingState(st.WorkingStatus), true)
	}
	b.publishSettingsState()
}

// discoveryTopic is where Home Assistant looks for one entity's config. These
// are published retained, so renaming a key strands the old entity in Home
// Assistant until something publishes an empty payload to its old topic.
func (b *Bridge) discoveryTopic(component, key string) string {
	return fmt.Sprintf("%s/%s/%s_%s/config", b.cfg.DiscoveryPrefix, component, b.cfg.DeviceID, key)
}

func (b *Bridge) publishDiscovery() {
	b.publishJSONRetained(
		fmt.Sprintf("%s/vacuum/%s/config", b.cfg.DiscoveryPrefix, b.cfg.DeviceID),
		b.vacuumConfig(),
	)
	for _, e := range b.buttons() {
		b.publishJSONRetained(
			b.discoveryTopic("button", e.slug),
			b.buttonConfig(e),
		)
	}
	for _, e := range b.selects() {
		b.publishJSONRetained(
			b.discoveryTopic("select", e.slug),
			b.selectConfig(e),
		)
	}
	if b.dryingConfigured() {
		b.publishJSONRetained(
			b.discoveryTopic("switch", "mop_drying"),
			b.switchConfig(),
		)
	}
	for _, e := range b.settings() {
		b.publishJSONRetained(
			b.discoveryTopic(component(e.Kind), e.Key),
			b.settingConfig(e),
		)
	}
}

func (b *Bridge) publish(topic, payload string, retained bool) {
	if b.client == nil {
		return
	}
	b.client.Publish(topic, 1, retained, payload)
}

func (b *Bridge) publishJSON(topic string, v any) {
	if b.client == nil {
		return
	}
	data, err := json.Marshal(v)
	if err != nil {
		b.log.Error("marshal failed", "topic", topic, "err", err)
		return
	}
	b.client.Publish(topic, 1, false, data)
}

func (b *Bridge) publishJSONRetained(topic string, v any) {
	if b.client == nil {
		return
	}
	data, err := json.Marshal(v)
	if err != nil {
		b.log.Error("marshal failed", "topic", topic, "err", err)
		return
	}
	b.client.Publish(topic, 1, true, data)
}
