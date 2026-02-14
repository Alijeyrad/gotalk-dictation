package ui

import (
	"context"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
)

type popState int

const (
	stHidden     popState = iota
	stListening           // pulsing red dot
	stProcessing          // spinning blue arc
	stDone                // green flash, then auto-hide
	stError               // red flash, then auto-hide
)

const (
	popSz = 44
	popCX = popSz / 2
	popCY = popSz / 2
	popBG = uint32(0x1C1C1E)
)

type X11Popup struct {
	mu     sync.Mutex
	conn   *xgb.Conn
	screen *xproto.ScreenInfo
	wid    xproto.Window
	gc     xproto.Gcontext
	state  popState
	stopCh chan struct{}

	hasFont  bool
	textFont xproto.Font
	textGC   xproto.Gcontext
	preview  string
}

func newX11Popup() (*X11Popup, error) {
	conn, err := xgb.NewConn()
	if err != nil {
		return nil, fmt.Errorf("X11: %w", err)
	}
	setup := xproto.Setup(conn)
	screen := setup.DefaultScreen(conn)
	p := &X11Popup{conn: conn, screen: screen, state: stHidden, stopCh: make(chan struct{})}
	if err := p.init(); err != nil {
		conn.Close()
		return nil, err
	}
	go p.eventLoop()
	go p.renderLoop()
	return p, nil
}

func (p *X11Popup) init() error {
	wid, err := xproto.NewWindowId(p.conn)
	if err != nil {
		return err
	}
	p.wid = wid

	if err := xproto.CreateWindowChecked(p.conn, p.screen.RootDepth, wid, p.screen.Root,
		0, 0, popSz, popSz, 0,
		xproto.WindowClassInputOutput, p.screen.RootVisual,
		xproto.CwBackPixel|xproto.CwOverrideRedirect|xproto.CwEventMask,
		[]uint32{
			popBG,
			1, // override_redirect: bypass WM so there's no title bar and no focus steal
			xproto.EventMaskExposure,
		},
	).Check(); err != nil {
		return fmt.Errorf("create window: %w", err)
	}

	// Best-effort compositor hints; non-fatal if the WM doesn't support them.
	p.setAtomProp("_NET_WM_WINDOW_TYPE", "_NET_WM_WINDOW_TYPE_NOTIFICATION")
	p.setAtomProp("_NET_WM_STATE", "_NET_WM_STATE_ABOVE")

	gc, err := xproto.NewGcontextId(p.conn)
	if err != nil {
		return err
	}
	p.gc = gc
	if err := xproto.CreateGCChecked(p.conn, gc, xproto.Drawable(wid),
		xproto.GcForeground|xproto.GcBackground|xproto.GcLineWidth,
		[]uint32{0xFFFFFF, popBG, 2},
	).Check(); err != nil {
		return err
	}

	// Try to load a server-side font for the Done-state text preview.
	// Fails silently on systems without X11 fonts; falls back to circle.
	if fid, err := xproto.NewFontId(p.conn); err == nil {
		if xproto.OpenFontChecked(p.conn, fid, uint16(len("fixed")), "fixed").Check() == nil {
			if tgc, err2 := xproto.NewGcontextId(p.conn); err2 == nil {
				if xproto.CreateGCChecked(p.conn, tgc, xproto.Drawable(wid),
					xproto.GcForeground|xproto.GcBackground|xproto.GcFont,
					[]uint32{0xFFFFFF, popBG, uint32(fid)},
				).Check() == nil {
					p.hasFont = true
					p.textFont = fid
					p.textGC = tgc
				}
			}
		}
	}
	return nil
}

func (p *X11Popup) setAtomProp(prop, val string) {
	pr, err := xproto.InternAtom(p.conn, false, uint16(len(prop)), prop).Reply()
	if err != nil || pr.Atom == 0 {
		return
	}
	vr, err := xproto.InternAtom(p.conn, false, uint16(len(val)), val).Reply()
	if err != nil || vr.Atom == 0 {
		return
	}
	a := vr.Atom
	xproto.ChangeProperty(p.conn, xproto.PropModeReplace, p.wid, //nolint:errcheck
		pr.Atom, xproto.AtomAtom, 32, 1,
		[]byte{byte(a), byte(a >> 8), byte(a >> 16), byte(a >> 24)})
}

func (p *X11Popup) Show(s popState) {
	// Reset any leftover text preview from the previous Done state.
	p.mu.Lock()
	p.preview = ""
	p.mu.Unlock()
	xproto.ConfigureWindow(p.conn, p.wid, //nolint:errcheck
		xproto.ConfigWindowWidth|xproto.ConfigWindowHeight, []uint32{popSz, popSz})

	cx, cy := p.queryCaretPos()
	x := i16clamp(cx-popSz/2, 0, int16(p.screen.WidthInPixels)-popSz)
	y := i16clamp(cy-popSz-10, 0, int16(p.screen.HeightInPixels)-popSz)

	xproto.ConfigureWindow(p.conn, p.wid, //nolint:errcheck
		xproto.ConfigWindowX|xproto.ConfigWindowY, []uint32{uint32(x), uint32(y)})
	xproto.MapWindow(p.conn, p.wid)       //nolint:errcheck
	xproto.ConfigureWindow(p.conn, p.wid, //nolint:errcheck
		xproto.ConfigWindowStackMode, []uint32{uint32(xproto.StackModeAbove)})

	p.mu.Lock()
	p.state = s
	p.mu.Unlock()
}

func (p *X11Popup) SetState(s popState) {
	p.mu.Lock()
	p.state = s
	p.mu.Unlock()
}

// ShowDone switches to the Done state and displays a short text preview.
// Only ASCII text is rendered; non-ASCII falls back to the green circle.
func (p *X11Popup) ShowDone(text string) {
	var preview string
	if p.hasFont {
		preview = asciiPreview(text, 13)
	}

	p.mu.Lock()
	p.preview = preview
	p.state = stDone
	p.mu.Unlock()

	if preview != "" {
		w := uint32(len(preview)*7 + 20)
		if w < 80 {
			w = 80
		}
		xproto.ConfigureWindow(p.conn, p.wid, //nolint:errcheck
			xproto.ConfigWindowWidth|xproto.ConfigWindowHeight, []uint32{w, 28})
	}
}

// asciiPreview returns the first maxRunes ASCII characters of s followed by
// "..." if truncated. Returns "" if s contains any non-ASCII character.
func asciiPreview(s string, maxRunes int) string {
	for _, r := range s {
		if r > 127 {
			return ""
		}
	}
	runes := []rune(s)
	if len(runes) > maxRunes {
		return string(runes[:maxRunes]) + "..."
	}
	return s
}

func (p *X11Popup) Hide() {
	p.mu.Lock()
	p.state = stHidden
	hadPreview := p.preview != ""
	p.preview = ""
	p.mu.Unlock()
	if hadPreview {
		xproto.ConfigureWindow(p.conn, p.wid, //nolint:errcheck
			xproto.ConfigWindowWidth|xproto.ConfigWindowHeight, []uint32{popSz, popSz})
	}
	xproto.UnmapWindow(p.conn, p.wid) //nolint:errcheck
}

func (p *X11Popup) Close() {
	select {
	case <-p.stopCh:
	default:
		close(p.stopCh)
	}
	if p.hasFont {
		xproto.CloseFont(p.conn, p.textFont) //nolint:errcheck
	}
	xproto.DestroyWindow(p.conn, p.wid) //nolint:errcheck
	p.conn.Close()
}

func (p *X11Popup) renderLoop() {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	frame := 0
	for {
		select {
		case <-ticker.C:
			p.mu.Lock()
			s := p.state
			preview := p.preview
			p.mu.Unlock()
			if s != stHidden {
				p.drawFrame(s, preview, frame)
				frame++
			} else {
				frame = 0
			}
		case <-p.stopCh:
			return
		}
	}
}

func (p *X11Popup) eventLoop() {
	for {
		ev, err := p.conn.WaitForEvent()
		if err != nil || ev == nil {
			return
		}
		if _, ok := ev.(xproto.ExposeEvent); ok {
			p.mu.Lock()
			s := p.state
			preview := p.preview
			p.mu.Unlock()
			if s != stHidden {
				p.drawFrame(s, preview, 0)
			}
		}
		select {
		case <-p.stopCh:
			return
		default:
		}
	}
}

func (p *X11Popup) drawFrame(s popState, preview string, frame int) {
	d := xproto.Drawable(p.wid)
	clearW, clearH := uint16(popSz), uint16(popSz)
	if preview != "" && s == stDone {
		clearW = uint16(len(preview)*7 + 20)
		if clearW < 80 {
			clearW = 80
		}
		clearH = 28
	}
	p.setFG(popBG)
	xproto.PolyFillRectangle(p.conn, d, p.gc, //nolint:errcheck
		[]xproto.Rectangle{{X: 0, Y: 0, Width: clearW, Height: clearH}})

	switch s {
	case stListening:
		p.drawListening(frame)
	case stProcessing:
		p.drawProcessing(frame)
	case stDone:
		p.drawFlash(0x30D158, preview)
	case stError:
		p.drawFlash(0xFF3B30, "")
	}
}

func (p *X11Popup) drawListening(frame int) {
	t := float64(frame) * 2 * math.Pi / 40
	r := int(12 + 5*math.Sin(t))
	p.fillCircle(popCX, popCY, r, 0xFF3B30)
}

func (p *X11Popup) drawProcessing(frame int) {
	const (
		arcR    = uint16(17)
		lineW   = uint32(4)
		arcX    = int16(popCX) - int16(arcR)
		arcY    = int16(popCY) - int16(arcR)
		arcWH   = arcR * 2
		fullArc = int16(360 * 64)
		sweep   = int16(100 * 64)
	)

	d := xproto.Drawable(p.wid)

	xproto.ChangeGC(p.conn, p.gc, xproto.GcForeground|xproto.GcLineWidth, //nolint:errcheck
		[]uint32{0x0A3060, lineW})
	xproto.PolyArc(p.conn, d, p.gc, []xproto.Arc{{ //nolint:errcheck
		X: arcX, Y: arcY, Width: arcWH, Height: arcWH,
		Angle1: 0, Angle2: fullArc,
	}})

	deg := int16((frame * 18) % 360)
	xproto.ChangeGC(p.conn, p.gc, xproto.GcForeground, []uint32{0x0A84FF}) //nolint:errcheck
	xproto.PolyArc(p.conn, d, p.gc, []xproto.Arc{{                         //nolint:errcheck
		X: arcX, Y: arcY, Width: arcWH, Height: arcWH,
		Angle1: deg * 64, Angle2: sweep,
	}})
}

func (p *X11Popup) drawFlash(color uint32, preview string) {
	if preview != "" {
		d := xproto.Drawable(p.wid)
		// Small colored dot on the left, then the preview text.
		p.setFG(color)
		xproto.PolyFillArc(p.conn, d, p.gc, []xproto.Arc{{ //nolint:errcheck
			X: 5, Y: 8, Width: 12, Height: 12, Angle1: 0, Angle2: 360 * 64,
		}})
		xproto.ImageText8(p.conn, uint8(len(preview)), d, p.textGC, 22, 19, preview) //nolint:errcheck
		return
	}
	p.fillCircle(popCX, popCY, 17, color)
}

func (p *X11Popup) fillCircle(x, y, r int, color uint32) {
	p.setFG(color)
	xproto.PolyFillArc(p.conn, xproto.Drawable(p.wid), p.gc, //nolint:errcheck
		[]xproto.Arc{{
			X: int16(x - r), Y: int16(y - r),
			Width: uint16(r * 2), Height: uint16(r * 2),
			Angle1: 0, Angle2: 360 * 64,
		}})
}

func (p *X11Popup) setFG(c uint32) {
	xproto.ChangeGC(p.conn, p.gc, xproto.GcForeground, []uint32{c}) //nolint:errcheck
}

// queryCaretPos returns the best-guess screen position for the text caret.
// Strategy: AT-SPI2 (exact caret) → xdotool focused window top → mouse pointer.
func (p *X11Popup) queryCaretPos() (int16, int16) {
	if x, y, ok := queryCaretViaAtspi(); ok {
		return int16(x), int16(y)
	}

	if out, err := exec.Command("xdotool", "getwindowfocus", "getwindowgeometry", "--shell").Output(); err == nil {
		vals := parseShellVars(string(out))
		x, y, w := vals["X"], vals["Y"], vals["WIDTH"]
		if w > 0 {
			return int16(x + w/2), int16(y + 40)
		}
	}

	if r, err := xproto.QueryPointer(p.conn, p.screen.Root).Reply(); err == nil {
		return r.RootX, r.RootY
	}
	return int16(p.screen.WidthInPixels / 2), int16(p.screen.HeightInPixels / 2)
}

// queryCaretViaAtspi shells out to python3+gi to read the focused widget's
// caret position from the AT-SPI2 accessibility tree.
func queryCaretViaAtspi() (x, y int, ok bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	out, err := exec.CommandContext(ctx, "python3", "-c", atspiScript).Output()
	if err != nil {
		return 0, 0, false
	}
	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) != 2 {
		return 0, 0, false
	}
	px, err1 := strconv.Atoi(parts[0])
	py, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return px, py, true
}

const atspiScript = `
import sys
try:
    import gi
    gi.require_version('Atspi', '2.0')
    from gi.repository import Atspi
    desktop = Atspi.get_desktop(0)
    def find(obj, d=0):
        if d > 20: return None
        try:
            ss = obj.get_state_set()
            if ss.contains(Atspi.StateType.FOCUSED):
                try:
                    t = obj.get_text_iface()
                    if t:
                        o = t.get_caret_offset()
                        e = t.get_character_extents(o, Atspi.CoordType.SCREEN)
                        if e.x > 0 or e.y > 0:
                            return (e.x, e.y)
                except: pass
                try:
                    c = obj.get_component_iface()
                    if c:
                        e = c.get_extents(Atspi.CoordType.SCREEN)
                        if e.width > 0:
                            return (e.x + e.width // 2, e.y)
                except: pass
            for j in range(obj.get_child_count()):
                ch = obj.get_child_at_index(j)
                if ch:
                    r = find(ch, d + 1)
                    if r: return r
        except: pass
        return None
    for i in range(desktop.get_child_count()):
        app = desktop.get_child_at_index(i)
        if app:
            r = find(app)
            if r:
                print(r[0], r[1])
                sys.exit(0)
    sys.exit(1)
except:
    sys.exit(1)
`

func parseShellVars(s string) map[string]int {
	m := make(map[string]int)
	for _, line := range strings.Split(s, "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
				m[k] = n
			}
		}
	}
	return m
}

func i16clamp(v, lo, hi int16) int16 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
