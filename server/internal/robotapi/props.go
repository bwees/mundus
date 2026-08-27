package robotapi

import (
	"encoding/json"
	"os"
	"strconv"
	"sync"
)

// Properties persists device settings by merging into control_center's property
// cache (a flat {"<id>": value} JSON map). These are the shadow/property settings
// with no local funcRequest route (volume, drying, dust, carpet, child-lock, …).
// The brain reads these values when it runs the relevant sequence.
type Properties struct {
	path string
	mu   sync.Mutex
}

func NewProperties(path string) *Properties { return &Properties{path: path} }

func (p *Properties) All() (map[string]any, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.readLocked()
}

func (p *Properties) readLocked() (map[string]any, error) {
	m := map[string]any{}
	data, err := os.ReadFile(p.path)
	if err != nil {
		if os.IsNotExist(err) {
			return m, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return m, nil
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func (p *Properties) Get(id int) (any, bool) {
	m, err := p.All()
	if err != nil {
		return nil, false
	}
	v, ok := m[strconv.Itoa(id)]
	return v, ok
}

func (p *Properties) Set(id int, v any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	m, err := p.readLocked()
	if err != nil {
		return err
	}
	m[strconv.Itoa(id)] = v
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := p.path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p.path)
}

// Numbers decode out of JSON as float64.
func (p *Properties) GetInt(id int, def int) int {
	v, ok := p.Get(id)
	if !ok {
		return def
	}
	switch n := v.(type) {
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

func (p *Properties) GetBool(id int, def bool) bool {
	v, ok := p.Get(id)
	if !ok {
		return def
	}
	switch n := v.(type) {
	case bool:
		return n
	case float64:
		return n != 0
	}
	return def
}
