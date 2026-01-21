package session

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Frame types for the PTY multiplexing protocol.
// This is a simple binary protocol: [type:1][length:4][payload:N]
const (
	// FrameData carries raw terminal data (PTY output or client input)
	FrameData byte = 1

	// FrameResize signals a terminal size change
	FrameResize byte = 2

	// FrameClose signals the session is ending
	FrameClose byte = 3

	// FrameHello is sent by client on connect with terminal size
	FrameHello byte = 4

	// FrameWelcome is sent by server in response to Hello
	FrameWelcome byte = 5
)

// MaxFrameSize is the maximum payload size (1MB should be plenty)
const MaxFrameSize = 1024 * 1024

// Frame represents a protocol message.
type Frame struct {
	Type    byte
	Payload []byte
}

// WriteFrame writes a frame to the writer.
// Uses a single write for better performance.
func WriteFrame(w io.Writer, frameType byte, payload []byte) error {
	if len(payload) > MaxFrameSize {
		return fmt.Errorf("frame payload too large: %d > %d", len(payload), MaxFrameSize)
	}

	// Combine header and payload into single buffer for one syscall
	// Header: 1 byte type + 4 bytes length = 5 bytes
	buf := make([]byte, 5+len(payload))
	buf[0] = frameType
	binary.BigEndian.PutUint32(buf[1:5], uint32(len(payload)))
	if len(payload) > 0 {
		copy(buf[5:], payload)
	}

	_, err := w.Write(buf)
	return err
}

// ReadFrame reads a frame from the reader.
func ReadFrame(r io.Reader) (*Frame, error) {
	// Read type byte
	typeBuf := make([]byte, 1)
	if _, err := io.ReadFull(r, typeBuf); err != nil {
		return nil, err
	}

	// Read length
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(r, lenBuf); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(lenBuf)

	if length > MaxFrameSize {
		return nil, fmt.Errorf("frame payload too large: %d > %d", length, MaxFrameSize)
	}

	// Read payload
	payload := make([]byte, length)
	if length > 0 {
		if _, err := io.ReadFull(r, payload); err != nil {
			return nil, err
		}
	}

	return &Frame{
		Type:    typeBuf[0],
		Payload: payload,
	}, nil
}

// ResizePayload encodes terminal dimensions for FrameResize.
type ResizePayload struct {
	Rows uint16
	Cols uint16
}

// EncodeResize creates a resize frame payload.
func EncodeResize(rows, cols int) []byte {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint16(buf[0:2], uint16(rows))
	binary.BigEndian.PutUint16(buf[2:4], uint16(cols))
	return buf
}

// DecodeResize parses a resize frame payload.
func DecodeResize(payload []byte) (rows, cols int, err error) {
	if len(payload) < 4 {
		return 0, 0, fmt.Errorf("resize payload too short: %d", len(payload))
	}
	rows = int(binary.BigEndian.Uint16(payload[0:2]))
	cols = int(binary.BigEndian.Uint16(payload[2:4]))
	return rows, cols, nil
}

// HelloPayload is sent by client on connection.
type HelloPayload struct {
	Rows    uint16
	Cols    uint16
	Version string // THICC version for compatibility check
}

// EncodeHello creates a hello frame payload.
func EncodeHello(rows, cols int, version string) []byte {
	versionBytes := []byte(version)
	buf := make([]byte, 4+2+len(versionBytes))
	binary.BigEndian.PutUint16(buf[0:2], uint16(rows))
	binary.BigEndian.PutUint16(buf[2:4], uint16(cols))
	binary.BigEndian.PutUint16(buf[4:6], uint16(len(versionBytes)))
	copy(buf[6:], versionBytes)
	return buf
}

// DecodeHello parses a hello frame payload.
func DecodeHello(payload []byte) (rows, cols int, version string, err error) {
	if len(payload) < 6 {
		return 0, 0, "", fmt.Errorf("hello payload too short: %d", len(payload))
	}
	rows = int(binary.BigEndian.Uint16(payload[0:2]))
	cols = int(binary.BigEndian.Uint16(payload[2:4]))
	versionLen := int(binary.BigEndian.Uint16(payload[4:6]))
	if len(payload) < 6+versionLen {
		return 0, 0, "", fmt.Errorf("hello payload truncated")
	}
	version = string(payload[6 : 6+versionLen])
	return rows, cols, version, nil
}

// WelcomePayload is sent by server in response to Hello.
type WelcomePayload struct {
	Accepted    bool
	SessionName string
	Version     string
	Reason      string // If not accepted, explains why
}

// EncodeWelcome creates a welcome frame payload.
func EncodeWelcome(accepted bool, sessionName, version, reason string) []byte {
	sessionBytes := []byte(sessionName)
	versionBytes := []byte(version)
	reasonBytes := []byte(reason)

	// Format: [accepted:1][sessionLen:2][session:N][versionLen:2][version:N][reasonLen:2][reason:N]
	buf := make([]byte, 1+2+len(sessionBytes)+2+len(versionBytes)+2+len(reasonBytes))
	offset := 0

	// Accepted flag
	if accepted {
		buf[offset] = 1
	} else {
		buf[offset] = 0
	}
	offset++

	// Session name
	binary.BigEndian.PutUint16(buf[offset:], uint16(len(sessionBytes)))
	offset += 2
	copy(buf[offset:], sessionBytes)
	offset += len(sessionBytes)

	// Version
	binary.BigEndian.PutUint16(buf[offset:], uint16(len(versionBytes)))
	offset += 2
	copy(buf[offset:], versionBytes)
	offset += len(versionBytes)

	// Reason
	binary.BigEndian.PutUint16(buf[offset:], uint16(len(reasonBytes)))
	offset += 2
	copy(buf[offset:], reasonBytes)

	return buf
}

// DecodeWelcome parses a welcome frame payload.
func DecodeWelcome(payload []byte) (accepted bool, sessionName, version, reason string, err error) {
	if len(payload) < 1 {
		return false, "", "", "", fmt.Errorf("welcome payload too short")
	}

	offset := 0
	accepted = payload[offset] == 1
	offset++

	// Read session name
	if len(payload) < offset+2 {
		return false, "", "", "", fmt.Errorf("welcome payload truncated at session length")
	}
	sessionLen := int(binary.BigEndian.Uint16(payload[offset:]))
	offset += 2
	if len(payload) < offset+sessionLen {
		return false, "", "", "", fmt.Errorf("welcome payload truncated at session")
	}
	sessionName = string(payload[offset : offset+sessionLen])
	offset += sessionLen

	// Read version
	if len(payload) < offset+2 {
		return false, "", "", "", fmt.Errorf("welcome payload truncated at version length")
	}
	versionLen := int(binary.BigEndian.Uint16(payload[offset:]))
	offset += 2
	if len(payload) < offset+versionLen {
		return false, "", "", "", fmt.Errorf("welcome payload truncated at version")
	}
	version = string(payload[offset : offset+versionLen])
	offset += versionLen

	// Read reason
	if len(payload) < offset+2 {
		return false, "", "", "", fmt.Errorf("welcome payload truncated at reason length")
	}
	reasonLen := int(binary.BigEndian.Uint16(payload[offset:]))
	offset += 2
	if len(payload) < offset+reasonLen {
		return false, "", "", "", fmt.Errorf("welcome payload truncated at reason")
	}
	reason = string(payload[offset : offset+reasonLen])

	return accepted, sessionName, version, reason, nil
}

// CloseReasons provides standard close messages.
var CloseReasons = struct {
	NewClient      string
	ServerShutdown string
	ChildExited    string
	Error          string
}{
	NewClient:      "Another client connected",
	ServerShutdown: "Server is shutting down",
	ChildExited:    "THICC process exited",
	Error:          "An error occurred",
}
