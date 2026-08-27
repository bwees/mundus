package robot

import "testing"

func TestWorkingStatusToHA(t *testing.T) {
	cases := map[int]string{
		2:  "docked",
		3:  "docked",
		36: "docked",
		8:  "cleaning",
		9:  "cleaning",
		10: "cleaning",
		11: "paused",
		32: "paused",
		15: "returning",
		13: "error",
		1:  "idle",
		99: "idle",
	}
	for ws, want := range cases {
		if got := workingStatusToHA(ws); got != want {
			t.Errorf("workingStatusToHA(%d) = %q, want %q", ws, got, want)
		}
	}
}

func TestFanRoundTrip(t *testing.T) {
	for _, speed := range []string{"Quiet", "Standard", "Strong", "Max"} {
		level, ok := FanSpeedToLevel(speed)
		if !ok {
			t.Fatalf("FanSpeedToLevel(%q) not ok", speed)
		}
		if got := FanLevelToHA(level); got != speed {
			t.Errorf("round trip %q -> %d -> %q", speed, level, got)
		}
	}
	if _, ok := FanSpeedToLevel("turbo"); ok {
		t.Error("FanSpeedToLevel(turbo) should not be ok")
	}
}

func TestParseBrainStatus(t *testing.T) {
	// abridged real dump from a live S20 (idle, on the floor).
	out := `Terminated
{
  "battery_percent": 100,
  "curr_running_state": "Sleeping",
  "fan_level": 0,
  "is_charging": false,
  "is_on_base": false
}`
	b, ok := parseBrainStatus(out)
	if !ok {
		t.Fatal("expected parse ok")
	}
	if b.BatteryPercent == nil || *b.BatteryPercent != 100 {
		t.Errorf("battery = %v want 100", b.BatteryPercent)
	}
	if b.FanLevel == nil || *b.FanLevel != 0 {
		t.Errorf("fan_level = %v want 0", b.FanLevel)
	}
	if b.CurrRunningState != "Sleeping" {
		t.Errorf("curr_running_state = %q want Sleeping", b.CurrRunningState)
	}
	if _, ok := parseBrainStatus("no json here"); ok {
		t.Error("expected parse failure on non-json")
	}
}

func TestParseIotState(t *testing.T) {
	out := `{"cloud_conn":{"is_connected":true},"srv":{"state":21,"task_type":"none"}}`
	s, ok := parseIotState(out)
	if !ok {
		t.Fatal("expected parse ok")
	}
	if s.Srv.State != 21 {
		t.Errorf("srv.state = %d want 21", s.Srv.State)
	}
	if !s.CloudConn.IsConnected {
		t.Error("cloud_conn.is_connected should be true")
	}
	if workingStatusToHA(s.Srv.State) != "idle" {
		t.Errorf("state 21 should map to idle")
	}
}

func TestRunStateToHA(t *testing.T) {
	cases := map[string]string{
		"Sleeping":        "idle",
		"Standby":         "idle",
		"Sweeping":        "cleaning",
		"Mopping":         "cleaning",
		"Pause":           "paused",
		"BackingToCharge": "returning",
		"Charging":        "docked",
		"Fault":           "error",
	}
	for in, want := range cases {
		if got := runStateToHA(in); got != want {
			t.Errorf("runStateToHA(%q) = %q want %q", in, got, want)
		}
	}
}
