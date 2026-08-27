package settings

import "testing"

type fakeProps map[int]any

func (f fakeProps) Set(id int, v any) error { f[id] = v; return nil }

func (f fakeProps) GetInt(id, def int) int {
	if v, ok := f[id].(int); ok {
		return v
	}
	return def
}

func (f fakeProps) GetBool(id int, def bool) bool {
	if v, ok := f[id].(bool); ok {
		return v
	}
	return def
}

func byKey(t *testing.T, key string) Setting {
	t.Helper()
	for _, s := range All() {
		if s.Key == key {
			return s
		}
	}
	t.Fatalf("no setting %q", key)
	return Setting{}
}

func TestKeysAndPropsAreUnique(t *testing.T) {
	keys := map[string]bool{}
	props := map[int]string{}
	for _, s := range All() {
		if keys[s.Key] {
			t.Errorf("duplicate key %q", s.Key)
		}
		keys[s.Key] = true
		if prev, ok := props[s.Prop]; ok {
			t.Errorf("property %d used by both %q and %q", s.Prop, prev, s.Key)
		}
		props[s.Prop] = s.Key
	}
}

func TestEverySettingIsWellFormed(t *testing.T) {
	for _, s := range All() {
		if s.Key == "" || s.Name == "" || s.Prop == 0 {
			t.Errorf("%+v is missing key, name or prop", s)
		}
		switch s.Kind {
		case Choice:
			if len(s.Options) < 2 {
				t.Errorf("%s is a choice with %d options", s.Key, len(s.Options))
			}
			if !s.HasValue(s.Default) {
				t.Errorf("%s default %d is not one of its options", s.Key, s.Default)
			}
		case Number:
			if s.Min >= s.Max {
				t.Errorf("%s range %d-%d is empty", s.Key, s.Min, s.Max)
			}
			if s.Default < s.Min || s.Default > s.Max {
				t.Errorf("%s default %d is outside %d-%d", s.Key, s.Default, s.Min, s.Max)
			}
		case Toggle:
			if len(s.Options) != 0 {
				t.Errorf("%s is a toggle with options", s.Key)
			}
		default:
			t.Errorf("%s has unknown kind %q", s.Key, s.Kind)
		}
	}
}

func TestToggleRoundTrip(t *testing.T) {
	p := fakeProps{}
	s := byKey(t, "auto_empty")
	for _, want := range []int{1, 0, 1} {
		if err := s.Write(p, want); err != nil {
			t.Fatal(err)
		}
		if got := s.Read(p); got != want {
			t.Errorf("wrote %d, read %d", want, got)
		}
	}
}

// Child lock displays the opposite of what it stores. Callers should never see
// that, but the stored property still has to be the inverted one.
func TestChildLockStoresInverted(t *testing.T) {
	p := fakeProps{}
	s := byKey(t, "child_lock")
	if !s.inverted {
		t.Fatal("child_lock lost its inverted flag")
	}
	if err := s.Write(p, 1); err != nil {
		t.Fatal(err)
	}
	if stored := p[1057]; stored != false {
		t.Errorf("child lock on stored %v, want false", stored)
	}
	if got := s.Read(p); got != 1 {
		t.Errorf("read back %d, want 1", got)
	}

	if err := s.Write(p, 0); err != nil {
		t.Fatal(err)
	}
	if stored := p[1057]; stored != true {
		t.Errorf("child lock off stored %v, want true", stored)
	}
	if got := s.Read(p); got != 0 {
		t.Errorf("read back %d, want 0", got)
	}
}

func TestChoiceRejectsUnlistedValue(t *testing.T) {
	p := fakeProps{}
	s := byKey(t, "dry_duration")
	if err := s.Write(p, 99); err == nil {
		t.Error("accepted a value that is not an option")
	}
	if err := s.Write(p, 4); err != nil {
		t.Errorf("rejected a listed value: %v", err)
	}
}

func TestNumberRejectsOutOfRange(t *testing.T) {
	p := fakeProps{}
	s := byKey(t, "volume")
	for _, v := range []int{-1, 101} {
		if err := s.Write(p, v); err == nil {
			t.Errorf("accepted volume %d", v)
		}
	}
	if err := s.Write(p, 60); err != nil {
		t.Errorf("rejected volume 60: %v", err)
	}
}

func TestUnsetSettingReadsItsDefault(t *testing.T) {
	p := fakeProps{}
	for _, s := range All() {
		if got := s.Read(p); got != s.Default {
			t.Errorf("%s read %d on an empty store, want default %d", s.Key, got, s.Default)
		}
	}
}

// The carpet and dust values that were previously wrong in the web UI.
func TestConfirmedVendorValues(t *testing.T) {
	carpet := byKey(t, "carpet_clean")
	if carpet.LabelOf(0) != "Adapt" || carpet.LabelOf(1) != "Avoid" {
		t.Errorf("carpet_clean labels are %v", carpet.Labels())
	}
	dust := byKey(t, "smart_dust")
	if got := dust.Labels(); got[0] != "Normal" || got[1] != "Fast" || got[2] != "Super" {
		t.Errorf("smart_dust labels are %v", got)
	}
}

func TestLabelValueRoundTrip(t *testing.T) {
	for _, s := range All() {
		if s.Kind != Choice {
			continue
		}
		for _, o := range s.Options {
			v, ok := s.ValueOf(o.Label)
			if !ok || v != o.Value {
				t.Errorf("%s: %q did not map back to %d", s.Key, o.Label, o.Value)
			}
			if s.LabelOf(o.Value) != o.Label {
				t.Errorf("%s: %d did not map back to %q", s.Key, o.Value, o.Label)
			}
		}
	}
}
