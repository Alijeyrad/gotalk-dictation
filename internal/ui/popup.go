package ui

// Pure X11 floating indicator — override-redirect, non-focusable, always on top.
// No text. Animated icons only. Positioned above the active window.

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
	stHidden    popState = iota
	stListening          // pulsing red dot
	stProcessing         // spinning blue arc
	stDone               // green flash, then auto-hide
	stError              // red flash, then auto-hide
)

const (
	popSz = 44 // window is a 44×44 square
	popCX = popSz / 2
	popCY = popSz / 2
	popBG = uint32(0x1C1C1E) // ~Apple dark bg
)

// X11Popup is a chromeless, non-focusable overlay drawn directly via X11.
type X11Popup struct {
	mu     sync.Mutex // guards state only
	conn   *xgb.Conn
	screen *xproto.ScreenInfo
	wid    xproto.Window
	gc     xproto.Gcontext

	state  popState
	stopCh chan struct{}
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
			1, // override_redirect: bypass WM — no title bar, no focus steal
			xproto.EventMaskExposure,
		},
	).Check(); err != nil {
		return fmt.Errorf("create window: %w", err)
	}

	// Compositor hints (best-effort; non-fatal if unsupported)
	p.setAtomProp("_NET_WM_WINDOW_TYPE", "_NET_WM_WINDOW_TYPE_NOTIFICATION")
	p.setAtomProp("_NET_WM_STATE", "_NET_WM_STATE_ABOVE")

	gc, err := xproto.NewGcontextId(p.conn)
	if err != nil {
		return err
	}
	p.gc = gc
	return xproto.CreateGCChecked(p.conn, gc, xproto.Drawable(wid),
		xproto.GcForeground|xproto.GcBackground|xproto.GcLineWidth,
		[]uint32{0xFFFFFF, popBG, 2},
	).Check()
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

// ---------- public API -------------------------------------------------------

// Show positions the popup near the text caret and starts the animation.
func (p *X11Popup) Show(s popState) {
	cx, cy := p.queryCaretPos()
	// Place popup so its bottom edge is 10px above the target point.
	x := i16clamp(cx-popSz/2, 0, int16(p.screen.WidthInPixels)-popSz)
	y := i16clamp(cy-popSz-10, 0, int16(p.screen.HeightInPixels)-popSz)

	xproto.ConfigureWindow(p.conn, p.wid, //nolint:errcheck
		xproto.ConfigWindowX|xproto.ConfigWindowY, []uint32{uint32(x), uint32(y)})
	xproto.MapWindow(p.conn, p.wid) //nolint:errcheck
	xproto.ConfigureWindow(p.conn, p.wid, //nolint:errcheck
		xproto.ConfigWindowStackMode, []uint32{uint32(xproto.StackModeAbove)})

	p.mu.Lock()
	p.state = s
	p.mu.Unlock()
}

// SetState changes animation without moving the window.
func (p *X11Popup) SetState(s popState) {
	p.mu.Lock()
	p.state = s
	p.mu.Unlock()
}

// Hide removes the popup.
func (p *X11Popup) Hide() {
	p.mu.Lock()
	p.state = stHidden
	p.mu.Unlock()
	xproto.UnmapWindow(p.conn, p.wid) //nolint:errcheck
}

// Close shuts down the popup.
func (p *X11Popup) Close() {
	select {
	case <-p.stopCh:
	default:
		close(p.stopCh)
	}
	xproto.DestroyWindow(p.conn, p.wid) //nolint:errcheck
	p.conn.Close()
}

// ---------- animation --------------------------------------------------------

// renderLoop drives animation at 20 fps.
func (p *X11Popup) renderLoop() {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	frame := 0
	for {
		select {
		case <-ticker.C:
			p.mu.Lock()
			s := p.state
			p.mu.Unlock()
			if s != stHidden {
				p.drawFrame(s, frame)
				frame++
			} else {
				frame = 0
			}
		case <-p.stopCh:
			return
		}
	}
}

// eventLoop handles Expose events so the window redraws after being uncovered.
func (p *X11Popup) eventLoop() {
	for {
		ev, err := p.conn.WaitForEvent()
		if err != nil || ev == nil {
			return
		}
		if _, ok := ev.(xproto.ExposeEvent); ok {
			p.mu.Lock()
			s := p.state
			p.mu.Unlock()
			if s != stHidden {
				p.drawFrame(s, 0)
			}
		}
		select {
		case <-p.stopCh:
			return
		default:
		}
	}
}

func (p *X11Popup) drawFrame(s popState, frame int) {
	d := xproto.Drawable(p.wid)

	// Clear background
	p.setFG(popBG)
	xproto.PolyFillRectangle(p.conn, d, p.gc, //nolint:errcheck
		[]xproto.Rectangle{{0, 0, popSz, popSz}})

	switch s {
	case stListening:
		p.drawListening(frame)
	case stProcessing:
		p.drawProcessing(frame)
	case stDone:
		p.drawFlash(0x30D158) // Apple green
	case stError:
		p.drawFlash(0xFF3B30) // Apple red
	}
}

// drawListening draws a pulsing red dot.
func (p *X11Popup) drawListening(frame int) {
	t := float64(frame) * 2 * math.Pi / 40 // period = 40 frames = 2 s
	r := int(12 + 5*math.Sin(t))            // radius 7–17 px
	p.fillCircle(popCX, popCY, r, 0xFF3B30)
}

// drawProcessing draws an iOS-style spinning arc.
func (p *X11Popup) drawProcessing(frame int) {
	const (
		arcR    = uint16(17)
		lineW   = uint32(4)
		arcX    = int16(popCX) - int16(arcR)
		arcY    = int16(popCY) - int16(arcR)
		arcWH   = arcR * 2
		fullArc = int16(360 * 64)
		sweep   = int16(100 * 64) // arc sweep in 1/64°
	)

	d := xproto.Drawable(p.wid)

	// Dim background ring
	xproto.ChangeGC(p.conn, p.gc, xproto.GcForeground|xproto.GcLineWidth, //nolint:errcheck
		[]uint32{0x0A3060, lineW})
	xproto.PolyArc(p.conn, d, p.gc, []xproto.Arc{{ //nolint:errcheck
		X: arcX, Y: arcY, Width: arcWH, Height: arcWH,
		Angle1: 0, Angle2: fullArc,
	}})

	// Bright spinning arc — 1 revolution/second at 20 fps (18°/frame)
	deg := int16((frame * 18) % 360)
	xproto.ChangeGC(p.conn, p.gc, xproto.GcForeground, []uint32{0x0A84FF}) //nolint:errcheck
	xproto.PolyArc(p.conn, d, p.gc, []xproto.Arc{{ //nolint:errcheck
		X: arcX, Y: arcY, Width: arcWH, Height: arcWH,
		Angle1: deg * 64, Angle2: sweep,
	}})
}

// drawFlash draws a solid filled circle (for Done/Error states).
func (p *X11Popup) drawFlash(color uint32) {
	p.fillCircle(popCX, popCY, 17, color)
}

// ---------- helpers ----------------------------------------------------------

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

// queryCaretPos returns the best guess for the text caret's screen position.
//
// Strategy (first success wins):
//  1. AT-SPI2 accessibility — queries the focused widget's caret extents
//     via Python+gi. Gives the exact character position.
//  2. xdotool getwindowfocus — center of the X11 input-focus window.
//  3. Mouse pointer — last resort.
func (p *X11Popup) queryCaretPos() (int16, int16) {
	// 1. AT-SPI2 caret position (exact).
	if x, y, ok := queryCaretViaAtspi(); ok {
		return int16(x), int16(y)
	}

	// 2. Top-center of focused window (just below title bar).
	if out, err := exec.Command("xdotool", "getwindowfocus", "getwindowgeometry", "--shell").Output(); err == nil {
		vals := parseShellVars(string(out))
		x, y, w := vals["X"], vals["Y"], vals["WIDTH"]
		if w > 0 {
			return int16(x + w/2), int16(y + 40)
		}
	}

	// 3. Mouse pointer.
	if r, err := xproto.QueryPointer(p.conn, p.screen.Root).Reply(); err == nil {
		return r.RootX, r.RootY
	}
	return int16(p.screen.WidthInPixels / 2), int16(p.screen.HeightInPixels / 2)
}

// queryCaretViaAtspi shells out to python3 with the AT-SPI2 GObject
// introspection bindings to walk the accessibility tree and return the
// screen coordinates of the text caret in the focused widget.
func queryCaretViaAtspi() (x, y int, ok bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", "-c", atspiScript)
	out, err := cmd.Output()
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

// parseShellVars parses KEY=VALUE lines (like xdotool --shell output).
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
