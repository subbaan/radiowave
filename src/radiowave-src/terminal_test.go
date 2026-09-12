package main

import (
	"slices"
	"testing"
)

func TestRawModeKeepsTerminalNewlineProcessing(t *testing.T) {
	args := rawModeArgs()
	if !slices.Contains(args, "opost") {
		t.Fatalf("raw terminal args must re-enable opost: %v", args)
	}
	if !slices.Contains(args, "onlcr") {
		t.Fatalf("raw terminal args must re-enable onlcr: %v", args)
	}
}
