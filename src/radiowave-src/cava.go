package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// cavaBars is the resolution requested from cava. The renderer downsamples
// this to whatever width is actually on screen.
const cavaBars = 128

// CavaVisualizer drives github.com/karlstav/cava in raw-output mode and
// parses its ascii bar stream. It captures whatever is playing through the
// system's default audio sink (via cava's own pulse/pipewire input), so it
// does not need any special wiring to mpv.
type CavaVisualizer struct {
	mu     sync.Mutex
	levels []float64

	cancel     context.CancelFunc
	cmd        *exec.Cmd
	configPath string
}

func cavaAvailable() bool {
	_, err := exec.LookPath("cava")
	return err == nil
}

func NewCavaVisualizer() *CavaVisualizer {
	return &CavaVisualizer{levels: make([]float64, cavaBars)}
}

func (v *CavaVisualizer) Name() string { return "cava" }
func (v *CavaVisualizer) Demo() bool   { return false }

func (v *CavaVisualizer) Start() error {
	if _, err := exec.LookPath("cava"); err != nil {
		return fmt.Errorf("cava not found in PATH")
	}
	configPath := filepath.Join(os.TempDir(), fmt.Sprintf("radiowave-cava-%d.conf", os.Getpid()))
	config := fmt.Sprintf(`[general]
bars = %d
framerate = 30

[input]
method = pulse
source = auto

[output]
method = raw
channels = mono
mono_option = average
reverse = 0
raw_target = /dev/stdout
data_format = ascii
ascii_max_range = 100
bar_delimiter = 59
frame_delimiter = 10
`, cavaBars)
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "cava", "-p", configPath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		_ = os.Remove(configPath)
		return err
	}
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		cancel()
		_ = os.Remove(configPath)
		return err
	}

	v.configPath = configPath
	v.cancel = cancel
	v.cmd = cmd
	go v.readLoop(stdout)
	return nil
}

func (v *CavaVisualizer) readLoop(stdout io.Reader) {
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 16*1024), 16*1024)
	for scanner.Scan() {
		fields := strings.Split(strings.TrimRight(scanner.Text(), ";"), ";")
		levels := make([]float64, 0, len(fields))
		for _, f := range fields {
			if f == "" {
				continue
			}
			n, err := strconv.Atoi(f)
			if err != nil {
				continue
			}
			levels = append(levels, clamp01(float64(n)/100.0))
		}
		if len(levels) == 0 {
			continue
		}
		v.mu.Lock()
		v.levels = levels
		v.mu.Unlock()
	}
}

func (v *CavaVisualizer) Stop() {
	if v.cancel != nil {
		v.cancel()
	}
	if v.cmd != nil && v.cmd.Process != nil {
		_ = v.cmd.Process.Kill()
		_, _ = v.cmd.Process.Wait()
	}
	if v.configPath != "" {
		_ = os.Remove(v.configPath)
	}
	v.cmd, v.cancel, v.configPath = nil, nil, ""
}

func (v *CavaVisualizer) Levels(n int) []float64 {
	if n < 1 {
		return nil
	}
	v.mu.Lock()
	src := v.levels
	v.mu.Unlock()
	if len(src) == 0 {
		return make([]float64, n)
	}
	out := make([]float64, n)
	for i := range out {
		idx := i * len(src) / n
		if idx >= len(src) {
			idx = len(src) - 1
		}
		out[i] = src[idx]
	}
	return out
}
