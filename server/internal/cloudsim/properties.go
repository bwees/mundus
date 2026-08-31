package cloudsim

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Device settings are applied by sending a propertySet, the same message the
// SwitchBot cloud would send. Writing property ids into control_center's
// property_table_cache.json does nothing: that file is its outbound push queue,
// it is rewritten from control_center's own table, and the real settings live
// in db/setting.json keyed by name.
//
// Nothing persists them for us either -- the cloud shadow used to. So the
// values are kept here and replayed onto every new session, which is what makes
// a setting survive a control_center restart.

const propertyStoreName = "properties.json"

type session struct {
	conn  net.Conn
	topic string // v1_1/<model>/<deviceID>/propertySet
}

type properties struct {
	mu     sync.Mutex
	path   string
	values map[string]any
}

func loadProperties(dir string) (*properties, error) {
	p := &properties{path: filepath.Join(dir, propertyStoreName), values: map[string]any{}}
	data, err := os.ReadFile(p.path)
	if err != nil {
		if os.IsNotExist(err) {
			return p, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return p, nil
	}
	if err := json.Unmarshal(data, &p.values); err != nil {
		return nil, fmt.Errorf("parse %s: %w", p.path, err)
	}
	if p.values == nil {
		p.values = map[string]any{}
	}
	return p, nil
}

func (p *properties) all() map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make(map[string]any, len(p.values))
	for k, v := range p.values {
		out[k] = v
	}
	return out
}

func (p *properties) get(id int) (any, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	v, ok := p.values[strconv.Itoa(id)]
	return v, ok
}

func (p *properties) set(id int, v any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.values[strconv.Itoa(id)] = v
	out, err := json.MarshalIndent(p.values, "", "  ")
	if err != nil {
		return err
	}
	tmp := p.path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p.path)
}

// adoptSession records where to publish for this connection and replays the
// stored settings, so a control_center restart does not lose them.
func (s *Server) adoptSession(c net.Conn, topics []string) {
	for _, t := range topics {
		if !strings.HasSuffix(t, "/propertySet") {
			continue
		}
		s.mu.Lock()
		s.session = &session{conn: c, topic: t}
		s.mu.Unlock()
		s.cfg.Log.Info("cloudsim session ready", "topic", t)

		if stored := s.props.all(); len(stored) > 0 {
			if err := s.publishProperties(stored); err != nil {
				s.cfg.Log.Error("cloudsim replay failed", "err", err)
			} else {
				s.cfg.Log.Info("cloudsim replayed settings", "count", len(stored))
			}
		}
		return
	}
}

func (s *Server) clearSession(c net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session != nil && s.session.conn == c {
		s.session = nil
	}
}

// SetProperty applies a device setting and remembers it for replay.
func (s *Server) SetProperty(id int, value any) error {
	if err := s.publishProperties(map[string]any{strconv.Itoa(id): value}); err != nil {
		return err
	}
	return s.props.set(id, value)
}

// Property returns a stored value. Only settings mundus has written are known;
// anything else falls back to the caller's default.
func (s *Server) Property(id int) (any, bool) { return s.props.get(id) }

// Settings adapts the simulator to the settings package: reads come from what
// mundus has stored, writes go to the robot as a propertySet.
func (s *Server) Settings() *SettingsBackend { return &SettingsBackend{srv: s} }

type SettingsBackend struct{ srv *Server }

// A value just written is still the int or bool it was set as, while one
// reloaded from disk has been through JSON and comes back as float64. Both
// shapes have to be accepted or a setting reads as its default until restart.
func (b *SettingsBackend) GetInt(id, def int) int {
	v, ok := b.srv.Property(id)
	if !ok {
		return def
	}
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	case bool:
		if n {
			return 1
		}
		return 0
	}
	return def
}

func (b *SettingsBackend) GetBool(id int, def bool) bool {
	v, ok := b.srv.Property(id)
	if !ok {
		return def
	}
	switch n := v.(type) {
	case bool:
		return n
	case int:
		return n != 0
	case float64:
		return n != 0
	}
	return def
}

func (b *SettingsBackend) Set(id int, v any) error { return b.srv.SetProperty(id, v) }

// Connected reports whether control_center currently has a session with us,
// which is what makes a setting change deliverable.
func (s *Server) Connected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.session != nil
}

func (s *Server) publishProperties(values map[string]any) error {
	s.mu.Lock()
	sess := s.session
	s.mu.Unlock()
	if sess == nil {
		return fmt.Errorf("the robot is not connected to the local cloud; settings can only be changed while cloud connectivity is disabled")
	}

	s.seq++
	body, err := json.Marshal(map[string]any{
		"code":     1,
		"deviceID": deviceIDFromTopic(sess.topic),
		"version":  "1.0",
		"sign":     "",
		"payload": map[string]any{
			"property":  values,
			"seq":       s.seq,
			"timestamp": time.Now().UnixMilli(),
		},
	})
	if err != nil {
		return err
	}
	if _, err := sess.conn.Write(encodePublish(sess.topic, body)); err != nil {
		return fmt.Errorf("publish propertySet: %w", err)
	}
	return nil
}

// deviceIDFromTopic pulls the id out of v1_1/<model>/<deviceID>/propertySet.
func deviceIDFromTopic(topic string) string {
	parts := strings.Split(topic, "/")
	if len(parts) < 4 {
		return ""
	}
	return parts[len(parts)-2]
}
