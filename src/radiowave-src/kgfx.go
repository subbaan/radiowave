package main

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Pixel visualizers are presented through the kitty graphics protocol. The
// cell renderer stays the source of truth for layout: it reserves the
// visualizer rectangle as blank cells and records where it is (App.pixRect),
// then each frame is placed there as one RGBA image at z=-1 — above the cell
// backgrounds, below the text — so labels still draw on top of it.
//
// Two transports, as in bunzi:
//   - shm: the full-resolution frame is written straight into a POSIX shared
//     memory object and kitty maps it. No base64, no pty traffic.
//   - direct: the canvas-resolution frame is zlib'd and base64'd through the
//     tty, and the terminal scales it into the cell rectangle. Slower, but
//     works anywhere the protocol does (e.g. over ssh).

type gfxTransport int

const (
	gfxOff gfxTransport = iota
	gfxSHM
	gfxDirect
)

func (t gfxTransport) String() string {
	switch t {
	case gfxSHM:
		return "shm"
	case gfxDirect:
		return "direct"
	default:
		return "off"
	}
}

const (
	gfxImageID = 7341
	// kitty unlinks every shm object it reads; the ring only mops up any
	// it never got to (e.g. frames dropped during a resize).
	gfxShmRing  = 8
	gfxChunk    = 4096
	gfxProbeFor = 1500 * time.Millisecond
)

type kittyGfx struct {
	tx     gfxTransport
	seq    uint64
	placed bool
	shown  pixShape // what the placed image was rendered for
	staged []byte   // direct transport: raw RGBA frame
	zbuf   bytes.Buffer
	zw     *zlib.Writer
}

// gfxProbe is what the terminal answered to the startup capability query.
type gfxProbe struct {
	direct, shm  bool
	cellW, cellH int
	answered     bool
}

func shmName(seq uint64) string { return fmt.Sprintf("radiowave-%d-%d", os.Getpid(), seq) }

func shmPath(name string) string { return "/dev/shm/" + name }

var (
	probeGfxRe  = regexp.MustCompile(`\x1b_Gi=(\d+)(?:,[^;]*)?;([^\x1b]*)\x1b\\`)
	probeCellRe = regexp.MustCompile(`\x1b\[6;(\d+);(\d+)t`)
	probeDARe   = regexp.MustCompile(`\x1b\[\?[0-9;]*c`)
)

// probeGraphics asks the terminal whether it speaks the graphics protocol
// (direct and shared-memory transmission) and for its cell size in pixels.
// DA1 is appended because every terminal answers it, so it bounds the wait.
// It must run in raw mode before the key reader starts, or the replies
// would be read as keypresses.
func probeGraphics() gfxProbe {
	var res gfxProbe
	probeName := fmt.Sprintf("radiowave-probe-%d", os.Getpid())
	shmMade := os.WriteFile(shmPath(probeName), []byte{255, 255, 255, 255}, 0o600) == nil
	defer os.Remove(shmPath(probeName)) // kitty normally unlinks it already

	var q strings.Builder
	q.WriteString("\x1b_Gi=31,s=1,v=1,a=q,t=d,f=24;AAAA\x1b\\")
	if shmMade {
		fmt.Fprintf(&q, "\x1b_Gi=32,s=1,v=1,a=q,t=s,f=32;%s\x1b\\",
			base64.StdEncoding.EncodeToString([]byte("/"+probeName)))
	}
	q.WriteString("\x1b[16t\x1b[c")
	os.Stdout.WriteString(q.String())

	// Raw mode is "min 0 time 1", so each Read returns within 100ms.
	var got []byte
	buf := make([]byte, 256)
	deadline := time.Now().Add(gfxProbeFor)
	for time.Now().Before(deadline) {
		n, _ := os.Stdin.Read(buf)
		got = append(got, buf[:n]...)
		if probeDARe.Match(got) {
			res.answered = true
			break
		}
	}
	for _, m := range probeGfxRe.FindAllSubmatch(got, -1) {
		ok := string(m[2]) == "OK"
		switch string(m[1]) {
		case "31":
			res.direct = ok
		case "32":
			res.shm = ok
		}
	}
	if m := probeCellRe.FindSubmatch(got); m != nil {
		res.cellH, _ = strconv.Atoi(string(m[1]))
		res.cellW, _ = strconv.Atoi(string(m[2]))
	}
	return res
}

// chooseTransport resolves the --gfx request against what the probe found.
func chooseTransport(want string, p gfxProbe) gfxTransport {
	switch want {
	case "off":
		return gfxOff
	case "direct":
		if p.direct {
			return gfxDirect
		}
		return gfxOff
	case "shm":
		if p.shm {
			return gfxSHM
		}
		return gfxOff
	}
	switch {
	case p.shm:
		return gfxSHM
	case p.direct:
		return gfxDirect
	}
	return gfxOff
}

// cellPixels reports the terminal cell size, preferring TIOCGWINSZ and
// falling back to the probed CSI 16 t answer.
func cellPixels(cols, rows int, p gfxProbe) (int, int) {
	ws := &winSize{}
	if ioctlWinSize(ws) && ws.Xpixel > 0 && ws.Ypixel > 0 && cols > 0 && rows > 0 {
		return int(ws.Xpixel) / cols, int(ws.Ypixel) / rows
	}
	return p.cellW, p.cellH
}

// frame returns the escape sequence that shows cv in rect, upscaled by
// scale (shm) or by the terminal (direct). outW/outH are the rectangle's
// size in pixels.
func (g *kittyGfx) frame(cv *pixCanvas, scale, outW, outH int, r vizRect) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "\x1b[%d;%dH", r.top, r.left)
	keys := fmt.Sprintf("a=T,f=32,i=%d,p=1,q=2,C=1,z=-1,c=%d,r=%d", gfxImageID, r.cols, r.rows)
	switch g.tx {
	case gfxSHM:
		if err := g.sendSHM(&b, cv, scale, outW, outH, keys); err != nil {
			return "", err
		}
	case gfxDirect:
		if err := g.sendDirect(&b, cv, keys); err != nil {
			return "", err
		}
	default:
		return "", nil
	}
	g.seq++
	g.placed = true
	return b.String(), nil
}

func (g *kittyGfx) sendSHM(b *strings.Builder, cv *pixCanvas, scale, outW, outH int, keys string) error {
	if g.seq >= gfxShmRing {
		_ = os.Remove(shmPath(shmName(g.seq - gfxShmRing)))
	}
	name := shmName(g.seq)
	size := outW * outH * 4
	f, err := os.OpenFile(shmPath(name), os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := f.Truncate(int64(size)); err != nil {
		_ = os.Remove(shmPath(name))
		return err
	}
	m, err := syscall.Mmap(int(f.Fd()), 0, size, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		_ = os.Remove(shmPath(name))
		return err
	}
	cv.encodeRGBA(m, scale, outW, outH)
	_ = syscall.Munmap(m)
	fmt.Fprintf(b, "\x1b_G%s,s=%d,v=%d,t=s,S=%d;%s\x1b\\", keys, outW, outH, size,
		base64.StdEncoding.EncodeToString([]byte("/"+name)))
	return nil
}

func (g *kittyGfx) sendDirect(b *strings.Builder, cv *pixCanvas, keys string) error {
	size := cv.w * cv.h * 4
	if cap(g.staged) < size {
		g.staged = make([]byte, size)
	}
	g.staged = g.staged[:size]
	cv.encodeRGBA(g.staged, 1, cv.w, cv.h)

	g.zbuf.Reset()
	if g.zw == nil {
		g.zw, _ = zlib.NewWriterLevel(&g.zbuf, zlib.BestSpeed)
	} else {
		g.zw.Reset(&g.zbuf)
	}
	if _, err := g.zw.Write(g.staged); err != nil {
		return err
	}
	if err := g.zw.Close(); err != nil {
		return err
	}
	enc := base64.StdEncoding.EncodeToString(g.zbuf.Bytes())
	for off := 0; off < len(enc); off += gfxChunk {
		end := min(len(enc), off+gfxChunk)
		more := 0
		if end < len(enc) {
			more = 1
		}
		if off == 0 {
			fmt.Fprintf(b, "\x1b_G%s,s=%d,v=%d,t=d,o=z,m=%d;", keys, cv.w, cv.h, more)
		} else {
			fmt.Fprintf(b, "\x1b_Gm=%d;", more)
		}
		b.WriteString(enc[off:end])
		b.WriteString("\x1b\\")
	}
	return nil
}

// hide removes the placement and frees the image data, if anything is shown.
func (g *kittyGfx) hide() string {
	if !g.placed {
		return ""
	}
	g.placed = false
	return fmt.Sprintf("\x1b_Ga=d,d=I,i=%d,q=2\x1b\\", gfxImageID)
}

// cleanup removes any shm objects kitty has not consumed.
func (g *kittyGfx) cleanup() {
	from := uint64(0)
	if g.seq > gfxShmRing {
		from = g.seq - gfxShmRing
	}
	for s := from; s < g.seq; s++ {
		_ = os.Remove(shmPath(shmName(s)))
	}
}
