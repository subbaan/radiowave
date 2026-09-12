package main

import (
	"os"
	"testing"
	"time"
)

// TestCavaVisualizerReal exercises the real cava binary against whatever
// audio source is available. It's opt-in (not run by default) since it
// depends on cava being installed and reachable — set
// RADIOWAVE_TEST_CAVA=1 to run it.
func TestCavaVisualizerReal(t *testing.T) {
	if os.Getenv("RADIOWAVE_TEST_CAVA") == "" {
		t.Skip("set RADIOWAVE_TEST_CAVA=1 to exercise the real cava binary")
	}
	if !cavaAvailable() {
		t.Skip("cava not found in PATH")
	}
	v := NewCavaVisualizer()
	if err := v.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer v.Stop()

	time.Sleep(1500 * time.Millisecond)
	levels := v.Levels(32)
	if len(levels) != 32 {
		t.Fatalf("expected 32 levels, got %d", len(levels))
	}
	for i, l := range levels {
		if l < 0 || l > 1 {
			t.Fatalf("level %d out of [0,1] range: %v", i, l)
		}
	}
	if v.Demo() {
		t.Fatalf("CavaVisualizer must report Demo() == false")
	}
}
