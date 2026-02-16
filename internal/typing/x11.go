package typing

import (
	"fmt"
	"time"
	"unicode"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
	"github.com/jezek/xgb/xtest"
)

// x11Typer handles text input and clipboard via native X11 (XTest + selections).
type x11Typer struct {
	conn *xgb.Conn
	root xproto.Window
	wid  xproto.Window // hidden window for clipboard ownership

	// Keyboard mapping
	minKey  xproto.Keycode
	mapping *xproto.GetKeyboardMappingReply
	perCode int

	// Cached keycodes
	shiftL    xproto.Keycode
	ctrlL     xproto.Keycode
	backspace xproto.Keycode
	vKey      xproto.Keycode

	// Atoms
	clipboard  xproto.Atom
	utf8String xproto.Atom
	targets    xproto.Atom
	xselData   xproto.Atom
}

func newX11Typer() (*x11Typer, error) {
	conn, err := xgb.NewConn()
	if err != nil {
		return nil, err
	}

	if err := xtest.Init(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("XTest: %w", err)
	}

	setup := xproto.Setup(conn)
	screen := setup.DefaultScreen(conn)

	wid, err := xproto.NewWindowId(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}
	xproto.CreateWindow(conn, 0, wid, screen.Root, //nolint:errcheck
		0, 0, 1, 1, 0, xproto.WindowClassInputOnly, 0, 0, nil)

	x := &x11Typer{
		conn: conn,
		root: screen.Root,
		wid:  wid,
	}

	x.minKey = setup.MinKeycode
	x.mapping, err = xproto.GetKeyboardMapping(conn, x.minKey, byte(setup.MaxKeycode-x.minKey+1)).Reply()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("keyboard mapping: %w", err)
	}
	x.perCode = int(x.mapping.KeysymsPerKeycode)

	x.shiftL = x.findKeycode(0xFFE1)    // Shift_L
	x.ctrlL = x.findKeycode(0xFFE3)     // Control_L
	x.backspace = x.findKeycode(0xFF08) // BackSpace
	x.vKey = x.findKeycode(uint32('v'))

	x.clipboard = x.internAtom("CLIPBOARD")
	x.utf8String = x.internAtom("UTF8_STRING")
	x.targets = x.internAtom("TARGETS")
	x.xselData = x.internAtom("XSEL_DATA")

	return x, nil
}

func (x *x11Typer) close() {
	xproto.DestroyWindow(x.conn, x.wid) //nolint:errcheck
	x.conn.Close()
}

func (x *x11Typer) internAtom(name string) xproto.Atom {
	r, err := xproto.InternAtom(x.conn, false, uint16(len(name)), name).Reply()
	if err != nil {
		return 0
	}
	return r.Atom
}

func (x *x11Typer) findKeycode(keysym uint32) xproto.Keycode {
	for i, ks := range x.mapping.Keysyms {
		if uint32(ks) == keysym {
			return x.minKey + xproto.Keycode(i/x.perCode)
		}
	}
	return 0
}

// keysymInfo returns the keycode and whether Shift is needed for a keysym.
func (x *x11Typer) keysymInfo(keysym uint32) (xproto.Keycode, bool, bool) {
	for i, ks := range x.mapping.Keysyms {
		if uint32(ks) == keysym {
			kc := x.minKey + xproto.Keycode(i/x.perCode)
			col := i % x.perCode
			return kc, col == 1, true
		}
	}
	return 0, false, false
}

func (x *x11Typer) pressKey(kc xproto.Keycode) {
	xtest.FakeInput(x.conn, xproto.KeyPress, byte(kc), 0, x.root, 0, 0, 0) //nolint:errcheck
}

func (x *x11Typer) releaseKey(kc xproto.Keycode) {
	xtest.FakeInput(x.conn, xproto.KeyRelease, byte(kc), 0, x.root, 0, 0, 0) //nolint:errcheck
}

func (x *x11Typer) tap(kc xproto.Keycode) {
	x.pressKey(kc)
	x.releaseKey(kc)
}

func (x *x11Typer) tapShifted(kc xproto.Keycode) {
	x.pressKey(x.shiftL)
	x.pressKey(kc)
	x.releaseKey(kc)
	x.releaseKey(x.shiftL)
}

// typeRune sends a single character via XTest. Returns false if unmappable.
func (x *x11Typer) typeRune(r rune) bool {
	if r > 0xFFFF {
		return false
	}
	keysym := uint32(r)

	if kc, shift, ok := x.keysymInfo(keysym); ok {
		if shift {
			x.tapShifted(kc)
		} else {
			x.tap(kc)
		}
		return true
	}

	// Uppercase letter: try lowercase keysym + Shift.
	if unicode.IsUpper(r) {
		lower := uint32(unicode.ToLower(r))
		if kc, _, ok := x.keysymInfo(lower); ok {
			x.tapShifted(kc)
			return true
		}
	}
	return false
}

// typeString types text character by character via XTest FakeInput.
func (x *x11Typer) typeString(text string) {
	for _, r := range text {
		x.typeRune(r)
	}
	x.conn.Sync()
}

// sendBackspaces sends n BackSpace key presses.
func (x *x11Typer) sendBackspaces(n int) {
	for range n {
		x.tap(x.backspace)
	}
	x.conn.Sync()
}

// sendCtrlV sends Ctrl+V via XTest.
func (x *x11Typer) sendCtrlV() {
	x.pressKey(x.ctrlL)
	x.tap(x.vKey)
	x.releaseKey(x.ctrlL)
	x.conn.Sync()
}

// --- Clipboard (X11 selections) ---

// getClipboard reads the current CLIPBOARD selection contents.
func (x *x11Typer) getClipboard() string {
	xproto.ConvertSelection(x.conn, x.wid, x.clipboard, //nolint:errcheck
		x.utf8String, x.xselData, xproto.TimeCurrentTime)

	deadline := time.After(500 * time.Millisecond)
	for {
		select {
		case <-deadline:
			return ""
		default:
		}
		ev, err := x.conn.PollForEvent()
		if err != nil {
			return ""
		}
		if ev == nil {
			time.Sleep(5 * time.Millisecond)
			continue
		}
		if notify, ok := ev.(xproto.SelectionNotifyEvent); ok {
			if notify.Property == 0 {
				return ""
			}
			reply, err := xproto.GetProperty(x.conn, true, x.wid,
				x.xselData, xproto.AtomAny, 0, 1<<20).Reply()
			if err != nil {
				return ""
			}
			return string(reply.Value)
		}
	}
}

// setClipboardAndPaste takes CLIPBOARD ownership, sends Ctrl+V, then serves
// one SelectionRequest so the target app receives the text.
func (x *x11Typer) setClipboardAndPaste(text string) {
	data := []byte(text)

	xproto.SetSelectionOwner(x.conn, x.wid, x.clipboard, xproto.TimeCurrentTime) //nolint:errcheck

	// Send Ctrl+V — the target app will send a SelectionRequest.
	x.sendCtrlV()

	// Serve SelectionRequest events for up to 1 second.
	deadline := time.After(1 * time.Second)
	for {
		select {
		case <-deadline:
			return
		default:
		}
		ev, err := x.conn.PollForEvent()
		if err != nil {
			return
		}
		if ev == nil {
			time.Sleep(5 * time.Millisecond)
			continue
		}
		if req, ok := ev.(xproto.SelectionRequestEvent); ok {
			x.handleSelectionRequest(req, data)
			return // served the paste, done
		}
	}
}

func (x *x11Typer) handleSelectionRequest(req xproto.SelectionRequestEvent, data []byte) {
	prop := req.Property
	if prop == 0 {
		prop = req.Target
	}

	if req.Target == x.targets {
		// Respond with the list of supported targets.
		targets := []byte{
			byte(x.utf8String), byte(x.utf8String >> 8), byte(x.utf8String >> 16), byte(x.utf8String >> 24),
			byte(x.targets), byte(x.targets >> 8), byte(x.targets >> 16), byte(x.targets >> 24),
		}
		xproto.ChangeProperty(x.conn, xproto.PropModeReplace, req.Requestor, //nolint:errcheck
			prop, xproto.AtomAtom, 32, 2, targets)
	} else {
		// Respond with the actual data.
		xproto.ChangeProperty(x.conn, xproto.PropModeReplace, req.Requestor, //nolint:errcheck
			prop, x.utf8String, 8, uint32(len(data)), data)
	}

	// Notify the requestor.
	event := xproto.SelectionNotifyEvent{
		Time:      req.Time,
		Requestor: req.Requestor,
		Selection: req.Selection,
		Target:    req.Target,
		Property:  prop,
	}
	xproto.SendEvent(x.conn, false, req.Requestor, 0, //nolint:errcheck
		string(event.Bytes()))
	x.conn.Sync()
}
