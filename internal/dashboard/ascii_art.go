package dashboard

import "github.com/micro-editor/tcell/v2"

// MarshmallowArt is the simple cute character (using block characters)
var MarshmallowArt = []string{
	`  ▄▄▄▄▄▄▄▄▄  `,
	` █  ◉   ◉  █ `,
	`  ▀▀▀▀▀▀▀▀▀  `,
}

// MarshmallowArtWidth is the width of the ASCII art
var MarshmallowArtWidth = 13

// MarshmallowArtHeight is the height of the ASCII art
var MarshmallowArtHeight = len(MarshmallowArt)

// ArtColorRegion defines a colored region within the ASCII art
type ArtColorRegion struct {
	Line   int
	StartX int
	EndX   int
	Style  tcell.Style
}

// MarshmallowColors defines which parts of the art get special colors
var MarshmallowColors = []ArtColorRegion{
	// Eyes - bright cyan
	{Line: 1, StartX: 4, EndX: 5, Style: StyleArtSecondary},
	{Line: 1, StartX: 9, EndX: 10, Style: StyleArtSecondary},
}

// ThiccLogo - THICC logo in DOS Rebel font
var ThiccLogo = []string{
	`███████████ █████   █████ █████   █████████    █████████ `,
	`░█░░░███░░░█░░███   ░░███ ░░███   ███░░░░░███  ███░░░░░███`,
	`░   ░███  ░  ░███    ░███  ░███  ███     ░░░  ███     ░░░ `,
	`    ░███     ░███████████  ░███ ░███         ░███         `,
	`    ░███     ░███░░░░░███  ░███ ░███         ░███         `,
	`    ░███     ░███    ░███  ░███ ░░███     ███░░███     ███`,
	`    █████    █████   █████ █████ ░░█████████  ░░█████████ `,
	`   ░░░░░    ░░░░░   ░░░░░ ░░░░░   ░░░░░░░░░    ░░░░░░░░░  `,
}

// ThiccLogoWidth is the width of the logo
var ThiccLogoWidth = 58

// ThiccLogoHeight is the height of the logo
var ThiccLogoHeight = len(ThiccLogo)

// ThiccTagline appears under the logo
var ThiccTagline = "a terminal editor that sparks joy"

// GetLogoColorForChar returns the color style based on the character type
// Solid blocks (█) get one color, shaded blocks (▒, ░) get another for depth
// All styles have explicit fg AND bg to prevent color changes in light mode
func GetLogoColorForChar(ch rune) tcell.Style {
	switch ch {
	case '█':
		// Solid parts - bright magenta/pink
		return tcell.StyleDefault.Foreground(ColorMagenta).Background(ColorBgDark).Bold(true)
	case '▒', '░':
		// Shaded/outline parts - cyan for contrast
		return tcell.StyleDefault.Foreground(ColorCyan).Background(ColorBgDark).Bold(true)
	default:
		// Everything else (spaces, etc)
		return tcell.StyleDefault.Foreground(ColorMagenta).Background(ColorBgDark).Bold(true)
	}
}
