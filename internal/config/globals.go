package config

import (
	"github.com/ellery/thicc/internal/thicc"
)

const (
	// DefaultDoubleClickThreshold is how many milliseconds to wait before a second click is not a double click
	DefaultDoubleClickThreshold = 400
)

// DoubleClickThreshold is the current double-click threshold in milliseconds
var DoubleClickThreshold = DefaultDoubleClickThreshold

var Bindings map[string]map[string]string

func init() {
	Bindings = map[string]map[string]string{
		"command":  make(map[string]string),
		"buffer":   make(map[string]string),
		"terminal": make(map[string]string),
	}
}

// InitDoubleClickThreshold loads the double-click threshold from settings
func InitDoubleClickThreshold() {
	if thicc.GlobalThiccSettings != nil {
		threshold := thicc.GetDoubleClickThreshold()
		if threshold > 0 {
			DoubleClickThreshold = threshold
		}
	}
}
