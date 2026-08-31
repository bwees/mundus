package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`

	RobotAddr        string        `json:"robot_addr"`
	RobotDialTimeout time.Duration `json:"-"`
	RobotReadIdle    time.Duration `json:"-"`

	MQTTBroker   string `json:"mqtt_broker"`
	MQTTUsername string `json:"mqtt_username"`
	MQTTPassword string `json:"mqtt_password"`
	MQTTClientID string `json:"mqtt_client_id"`

	DiscoveryPrefix string        `json:"discovery_prefix"`
	BaseTopic       string        `json:"base_topic"`
	PropertyCache   string        `json:"property_cache"`
	MapDir          string        `json:"map_dir"`
	TrackPath       string        `json:"track_path"`
	WebAddr         string        `json:"web_addr"`
	WebStatic       string        `json:"web_static"`
	RuntimePath     string        `json:"runtime_path"`
	AuthPath        string        `json:"auth_path"`
	BinPath         string        `json:"bin_path"`
	LogDir          string        `json:"log_dir"`
	CloudSimDir     string        `json:"cloud_sim_dir"`
	PollInterval    time.Duration `json:"-"`

	RobotDialTimeoutMS int `json:"robot_dial_timeout_ms"`
	RobotReadIdleMS    int `json:"robot_read_idle_ms"`
	PollIntervalMS     int `json:"poll_interval_ms"`
}

func Default() Config {
	return Config{
		DeviceID:           "switchbot_s20",
		DeviceName:         "SwitchBot S20",
		RobotAddr:          "127.0.0.1:50000",
		MQTTBroker:         "tcp://127.0.0.1:1883",
		MQTTClientID:       "mundus",
		DiscoveryPrefix:    "homeassistant",
		BaseTopic:          "mundus",
		PropertyCache:      "/data/control_center/db/property_table_cache.json",
		MapDir:             "/data/map_server/map",
		TrackPath:          "/data/control_center/current_map/track.json",
		WebAddr:            ":8080",
		WebStatic:          "/opt/wlab/sweepbot/mundus/web",
		RuntimePath:        "/opt/wlab/sweepbot/mundus/runtime.json",
		AuthPath:           "/opt/wlab/sweepbot/mundus/auth.json",
		BinPath:            "/opt/wlab/sweepbot/mundus/mundus",
		LogDir:             "/opt/wlab/sweepbot/mundus/logs",
		CloudSimDir:        "/opt/wlab/sweepbot/mundus/cloudsim",
		RobotDialTimeoutMS: 5000,
		RobotReadIdleMS:    400,
		PollIntervalMS:     10000,
	}
}

func Load(path string) (Config, error) {
	c := Default()
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return c, fmt.Errorf("read config: %w", err)
		}
		if err := json.Unmarshal(data, &c); err != nil {
			return c, fmt.Errorf("parse config: %w", err)
		}
	}
	if rt, err := LoadRuntime(c.RuntimePath); err == nil {
		c.ApplyMQTT(rt)
	}
	c.derive()
	if err := c.validate(); err != nil {
		return c, err
	}
	return c, nil
}

// MQTTSettings are the connection fields configurable at runtime via the web UI
// and persisted to RuntimePath, overlaid on top of the static config at load.
type MQTTSettings struct {
	Broker          string `json:"mqtt_broker"`
	Username        string `json:"mqtt_username"`
	Password        string `json:"mqtt_password"`
	BaseTopic       string `json:"base_topic"`
	DiscoveryPrefix string `json:"discovery_prefix"`
}

func (c Config) MQTT() MQTTSettings {
	return MQTTSettings{
		Broker:          c.MQTTBroker,
		Username:        c.MQTTUsername,
		Password:        c.MQTTPassword,
		BaseTopic:       c.BaseTopic,
		DiscoveryPrefix: c.DiscoveryPrefix,
	}
}

func (c *Config) ApplyMQTT(s MQTTSettings) {
	c.MQTTBroker = s.Broker
	c.MQTTUsername = s.Username
	c.MQTTPassword = s.Password
	if s.BaseTopic != "" {
		c.BaseTopic = s.BaseTopic
	}
	if s.DiscoveryPrefix != "" {
		c.DiscoveryPrefix = s.DiscoveryPrefix
	}
}

func LoadRuntime(path string) (MQTTSettings, error) {
	var s MQTTSettings
	if path == "" {
		return s, fmt.Errorf("no runtime path")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return s, err
	}
	return s, nil
}

func SaveRuntime(path string, s MQTTSettings) error {
	if path == "" {
		return fmt.Errorf("no runtime path")
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (c *Config) derive() {
	c.RobotDialTimeout = time.Duration(c.RobotDialTimeoutMS) * time.Millisecond
	c.RobotReadIdle = time.Duration(c.RobotReadIdleMS) * time.Millisecond
	c.PollInterval = time.Duration(c.PollIntervalMS) * time.Millisecond
	c.DeviceID = sanitizeID(c.DeviceID)
	if c.MQTTClientID == "" {
		c.MQTTClientID = "mundus-" + c.DeviceID
	}
}

func (c *Config) validate() error {
	switch {
	case c.DeviceID == "":
		return fmt.Errorf("device_id is required")
	case c.RobotAddr == "":
		return fmt.Errorf("robot_addr is required")
	}
	return nil
}

func sanitizeID(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}
