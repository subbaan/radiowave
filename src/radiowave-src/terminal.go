package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

type winSize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

func terminalSize() (int, int) {
	ws := &winSize{}
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, os.Stdout.Fd(), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(ws)))
	if errno == 0 && ws.Col > 0 && ws.Row > 0 {
		return int(ws.Col), int(ws.Row)
	}
	return 120, 36
}

type Terminal struct {
	savedState string
}

func rawModeArgs() []string {
	// stty raw disables output post-processing as well as canonical input.
	// Keep input raw, but restore OPOST+ONLCR so every \n emitted by the
	// renderer returns to column 0 instead of continuing from the right edge.
	return []string{"raw", "-echo", "min", "0", "time", "1", "opost", "onlcr"}
}

func (t *Terminal) Enter() error {
	cmdState := exec.Command("stty", "-g")
	cmdState.Stdin = os.Stdin
	out, err := cmdState.Output()
	if err != nil {
		return fmt.Errorf("stty -g: %w", err)
	}
	t.savedState = strings.TrimSpace(string(out))
	cmd := exec.Command("stty", rawModeArgs()...)
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("stty raw: %w", err)
	}
	fmt.Print("\x1b[?1049h\x1b[?25l\x1b[?7l\x1b[2J\x1b[H")
	return nil
}

func (t *Terminal) Leave() {
	fmt.Print("\x1b[0m\x1b[?7h\x1b[?25h\x1b[?1049l")
	if t.savedState != "" {
		cmd := exec.Command("stty", t.savedState)
		cmd.Stdin = os.Stdin
		_ = cmd.Run()
	}
}

type Key struct {
	Name string
	Rune rune
}

func readKeys(ch chan<- Key, done <-chan struct{}) {
	one := make([]byte, 1)
	tail := make([]byte, 7)
	for {
		select {
		case <-done:
			return
		default:
		}
		n, err := os.Stdin.Read(one)
		if err != nil || n == 0 {
			continue
		}
		b := one[0]
		if b == 0x1b {
			n2, _ := os.Stdin.Read(tail)
			seq := string(append([]byte{b}, tail[:n2]...))
			switch seq {
			case "\x1b[A":
				ch <- Key{Name: "up"}
			case "\x1b[B":
				ch <- Key{Name: "down"}
			case "\x1b[C":
				ch <- Key{Name: "right"}
			case "\x1b[D":
				ch <- Key{Name: "left"}
			case "\x1b[3~":
				ch <- Key{Name: "delete"}
			case "\x1b[5~":
				ch <- Key{Name: "pgup"}
			case "\x1b[6~":
				ch <- Key{Name: "pgdn"}
			case "\x1b[H", "\x1bOH":
				ch <- Key{Name: "home"}
			case "\x1b[F", "\x1bOF":
				ch <- Key{Name: "end"}
			default:
				ch <- Key{Name: "esc"}
			}
			continue
		}
		switch b {
		case 3:
			ch <- Key{Name: "ctrl+c"}
		case 9:
			ch <- Key{Name: "tab"}
		case 13, 10:
			ch <- Key{Name: "enter"}
		case 127, 8:
			ch <- Key{Name: "backspace"}
		default:
			if b >= 32 && b < 127 {
				ch <- Key{Name: strconv.Itoa(int(b)), Rune: rune(b)}
			}
		}
	}
}
