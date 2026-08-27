package robot

import "testing"

func TestCleanTypeRoundTrip(t *testing.T) {
	for _, label := range CleanTypeOptions {
		wire, ok := CleanTypeWire(label)
		if !ok {
			t.Fatalf("CleanTypeWire(%q) not ok", label)
		}
		if got := CleanTypeLabel(wire); got != label {
			t.Errorf("round trip %q -> %q -> %q", label, wire, got)
		}
	}
	if _, ok := CleanTypeWire("Nope"); ok {
		t.Error("CleanTypeWire(Nope) should not be ok")
	}
}

func TestWaterLevelRoundTrip(t *testing.T) {
	for _, label := range WaterLevelOptions {
		v, ok := WaterLevelValue(label)
		if !ok {
			t.Fatalf("WaterLevelValue(%q) not ok", label)
		}
		if got := WaterLevelLabel(v); got != label {
			t.Errorf("round trip %q -> %d -> %q", label, v, got)
		}
	}
	if _, ok := WaterLevelValue("Flood"); ok {
		t.Error("WaterLevelValue(Flood) should not be ok")
	}
}

func TestPassesRoundTrip(t *testing.T) {
	for _, label := range PassesOptions {
		v, ok := PassesValue(label)
		if !ok {
			t.Fatalf("PassesValue(%q) not ok", label)
		}
		if got := PassesLabel(v); got != label {
			t.Errorf("round trip %q -> %d -> %q", label, v, got)
		}
	}
	if _, ok := PassesValue("3"); ok {
		t.Error("PassesValue(3) should not be ok")
	}
}
