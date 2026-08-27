// Package webapi serves the mundus web app: a fuego-generated JSON API
// (with OpenAPI) over the robot control layer, plus the SvelteKit static build.
package webapi

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-fuego/fuego"

	"github.com/bwees/mundus/server/internal/auth"
	"github.com/bwees/mundus/server/internal/config"
	"github.com/bwees/mundus/server/internal/robot"
	"github.com/bwees/mundus/server/internal/robotapi"
	"github.com/bwees/mundus/server/internal/settings"
	"github.com/bwees/mundus/server/internal/system"
	"github.com/bwees/mundus/server/internal/update"
)

// MQTTController is the subset of the HA bridge the web API drives to read and
// change the broker connection at runtime.
type MQTTController interface {
	CurrentMQTT() config.MQTTSettings
	MQTTConnected() bool
	Reconfigure(config.MQTTSettings)
	RoomsChanged()
}

type Deps struct {
	Auth        *auth.Store
	Security    fuego.Security
	API         *robotapi.API
	Props       *robotapi.Properties
	Robot       *robot.Robot
	Rooms       func() []robot.Room
	MQTT        MQTTController
	System      *system.System
	Update      *update.Updater
	MapDir      string
	RuntimePath string
	DeviceName  string
	Addr        string
	StaticPath  string
	Log         *slog.Logger
	// SpecMode registers every route regardless of nil deps so the OpenAPI
	// export is complete; handlers are never invoked in this mode.
	SpecMode bool
}

func toMode(m ModeDTO) robotapi.CleanMode {
	return robotapi.CleanMode{Type: m.Type, FanLevel: m.FanLevel, WaterLevel: m.WaterLevel, Times: m.Times}
}

func NewServer(d Deps) *fuego.Server {
	s := fuego.NewServer(
		fuego.WithAddr(d.Addr),
		fuego.WithSerializer(fuego.SendJSON),
	)
	s.OpenAPI.Config.JSONFilePath = "openapi.json"
	s.OpenAPI.Config.SpecURL = "/api/openapi.json"
	s.OpenAPI.Config.DisableSwaggerUI = true
	s.OpenAPI.Config.DisableDefaultServer = true
	s.OpenAPI.Config.PrettyFormatJSON = true

	api := fuego.Group(s, "/api")
	if !d.SpecMode {
		fuego.Use(api, requireAuth(d))
	}
	registerAuth(api, d)

	fuego.Get(api, "/state", func(ctx fuego.ContextNoBody) (StateDTO, error) {
		st, err := d.Robot.Poll()
		if err != nil {
			return StateDTO{}, err
		}
		return StateDTO{
			DeviceName: d.DeviceName,
			State:      st.HAState, BatteryLevel: st.Battery, FanSpeed: st.FanSpeed,
			WorkingStatus: st.WorkingStatus, ErrorCode: st.ErrorCode,
			Charging: st.Charging, Docked: st.Docked, CloudConnected: st.CloudConnected,
			RunState: st.RunState,
		}, nil
	}, fuego.OptionOperationID("getState"))

	fuego.Get(api, "/rooms", func(ctx fuego.ContextNoBody) ([]RoomDTO, error) {
		var out []RoomDTO
		for _, r := range d.Rooms() {
			out = append(out, RoomDTO{ID: r.ID, Name: r.Name})
		}
		return out, nil
	}, fuego.OptionOperationID("getRooms"))

	fuego.Post(api, "/clean", func(ctx fuego.ContextWithBody[CleanRequest]) (OK, error) {
		body, err := ctx.Body()
		if err != nil {
			return OK{}, err
		}
		mode := toMode(body.Mode)
		if mode.Type == "" {
			mode = robotapi.DefaultCleanMode()
		}
		if len(body.Rooms) > 0 {
			err = d.API.CleanRooms(body.Rooms, mode)
		} else {
			err = d.API.CleanAll(mode)
		}
		return OK{OK: err == nil}, err
	}, fuego.OptionOperationID("startClean"))

	fuego.Post(api, "/dock", cmd(func() error { return d.API.Dock() }), fuego.OptionOperationID("dock"))
	fuego.Post(api, "/pause", cmd(func() error { return d.API.Pause() }), fuego.OptionOperationID("pause"))
	fuego.Post(api, "/resume", cmd(func() error { return d.API.Resume() }), fuego.OptionOperationID("resume"))
	fuego.Post(api, "/stop", cmd(func() error { return d.API.Stop() }), fuego.OptionOperationID("stop"))
	fuego.Post(api, "/locate", cmd(func() error { return d.API.Locate() }), fuego.OptionOperationID("locate"))

	fuego.Post(api, "/self-clean", func(ctx fuego.ContextWithBody[SelfCleanRequest]) (OK, error) {
		body, err := ctx.Body()
		if err != nil {
			return OK{}, err
		}
		err = d.API.SelfClean(body.Action)
		return OK{OK: err == nil}, err
	}, fuego.OptionOperationID("selfClean"))

	fuego.Get(api, "/settings", func(ctx fuego.ContextNoBody) (SettingsDTO, error) {
		all := settings.All()
		values := make(map[string]int, len(all))
		for _, s := range all {
			values[s.Key] = s.Read(d.Props)
		}
		return SettingsDTO{Schema: all, Values: values}, nil
	}, fuego.OptionOperationID("getSettings"))

	fuego.Put(api, "/settings", func(ctx fuego.ContextWithBody[SettingsInput]) (OK, error) {
		body, err := ctx.Body()
		if err != nil {
			return OK{}, err
		}
		known := map[string]settings.Setting{}
		for _, s := range settings.All() {
			known[s.Key] = s
		}
		for key, v := range body.Values {
			s, ok := known[key]
			if !ok {
				return OK{}, fuego.BadRequestError{Title: "unknown setting " + key}
			}
			if err := s.Write(d.Props, v); err != nil {
				return OK{}, fuego.BadRequestError{Title: err.Error()}
			}
		}
		return OK{OK: true}, nil
	}, fuego.OptionOperationID("setSettings"))

	if d.SpecMode || (d.API != nil && d.MapDir != "") {
		registerMap(api, d)
	}

	if d.SpecMode || d.System != nil {
		registerSystem(api, d)
	}

	if d.SpecMode || d.Update != nil {
		registerUpdate(api, d)
	}

	if d.SpecMode || d.MQTT != nil {
		fuego.Get(api, "/config/mqtt", func(ctx fuego.ContextNoBody) (MQTTConfigDTO, error) {
			cur := d.MQTT.CurrentMQTT()
			return MQTTConfigDTO{
				Broker: cur.Broker, Username: cur.Username,
				BaseTopic: cur.BaseTopic, DiscoveryPrefix: cur.DiscoveryPrefix,
				HasPassword: cur.Password != "", Connected: d.MQTT.MQTTConnected(),
			}, nil
		}, fuego.OptionOperationID("getMqttConfig"))

		fuego.Put(api, "/config/mqtt", func(ctx fuego.ContextWithBody[MQTTConfigInput]) (MQTTConfigDTO, error) {
			body, err := ctx.Body()
			if err != nil {
				return MQTTConfigDTO{}, err
			}
			cur := d.MQTT.CurrentMQTT()
			next := config.MQTTSettings{
				Broker: body.Broker, Username: body.Username, Password: body.Password,
				BaseTopic: body.BaseTopic, DiscoveryPrefix: body.DiscoveryPrefix,
			}
			if next.Password == "" {
				next.Password = cur.Password
			}
			if err := config.SaveRuntime(d.RuntimePath, next); err != nil {
				return MQTTConfigDTO{}, err
			}
			d.MQTT.Reconfigure(next)
			return MQTTConfigDTO{
				Broker: next.Broker, Username: next.Username,
				BaseTopic: next.BaseTopic, DiscoveryPrefix: next.DiscoveryPrefix,
				HasPassword: next.Password != "", Connected: d.MQTT.MQTTConnected(),
			}, nil
		}, fuego.OptionOperationID("setMqttConfig"))
	}

	registerStatic(s, d.StaticPath, d.Log)
	return s
}

func cmd(fn func() error) func(fuego.ContextNoBody) (OK, error) {
	return func(ctx fuego.ContextNoBody) (OK, error) {
		err := fn()
		return OK{OK: err == nil}, err
	}
}

func registerStatic(s *fuego.Server, root string, log *slog.Logger) {
	if root == "" {
		return
	}
	index := filepath.Join(root, "index.html")
	fileServer := http.FileServer(http.Dir(root))
	handler := func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api") {
			http.NotFound(w, r)
			return
		}
		clean := filepath.Join(root, filepath.Clean(r.URL.Path))
		if info, err := os.Stat(clean); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, index)
	}
	fuego.Handle(s, "/", http.HandlerFunc(handler), fuego.OptionHide())
	log.Info("serving web frontend", "path", root)
}
