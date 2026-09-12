package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

type Player struct {
	cmd     *exec.Cmd
	socket  string
	volume  int
	muted   bool
	started bool
}

func NewPlayer() *Player {
	return &Player{volume: 80}
}

func (p *Player) Start() error {
	if p.started {
		return nil
	}
	if _, err := exec.LookPath("mpv"); err != nil {
		return errors.New("mpv not found in PATH")
	}
	p.socket = filepath.Join(os.TempDir(), "radiowave-"+strconv.Itoa(os.Getpid())+".sock")
	_ = os.Remove(p.socket)
	p.cmd = exec.Command("mpv",
		"--no-config",
		"--input-terminal=no",
		"--idle=yes",
		"--no-video",
		"--force-window=no",
		"--really-quiet",
		"--input-ipc-server="+p.socket,
		"--volume="+strconv.Itoa(p.volume),
	)
	p.cmd.Stdout = nil
	p.cmd.Stderr = nil
	if err := p.cmd.Start(); err != nil {
		return err
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(p.socket); err == nil {
			p.started = true
			return nil
		}
		time.Sleep(25 * time.Millisecond)
	}
	_ = p.cmd.Process.Kill()
	return fmt.Errorf("mpv IPC socket did not appear")
}

func (p *Player) Close() {
	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.command([]any{"quit"}, nil)
		_ = p.cmd.Process.Kill()
		_, _ = p.cmd.Process.Wait()
	}
	if p.socket != "" {
		_ = os.Remove(p.socket)
	}
	p.started = false
}

func (p *Player) command(cmd []any, out any) error {
	if !p.started && cmd[0] != "quit" {
		return errors.New("player not started")
	}
	conn, err := net.DialTimeout("unix", p.socket, 500*time.Millisecond)
	if err != nil {
		return err
	}
	defer conn.Close()
	payload, _ := json.Marshal(map[string]any{"command": cmd})
	if _, err := conn.Write(append(payload, '\n')); err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	line, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		return err
	}
	var resp struct {
		Data  json.RawMessage `json:"data"`
		Error string          `json:"error"`
	}
	if err := json.Unmarshal(line, &resp); err != nil {
		return err
	}
	if resp.Error != "success" {
		return errors.New(resp.Error)
	}
	return json.Unmarshal(resp.Data, out)
}

func (p *Player) Play(url string) error {
	return p.command([]any{"loadfile", url, "replace"}, nil)
}

func (p *Player) Stop() error {
	return p.command([]any{"stop"}, nil)
}

func (p *Player) ToggleMute() error {
	p.muted = !p.muted
	return p.command([]any{"set_property", "mute", p.muted}, nil)
}

func (p *Player) ChangeVolume(delta int) error {
	p.volume += delta
	if p.volume < 0 {
		p.volume = 0
	}
	if p.volume > 100 {
		p.volume = 100
	}
	return p.command([]any{"set_property", "volume", p.volume}, nil)
}

func (p *Player) Property(name string) string {
	var value string
	if err := p.command([]any{"get_property", name}, &value); err != nil {
		return ""
	}
	return value
}
