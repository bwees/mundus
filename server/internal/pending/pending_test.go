package pending

import (
	"testing"
	"time"
)

const window = 10 * time.Second

func TestReportsExpectedUntilObservationMoves(t *testing.T) {
	v := New[string](window)
	v.Resolve("docked")
	v.Expect("cleaning")

	if got := v.Resolve("docked"); got != "cleaning" {
		t.Fatalf("Resolve = %q, want cleaning", got)
	}
	if got := v.Resolve("cleaning"); got != "cleaning" {
		t.Fatalf("Resolve = %q, want cleaning", got)
	}
	if v.active {
		t.Fatal("expectation should end once the device confirms it")
	}
}

func TestYieldsToAnUnexpectedObservation(t *testing.T) {
	v := New[string](window)
	v.Resolve("cleaning")
	v.Expect("idle")

	if got := v.Resolve("returning"); got != "returning" {
		t.Fatalf("Resolve = %q, want returning", got)
	}
	if v.active {
		t.Fatal("expectation should end once the device moves elsewhere")
	}
}

func TestExpiresAfterWindow(t *testing.T) {
	v := New[bool](window)
	v.Resolve(false)
	v.Expect(true)
	v.at = time.Now().Add(-window - time.Second)

	if got := v.Resolve(false); got != false {
		t.Fatalf("Resolve = %v, want false", got)
	}
	if v.active {
		t.Fatal("expectation should end once the window elapses")
	}
}

func TestPassesThroughWithoutAnExpectation(t *testing.T) {
	v := New[string](window)
	if got := v.Resolve("docked"); got != "docked" {
		t.Fatalf("Resolve = %q, want docked", got)
	}
}
