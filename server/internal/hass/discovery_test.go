package hass

import (
	"testing"

	"github.com/bwees/mundus/server/internal/config"
)

func testBridge() *Bridge {
	return &Bridge{cfg: config.Config{DiscoveryPrefix: "homeassistant", DeviceID: "switchbot_s20"}}
}

func TestDiscoveryTopic(t *testing.T) {
	b := testBridge()
	got := b.discoveryTopic("switch", "auto_dry_after_wash")
	want := "homeassistant/switch/switchbot_s20_auto_dry_after_wash/config"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
