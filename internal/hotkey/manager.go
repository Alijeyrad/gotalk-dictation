package hotkey

import (
	"fmt"
	"log"
	"strings"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
)

// Manager registers and listens for a global X11 hotkey.
type Manager struct {
	conn     *xgb.Conn
	keycode  xproto.Keycode
	modMask  uint16
	stopCh   chan struct{}
}

// New creates a Manager for the given hotkey string (e.g. "Alt-d").
func New(hotkey string) (*Manager, error) {
	conn, err := xgb.NewConn()
	if err != nil {
		return nil, fmt.Errorf("connecting to X11: %w", err)
	}

	keycode, modMask, err := parseHotkey(conn, hotkey)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("parsing hotkey %q: %w", hotkey, err)
	}

	return &Manager{
		conn:    conn,
		keycode: keycode,
		modMask: modMask,
		stopCh:  make(chan struct{}),
	}, nil
}

// Register grabs the hotkey globally and starts the event loop.
// callback is invoked in a new goroutine on each keypress.
func (m *Manager) Register(callback func()) error {
	root := xproto.Setup(m.conn).DefaultScreen(m.conn).Root

	// Grab key with common modifier combinations to handle NumLock/CapsLock state.
	extras := []uint16{0, uint16(xproto.ModMask2), uint16(xproto.ModMaskLock), uint16(xproto.ModMask2) | uint16(xproto.ModMaskLock)}
	for _, extra := range extras {
		mod := m.modMask | extra
		err := xproto.GrabKeyChecked(m.conn, true, root, mod, m.keycode,
			xproto.GrabModeAsync, xproto.GrabModeAsync).Check()
		if err != nil {
			return fmt.Errorf("grabbing key (mod=%d): %w", mod, err)
		}
	}

	go m.eventLoop(callback)
	return nil
}

func (m *Manager) eventLoop(callback func()) {
	for {
		select {
		case <-m.stopCh:
			return
		default:
		}

		ev, err := m.conn.WaitForEvent()
		if err != nil {
			log.Printf("hotkey: X11 event error: %v", err)
			return
		}
		if ev == nil {
			return
		}

		if _, ok := ev.(xproto.KeyPressEvent); ok {
			go callback()
		}
	}
}

// Stop terminates the event loop and closes the X11 connection.
func (m *Manager) Stop() {
	select {
	case <-m.stopCh:
	default:
		close(m.stopCh)
	}
	m.conn.Close()
}

// parseHotkey parses a string like "Alt-d" or "Ctrl+Shift-a" into a keycode and modifier mask.
func parseHotkey(conn *xgb.Conn, hotkey string) (xproto.Keycode, uint16, error) {
	parts := strings.FieldsFunc(hotkey, func(r rune) bool {
		return r == '+' || r == '-'
	})

	var modMask uint16
	var keyName string

	for _, p := range parts {
		switch strings.ToLower(p) {
		case "alt", "mod1":
			modMask |= uint16(xproto.ModMask1)
		case "ctrl", "control":
			modMask |= uint16(xproto.ModMaskControl)
		case "shift":
			modMask |= uint16(xproto.ModMaskShift)
		case "super", "mod4", "win":
			modMask |= uint16(xproto.ModMask4)
		default:
			keyName = p
		}
	}

	if keyName == "" {
		return 0, 0, fmt.Errorf("no key specified")
	}

	keycode, err := findKeycode(conn, keyName)
	if err != nil {
		return 0, 0, err
	}

	return keycode, modMask, nil
}

// findKeycode maps a key name (single char or named key) to an X11 keycode.
func findKeycode(conn *xgb.Conn, keyName string) (xproto.Keycode, error) {
	setup := xproto.Setup(conn)
	min := setup.MinKeycode
	max := setup.MaxKeycode

	km, err := xproto.GetKeyboardMapping(conn, min, byte(max-min+1)).Reply()
	if err != nil {
		return 0, fmt.Errorf("getting keyboard mapping: %w", err)
	}

	var targetKeysym uint32
	if len(keyName) == 1 {
		// ASCII characters map directly to keysyms
		targetKeysym = uint32(keyName[0])
	} else {
		// Named keys
		switch strings.ToLower(keyName) {
		case "space":
			targetKeysym = 0x0020
		case "return", "enter":
			targetKeysym = 0xff0d
		case "escape", "esc":
			targetKeysym = 0xff1b
		case "tab":
			targetKeysym = 0xff09
		case "f1":
			targetKeysym = 0xffbe
		case "f2":
			targetKeysym = 0xffbf
		case "f3":
			targetKeysym = 0xffc0
		case "f4":
			targetKeysym = 0xffc1
		case "f5":
			targetKeysym = 0xffc2
		case "f6":
			targetKeysym = 0xffc3
		case "f7":
			targetKeysym = 0xffc4
		case "f8":
			targetKeysym = 0xffc5
		case "f9":
			targetKeysym = 0xffc6
		case "f10":
			targetKeysym = 0xffc7
		case "f11":
			targetKeysym = 0xffc8
		case "f12":
			targetKeysym = 0xffc9
		default:
			return 0, fmt.Errorf("unknown key name: %q", keyName)
		}
	}

	keysymsPerKeycode := int(km.KeysymsPerKeycode)
	for i, keysym := range km.Keysyms {
		if uint32(keysym) == targetKeysym {
			keycode := min + xproto.Keycode(i/keysymsPerKeycode)
			return keycode, nil
		}
	}

	return 0, fmt.Errorf("key %q not found in keyboard mapping", keyName)
}
