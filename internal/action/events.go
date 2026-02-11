package action

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/micro-editor/tcell/v2"
)

type Event interface {
	Name() string
}

// RawEvent is simply an escape code
// We allow users to directly bind escape codes
// to get around some of a limitations of terminals
type RawEvent struct {
	esc string
}

func (r RawEvent) Name() string {
	return r.esc
}

// KeyEvent is a key event containing a key code,
// some possible modifiers (alt, ctrl, etc...) and
// a rune if it was simply a character press
// Note: to be compatible with tcell events,
// for ctrl keys r=code
type KeyEvent struct {
	code tcell.Key
	mod  tcell.ModMask
	r    rune
	any  bool
}

func metaToAlt(mod tcell.ModMask) tcell.ModMask {
	if mod&tcell.ModMeta != 0 {
		mod &= ^tcell.ModMeta
		mod |= tcell.ModAlt
	}
	return mod
}

func metaToCtrl(mod tcell.ModMask) tcell.ModMask {
	if mod&tcell.ModMeta != 0 {
		mod &= ^tcell.ModMeta
		mod |= tcell.ModCtrl
	}
	return mod
}

func keyEvent(e *tcell.EventKey) KeyEvent {
	// When ModMeta is set, the user pressed Cmd (macOS) or Super (Linux).
	// Terminals rarely send ModMeta on native Linux, so receiving it
	// almost always means the user is on macOS — either locally or
	// connecting remotely via ssh/mosh. Map Meta to Ctrl so that
	// Cmd+C/V/X/Z/A etc. trigger the expected Ctrl-based bindings
	// (Copy, Paste, Cut, Undo, SelectAll).
	if e.Modifiers()&tcell.ModMeta != 0 {
		if e.Key() == tcell.KeyRune {
			r := e.Rune()
			if unicode.IsLetter(r) {
				// Cmd+letter → Ctrl+letter with proper KeyCtrl* code
				// so it matches bindings like "Ctrl-c": "Copy|CopyLine"
				lower := unicode.ToLower(r)
				ctrlKey := tcell.KeyCtrlA + tcell.Key(lower-'a')
				return KeyEvent{
					code: ctrlKey,
					mod:  metaToCtrl(e.Modifiers()),
					r:    rune(ctrlKey),
				}
			}
			// Cmd+symbol (e.g., Cmd+[, Cmd+]) → Alt+symbol
			// preserves bindings like "Alt-[": "DiffPrevious"
			return KeyEvent{
				code: e.Key(),
				mod:  metaToAlt(e.Modifiers()),
				r:    r,
			}
		}
		// Cmd+special key (arrows, Home, etc.) → Ctrl+special
		return KeyEvent{
			code: e.Key(),
			mod:  metaToCtrl(e.Modifiers()),
		}
	}

	// No ModMeta: keep existing Meta→Alt behavior for other cases
	ke := KeyEvent{
		code: e.Key(),
		mod:  metaToAlt(e.Modifiers()),
	}
	if e.Key() == tcell.KeyRune {
		ke.r = e.Rune()
	}
	return ke
}

func (k KeyEvent) Name() string {
	if k.any {
		return "<any>"
	}
	s := ""
	m := []string{}
	if k.mod&tcell.ModShift != 0 {
		m = append(m, "Shift")
	}
	if k.mod&tcell.ModAlt != 0 {
		m = append(m, "Alt")
	}
	if k.mod&tcell.ModMeta != 0 {
		m = append(m, "Meta")
	}
	if k.mod&tcell.ModCtrl != 0 {
		m = append(m, "Ctrl")
	}

	ok := false
	if s, ok = tcell.KeyNames[k.code]; !ok {
		if k.code == tcell.KeyRune {
			s = string(k.r)
		} else {
			s = fmt.Sprintf("Key[%d]", k.code)
		}
	}
	if len(m) != 0 {
		if k.mod&tcell.ModCtrl != 0 && strings.HasPrefix(s, "Ctrl-") {
			s = s[5:]
			if len(s) == 1 {
				s = strings.ToLower(s)
			}
		}
		return fmt.Sprintf("%s-%s", strings.Join(m, "-"), s)
	}
	return s
}

// A KeySequence defines a list of consecutive
// events. All events in the sequence must be KeyEvents
// or MouseEvents.
type KeySequenceEvent struct {
	keys []Event
}

func (k KeySequenceEvent) Name() string {
	buf := bytes.Buffer{}
	for _, e := range k.keys {
		buf.WriteByte('<')
		buf.WriteString(e.Name())
		buf.WriteByte('>')
	}
	return buf.String()
}

type MouseState int

const (
	MousePress = iota
	MouseDrag
	MouseRelease
)

// MouseEvent is a mouse event with a mouse button and
// any possible key modifiers
type MouseEvent struct {
	btn   tcell.ButtonMask
	mod   tcell.ModMask
	state MouseState
}

func (m MouseEvent) Name() string {
	mod := ""
	if m.mod&tcell.ModShift != 0 {
		mod = "Shift-"
	}
	if m.mod&tcell.ModAlt != 0 {
		mod = "Alt-"
	}
	if m.mod&tcell.ModMeta != 0 {
		mod = "Meta-"
	}
	if m.mod&tcell.ModCtrl != 0 {
		mod = "Ctrl-"
	}

	state := ""
	switch m.state {
	case MouseDrag:
		state = "Drag"
	case MouseRelease:
		state = "Release"
	}

	for k, v := range mouseEvents {
		if v == m.btn {
			return fmt.Sprintf("%s%s%s", mod, k, state)
		}
	}
	return ""
}

// ConstructEvent takes a tcell event and returns a micro
// event. Note that tcell events can't express certain
// micro events such as key sequences. This function is
// mostly used for debugging/raw panes or constructing
// intermediate micro events while parsing a sequence.
func ConstructEvent(event tcell.Event) (Event, error) {
	switch e := event.(type) {
	case *tcell.EventKey:
		return keyEvent(e), nil
	case *tcell.EventRaw:
		return RawEvent{
			esc: e.EscSeq(),
		}, nil
	case *tcell.EventMouse:
		mod := e.Modifiers()
		if mod&tcell.ModMeta != 0 {
			mod = metaToCtrl(mod)
		} else {
			mod = metaToAlt(mod)
		}
		return MouseEvent{
			btn: e.Buttons(),
			mod: mod,
		}, nil
	}
	return nil, errors.New("No micro event equivalent")
}

// A Handler will take a tcell event and execute it
// appropriately
type Handler interface {
	HandleEvent(tcell.Event)
	HandleCommand(string)
}
