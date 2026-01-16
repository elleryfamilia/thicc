package config

import (
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/ellery/thicc/internal/thicc"
	"github.com/micro-editor/tcell/v2"
)

// InTmux is true if running inside tmux (needs 256-color safe palette)
var InTmux = os.Getenv("TMUX") != ""

// DefStyle is Micro's default style
var DefStyle tcell.Style = tcell.StyleDefault

// Colorscheme is the current colorscheme
var Colorscheme map[string]tcell.Style

// GetColor takes in a syntax group and returns the colorscheme's style for that group
func GetColor(color string) tcell.Style {
	st := DefStyle
	if color == "" {
		return st
	}
	groups := strings.Split(color, ".")
	if len(groups) > 1 {
		curGroup := ""
		for i, g := range groups {
			if i != 0 {
				curGroup += "."
			}
			curGroup += g
			if style, ok := Colorscheme[curGroup]; ok {
				st = style
			}
		}
	} else if style, ok := Colorscheme[color]; ok {
		st = style
	} else {
		st = StringToStyle(color)
	}

	return st
}

// ColorschemeExists checks if a given colorscheme exists
func ColorschemeExists(colorschemeName string) bool {
	return FindRuntimeFile(RTColorscheme, colorschemeName) != nil
}

// DefaultBackgroundColor is the default background color hex value
const DefaultBackgroundColor = "#0b0614"

// ThiccBackground is the default background color for all panels
// Uses 256-color palette in tmux for compatibility, true color otherwise
var ThiccBackground = initThiccBackground()

func initThiccBackground() tcell.Color {
	return getBackgroundColor(DefaultBackgroundColor)
}

// getBackgroundColor converts a hex color to tcell.Color, handling tmux compatibility
func getBackgroundColor(hexColor string) tcell.Color {
	if InTmux {
		// Use 256-color palette for tmux compatibility
		return hexTo256Color(hexColor)
	}
	return tcell.GetColor(hexColor)
}

// ReloadThiccBackground reloads the background color from settings
func ReloadThiccBackground() {
	colorHex := DefaultBackgroundColor
	if thicc.GlobalThiccSettings != nil {
		customColor := thicc.GetBackgroundColor()
		if customColor != "" {
			colorHex = customColor
		}
	}
	ThiccBackground = getBackgroundColor(colorHex)
	// Update DefStyle with new background
	fg, _, _ := DefStyle.Decompose()
	DefStyle = DefStyle.Foreground(fg).Background(ThiccBackground)
}

// InitColorscheme picks and initializes the colorscheme when micro starts
func InitColorscheme() error {
	Colorscheme = make(map[string]tcell.Style)
	DefStyle = tcell.StyleDefault.Background(ThiccBackground)

	log.Printf("THICC: InitColorscheme starting, colorscheme setting = %v, InTmux = %v, ThiccBackground = %v", GlobalSettings["colorscheme"], InTmux, ThiccBackground)

	c, err := LoadDefaultColorscheme()
	if err == nil {
		Colorscheme = c
		log.Printf("THICC: Colorscheme loaded successfully with %d styles", len(c))
	} else {
		log.Printf("THICC: LoadDefaultColorscheme failed: %v", err)
		// The colorscheme setting seems broken (maybe because we have not validated
		// it earlier, see comment in verifySetting()). So reset it to the default
		// colorscheme and try again.
		GlobalSettings["colorscheme"] = DefaultGlobalOnlySettings["colorscheme"]
		if c, err2 := LoadDefaultColorscheme(); err2 == nil {
			Colorscheme = c
			log.Printf("THICC: Fallback colorscheme loaded with %d styles", len(c))
		} else {
			log.Printf("THICC: Fallback colorscheme also failed: %v", err2)
		}
	}

	// Ensure DefStyle uses the Thicc background color
	fg, _, _ := DefStyle.Decompose()
	DefStyle = DefStyle.Foreground(fg).Background(ThiccBackground)

	// Add default selection style if not defined in colorscheme
	// This prevents invisible text when using DefStyle.Reverse(true) fallback
	if _, ok := Colorscheme["selection"]; !ok {
		Colorscheme["selection"] = tcell.StyleDefault.
			Foreground(tcell.ColorWhite).
			Background(tcell.Color24) // Dark blue selection background
	}

	// Add diff styles with background colors for unified diff view
	// These use subtle, dark background tints for added/deleted lines
	var diffAddBg, diffDelBg, diffAddFg, diffDelFg tcell.Color
	if InTmux {
		// Use 256-color palette for tmux
		diffAddBg = tcell.Color236  // Very dark gray
		diffDelBg = tcell.Color236  // Very dark gray
		diffAddFg = tcell.Color114  // Light green
		diffDelFg = tcell.Color210  // Light red/salmon
	} else {
		// Use true colors for subtle tints
		diffAddBg = tcell.GetColor("#0f1a0f") // Very dark green tint
		diffDelBg = tcell.GetColor("#1a0f0f") // Very dark red tint
		diffAddFg = tcell.GetColor("#98c379") // Light green
		diffDelFg = tcell.GetColor("#e06c75") // Light red
	}
	Colorscheme["diff-add"] = tcell.StyleDefault.
		Foreground(diffAddFg).
		Background(diffAddBg)
	Colorscheme["diff-del"] = tcell.StyleDefault.
		Foreground(diffDelFg).
		Background(diffDelBg)
	Colorscheme["diff-header"] = tcell.StyleDefault.
		Foreground(tcell.Color141). // Light purple
		Background(ThiccBackground).
		Bold(true)
	Colorscheme["diff-hunk"] = tcell.StyleDefault.
		Foreground(tcell.Color45). // Cyan
		Background(ThiccBackground).
		Bold(true)

	return err
}

// LoadDefaultColorscheme loads the default colorscheme from $(ConfigDir)/colorschemes
func LoadDefaultColorscheme() (map[string]tcell.Style, error) {
	var parsedColorschemes []string
	return LoadColorscheme(GlobalSettings["colorscheme"].(string), &parsedColorschemes)
}

// LoadColorscheme loads the given colorscheme from a directory
func LoadColorscheme(colorschemeName string, parsedColorschemes *[]string) (map[string]tcell.Style, error) {
	c := make(map[string]tcell.Style)
	file := FindRuntimeFile(RTColorscheme, colorschemeName)
	if file == nil {
		return c, errors.New(colorschemeName + " is not a valid colorscheme")
	}
	if data, err := file.Data(); err != nil {
		return c, errors.New("Error loading colorscheme: " + err.Error())
	} else {
		var err error
		c, err = ParseColorscheme(file.Name(), string(data), parsedColorschemes)
		if err != nil {
			return c, err
		}
	}
	return c, nil
}

// ParseColorscheme parses the text definition for a colorscheme and returns the corresponding object
// Colorschemes are made up of color-link statements linking a color group to a list of colors
// For example, color-link keyword (blue,red) makes all keywords have a blue foreground and
// red background
func ParseColorscheme(name string, text string, parsedColorschemes *[]string) (map[string]tcell.Style, error) {
	var err error
	colorParser := regexp.MustCompile(`color-link\s+(\S*)\s+"(.*)"`)
	includeParser := regexp.MustCompile(`include\s+"(.*)"`)
	lines := strings.Split(text, "\n")
	c := make(map[string]tcell.Style)

	if parsedColorschemes != nil {
		*parsedColorschemes = append(*parsedColorschemes, name)
	}

lineLoop:
	for _, line := range lines {
		if strings.TrimSpace(line) == "" ||
			strings.TrimSpace(line)[0] == '#' {
			// Ignore this line
			continue
		}

		matches := includeParser.FindSubmatch([]byte(line))
		if len(matches) == 2 {
			// support includes only in case parsedColorschemes are given
			if parsedColorschemes != nil {
				include := string(matches[1])
				for _, name := range *parsedColorschemes {
					// check for circular includes...
					if name == include {
						// ...and prevent them
						continue lineLoop
					}
				}
				includeScheme, err := LoadColorscheme(include, parsedColorschemes)
				if err != nil {
					return c, err
				}
				for k, v := range includeScheme {
					c[k] = v
				}
			}
			continue
		}

		matches = colorParser.FindSubmatch([]byte(line))
		if len(matches) == 3 {
			link := string(matches[1])
			colors := string(matches[2])

			style := StringToStyle(colors)
			c[link] = style

			if link == "default" {
				DefStyle = style
			}
		} else {
			err = errors.New("Color-link statement is not valid: " + line)
		}
	}

	return c, err
}

// StringToStyle returns a style from a string
// The strings must be in the format "extra foregroundcolor,backgroundcolor"
// The 'extra' can be bold, reverse, italic or underline
// Note: Background colors from colorschemes are ignored - we always use ThiccBackground
// for visual consistency across the editor
func StringToStyle(str string) tcell.Style {
	var fg string
	spaceSplit := strings.Split(str, " ")
	split := strings.Split(spaceSplit[len(spaceSplit)-1], ",")
	fg = split[0]
	fg = strings.TrimSpace(fg)

	var fgColor tcell.Color
	var ok bool
	if fg == "" || fg == "default" {
		fgColor, _, _ = DefStyle.Decompose()
	} else {
		fgColor, ok = StringToColor(fg)
		if !ok {
			fgColor, _, _ = DefStyle.Decompose()
		}
	}

	// Always use ThiccBackground for consistent editor appearance
	style := DefStyle.Foreground(fgColor).Background(ThiccBackground)
	if strings.Contains(str, "bold") {
		style = style.Bold(true)
	}
	if strings.Contains(str, "italic") {
		style = style.Italic(true)
	}
	if strings.Contains(str, "reverse") {
		style = style.Reverse(true)
	}
	if strings.Contains(str, "underline") {
		style = style.Underline(true)
	}
	return style
}

// StringToColor returns a tcell color from a string representation of a color
// We accept either bright... or light... to mean the brighter version of a color
func StringToColor(str string) (tcell.Color, bool) {
	switch str {
	case "black":
		return tcell.ColorBlack, true
	case "red":
		return tcell.ColorMaroon, true
	case "green":
		return tcell.ColorGreen, true
	case "yellow":
		return tcell.ColorOlive, true
	case "blue":
		return tcell.ColorNavy, true
	case "magenta":
		return tcell.ColorPurple, true
	case "cyan":
		return tcell.ColorTeal, true
	case "white":
		return tcell.ColorSilver, true
	case "brightblack", "lightblack":
		return tcell.ColorGray, true
	case "brightred", "lightred":
		return tcell.ColorRed, true
	case "brightgreen", "lightgreen":
		return tcell.ColorLime, true
	case "brightyellow", "lightyellow":
		return tcell.ColorYellow, true
	case "brightblue", "lightblue":
		return tcell.ColorBlue, true
	case "brightmagenta", "lightmagenta":
		return tcell.ColorFuchsia, true
	case "brightcyan", "lightcyan":
		return tcell.ColorAqua, true
	case "brightwhite", "lightwhite":
		return tcell.ColorWhite, true
	case "default":
		return tcell.ColorDefault, true
	default:
		// Check if this is a 256 color
		if num, err := strconv.Atoi(str); err == nil {
			return GetColor256(num), true
		}
		// Check if this is a truecolor hex value
		if len(str) == 7 && str[0] == '#' {
			if InTmux {
				// Convert true color to closest 256-palette color for tmux compatibility
				return hexTo256Color(str), true
			}
			return tcell.GetColor(str), true
		}
		return tcell.ColorDefault, false
	}
}

// hexTo256Color converts a hex color string to the closest 256-palette color
func hexTo256Color(hex string) tcell.Color {
	// Parse hex color
	var r, g, b int
	fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b)

	// Use the 216-color cube (colors 16-231) for approximation
	// Each channel maps to 0-5 range
	ri := (r * 5) / 255
	gi := (g * 5) / 255
	bi := (b * 5) / 255

	// Calculate 216-color index: 16 + 36*r + 6*g + b
	colorIndex := 16 + 36*ri + 6*gi + bi

	return tcell.PaletteColor(colorIndex)
}

// GetColor256 returns the tcell color for a number between 0 and 255
func GetColor256(color int) tcell.Color {
	if color == 0 {
		return tcell.ColorDefault
	}
	return tcell.PaletteColor(color)
}
