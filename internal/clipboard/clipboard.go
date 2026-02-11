package clipboard

import (
	"errors"
	"sync"

	"github.com/zyedidia/clipper"
)

// pendingClip tracks the last text written to the clipboard from within
// the app. This ensures that reads return what we wrote, not stale content
// from the system clipboard (which may differ when running over ssh/mosh/
// zellij, or when the outer terminal emulator overwrites the clipboard).
// pendingClip persists until overwritten by the next clipboard.Write().
var (
	pendingClip   string
	pendingClipMu sync.Mutex
)

type Method int

const (
	// External relies on external tools for accessing the clipboard
	// These include xclip, xsel, wl-clipboard for linux, pbcopy/pbpaste on Mac,
	// and Syscalls on Windows.
	External Method = iota
	// Terminal uses the terminal to manage the clipboard via OSC 52. Many
	// terminals do not support OSC 52, in which case this method won't work.
	Terminal
	// Internal just manages the clipboard with an internal buffer and doesn't
	// attempt to interface with the system clipboard
	Internal
)

// CurrentMethod is the method used to store clipboard information
var CurrentMethod Method = Internal

// A Register is a buffer used to store text. The system clipboard has the 'clipboard'
// and 'primary' (linux-only) registers, but other registers may be used internal to micro.
type Register int

const (
	// ClipboardReg is the main system clipboard
	ClipboardReg Register = -1
	// PrimaryReg is the system primary clipboard (linux only)
	PrimaryReg = -2
)

var clipboard clipper.Clipboard

// Initialize attempts to initialize the clipboard using the given method
func Initialize(m Method) error {
	var err error
	switch m {
	case External:
		clips := make([]clipper.Clipboard, 0, len(clipper.Clipboards)+1)
		clips = append(clips, &clipper.Custom{
			Name: "micro-clip",
		})
		clips = append(clips, clipper.Clipboards...)
		clipboard, err = clipper.GetClipboard(clips...)
	}
	if err != nil {
		CurrentMethod = Internal
	}
	return err
}

// SetMethod changes the clipboard access method
func SetMethod(m string) Method {
	switch m {
	case "internal":
		CurrentMethod = Internal
	case "external":
		CurrentMethod = External
	case "terminal":
		CurrentMethod = Terminal
	}
	return CurrentMethod
}

// Read reads from a clipboard register
func Read(r Register) (string, error) {
	return read(r, CurrentMethod)
}

// Write writes text to a clipboard register
func Write(text string, r Register) error {
	return write(text, r, CurrentMethod)
}

// PasteText returns the best text to use for a paste operation.
// If we have copied text within the app, return that instead of
// the provided text (which may come from an EventPaste and contain
// stale/wrong content from the outer terminal emulator — especially
// over ssh/mosh where OSC 52 clipboard sync doesn't work).
func PasteText(eventText string) string {
	pendingClipMu.Lock()
	pending := pendingClip
	pendingClipMu.Unlock()

	if pending != "" {
		return pending
	}
	return eventText
}

// ReadMulti reads text from a clipboard register for a certain multi-cursor
func ReadMulti(r Register, num, ncursors int) (string, error) {
	clip, err := Read(r)
	if err != nil {
		return "", err
	}
	if ValidMulti(r, clip, ncursors) {
		return multi.getText(r, num), nil
	}
	return clip, nil
}

// WriteMulti writes text to a clipboard register for a certain multi-cursor
func WriteMulti(text string, r Register, num int, ncursors int) error {
	return writeMulti(text, r, num, ncursors, CurrentMethod)
}

// ValidMulti checks if the internal multi-clipboard is valid and up-to-date
// with the system clipboard
func ValidMulti(r Register, clip string, ncursors int) bool {
	return multi.isValid(r, clip, ncursors)
}

func writeMulti(text string, r Register, num int, ncursors int, m Method) error {
	multi.writeText(text, r, num, ncursors)
	return write(multi.getAllText(r), r, m)
}

func read(r Register, m Method) (string, error) {
	// If we have written to the clipboard from within the app, return
	// our internal copy. This handles:
	// 1. The outer terminal emulator overwriting the system clipboard.
	// 2. Running over ssh/mosh where the remote system clipboard
	//    differs from what the user copied inside the app.
	// pendingClip persists until overwritten by the next Write().
	if r == ClipboardReg || r == PrimaryReg {
		pendingClipMu.Lock()
		pending := pendingClip
		pendingClipMu.Unlock()

		if pending != "" {
			return pending, nil
		}
	}

	switch m {
	case External:
		switch r {
		case ClipboardReg:
			b, e := clipboard.ReadAll(clipper.RegClipboard)
			return string(b), e
		case PrimaryReg:
			b, e := clipboard.ReadAll(clipper.RegPrimary)
			return string(b), e
		default:
			return internal.read(r), nil
		}
	case Internal:
		return internal.read(r), nil
	case Terminal:
		switch r {
		case ClipboardReg:
			// terminal paste works by sending an esc sequence to the
			// terminal to trigger a paste event
			return terminal.read("clipboard")
		case PrimaryReg:
			return terminal.read("primary")
		default:
			return internal.read(r), nil
		}
	}
	return "", errors.New("Invalid clipboard method")
}

func write(text string, r Register, m Method) error {
	// Always mirror to internal so we have a reliable copy that
	// can't be overwritten by the outer terminal emulator.
	internal.write(text, r)
	if r == ClipboardReg || r == PrimaryReg {
		pendingClipMu.Lock()
		pendingClip = text
		pendingClipMu.Unlock()
	}

	switch m {
	case External:
		switch r {
		case ClipboardReg:
			clipboard.WriteAll(clipper.RegClipboard, []byte(text))
			// Also write via OSC 52 so the text reaches the local
			// clipboard when running over ssh/mosh/tmux/zellij.
			// Best-effort: ignore errors if the terminal doesn't
			// support OSC 52.
			terminal.write(text, "c")
			return nil
		case PrimaryReg:
			clipboard.WriteAll(clipper.RegPrimary, []byte(text))
			terminal.write(text, "p")
			return nil
		}
	case Terminal:
		switch r {
		case ClipboardReg:
			return terminal.write(text, "c")
		case PrimaryReg:
			return terminal.write(text, "p")
		}
	}
	return nil
}
