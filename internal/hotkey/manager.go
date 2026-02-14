package hotkey

import (
	"fmt"
	"strings"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
)

type Manager struct {
	conn    *xgb.Conn
	keycode xproto.Keycode
	modMask uint16
	stopCh  chan struct{}
}

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

func (m *Manager) grab() error {
	root := xproto.Setup(m.conn).DefaultScreen(m.conn).Root

	// Grab with NumLock/CapsLock modifier combinations so the hotkey works
	// regardless of lock key state.
	extras := []uint16{0, uint16(xproto.ModMask2), uint16(xproto.ModMaskLock), uint16(xproto.ModMask2) | uint16(xproto.ModMaskLock)}
	for _, extra := range extras {
		mod := m.modMask | extra
		if err := xproto.GrabKeyChecked(m.conn, true, root, mod, m.keycode,
			xproto.GrabModeAsync, xproto.GrabModeAsync).Check(); err != nil {
			return fmt.Errorf("grabbing key (mod=%d): %w", mod, err)
		}
	}
	return nil
}

func (m *Manager) Register(onPress func()) error {
	if err := m.grab(); err != nil {
		return err
	}
	go m.eventLoop(onPress, nil)
	return nil
}

// RegisterPushToTalk registers both press and release callbacks for
// push-to-talk mode. onPress fires on key down, onRelease on key up.
func (m *Manager) RegisterPushToTalk(onPress, onRelease func()) error {
	if err := m.grab(); err != nil {
		return err
	}
	xproto.ChangeWindowAttributes(m.conn, //nolint:errcheck
		xproto.Setup(m.conn).DefaultScreen(m.conn).Root,
		xproto.CwEventMask,
		[]uint32{uint32(xproto.EventMaskKeyPress | xproto.EventMaskKeyRelease)})
	go m.eventLoop(onPress, onRelease)
	return nil
}

func (m *Manager) eventLoop(onPress, onRelease func()) {
	for {
		select {
		case <-m.stopCh:
			return
		default:
		}

		ev, err := m.conn.WaitForEvent()
		if err != nil {
			return
		}
		if ev == nil {
			return
		}

		switch ev.(type) {
		case xproto.KeyPressEvent:
			if onPress != nil {
				go onPress()
			}
		case xproto.KeyReleaseEvent:
			if onRelease != nil {
				go onRelease()
			}
		}
	}
}

func (m *Manager) Stop() {
	select {
	case <-m.stopCh:
	default:
		close(m.stopCh)
	}
	m.conn.Close()
}

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
		targetKeysym = uint32(keyName[0])
	} else {
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
			return min + xproto.Keycode(i/keysymsPerKeycode), nil
		}
	}

	return 0, fmt.Errorf("key %q not found in keyboard mapping", keyName)
}
