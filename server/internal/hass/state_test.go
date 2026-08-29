package hass

import (
	"testing"
	"time"

	"github.com/bwees/mundus/server/internal/config"
	"github.com/bwees/mundus/server/internal/robot"
)

func TestDryingStateFollowsWorkStatus(t *testing.T) {
	b := New(config.Config{PollInterval: 10 * time.Second}, nil, nil, nil, nil, nil)

	if got := b.dryingState(robot.WorkStatusDryingMop); got != "ON" {
		t.Fatalf("dryingState = %q, want ON", got)
	}
	if got := b.dryingState(2); got != "OFF" {
		t.Fatalf("dryingState = %q, want OFF", got)
	}
}

func TestDryingStateReflectsJustIssuedCommand(t *testing.T) {
	b := New(config.Config{PollInterval: 10 * time.Second}, nil, nil, nil, nil, nil)
	b.dryingState(2)
	b.drying.Expect(true)

	if got := b.dryingState(2); got != "ON" {
		t.Fatalf("dryingState = %q, want ON before the device catches up", got)
	}
	if got := b.dryingState(robot.WorkStatusDryingMop); got != "ON" {
		t.Fatalf("dryingState = %q, want ON", got)
	}
}
