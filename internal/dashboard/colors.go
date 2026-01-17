package dashboard

import "github.com/micro-editor/tcell/v2"

// Spider-Verse inspired color palette (256-color safe)
var (
	// Primary accent - Hot pink/Magenta (Miles Morales vibes)
	ColorMagenta     = tcell.Color205 // #FF5FAF - Hot pink
	ColorMagentaDark = tcell.Color162 // #D75F87 - Darker pink

	// Secondary accent - Electric cyan (web/tech feel)
	ColorCyan     = tcell.Color51 // #00FFFF - Bright cyan
	ColorCyanDark = tcell.Color44 // #00D7D7 - Darker cyan

	// Tertiary - Bright yellow (energy/action)
	ColorYellow    = tcell.Color226 // #FFFF00 - Bright yellow
	ColorYellowDim = tcell.Color220 // #FFD700 - Gold

	// Violet - Spider-Verse shadow/accent
	ColorViolet = tcell.NewRGBColor(160, 60, 210) // #A03CD2 - Spider-Verse violet

	// Background - Deep purple/black
	ColorBgDark  = tcell.Color16  // #000000 - Pure black
	ColorBgPanel = tcell.Color233 // #121212 - Very dark gray

	// Text colors
	ColorTextBright = tcell.ColorWhite
	ColorTextDim    = tcell.Color245 // #8A8A8A - Medium gray
	ColorTextMuted  = tcell.Color240 // #585858 - Dark gray
)

// Pre-defined styles for dashboard elements
// NOTE: All styles MUST set both Foreground AND Background explicitly
// to prevent color changes in light mode terminals
var (
	// Title style - big bold pink
	StyleTitle = tcell.StyleDefault.
			Foreground(ColorMagenta).
			Background(ColorBgDark).
			Bold(true)

	// Menu item styles
	StyleMenuItem = tcell.StyleDefault.
			Foreground(ColorTextBright).
			Background(ColorBgDark)

	StyleMenuSelected = tcell.StyleDefault.
				Foreground(ColorBgDark).
				Background(ColorYellow).
				Bold(true)

	// Shortcut hints - cyan accent
	StyleShortcut = tcell.StyleDefault.
			Foreground(ColorCyan).
			Background(ColorBgDark)

	// Recent projects list
	StyleRecentItem = tcell.StyleDefault.
			Foreground(ColorTextDim).
			Background(ColorBgDark)

	StyleRecentSelected = tcell.StyleDefault.
				Foreground(ColorBgDark).
				Background(ColorYellow).
				Bold(true)

	StyleRecentFolder = tcell.StyleDefault.
				Foreground(tcell.Color33). // Bright blue for folders
				Background(ColorBgDark)

	// Border style
	StyleBorder = tcell.StyleDefault.
			Foreground(ColorMagenta).
			Background(ColorBgDark)

	StyleBorderDim = tcell.StyleDefault.
			Foreground(ColorViolet).
			Background(ColorBgDark)

	// Version and footer
	StyleVersion = tcell.StyleDefault.
			Foreground(ColorViolet).
			Background(ColorBgDark)

	StyleFooterHint = tcell.StyleDefault.
			Foreground(ColorTextDim).
			Background(ColorBgDark)

	StyleFooterKey = tcell.StyleDefault.
			Foreground(ColorYellow).
			Background(ColorBgDark).
			Bold(true)

	// ASCII art - gradient effect
	StyleArtPrimary = tcell.StyleDefault.
			Foreground(ColorMagenta).
			Background(ColorBgDark)

	StyleArtSecondary = tcell.StyleDefault.
				Foreground(ColorCyan).
				Background(ColorBgDark)

	StyleArtAccent = tcell.StyleDefault.
			Foreground(ColorYellow).
			Background(ColorBgDark)

	// Section header
	StyleSectionHeader = tcell.StyleDefault.
				Foreground(ColorCyan).
				Background(ColorBgDark).
				Bold(true)

	// Background fill
	StyleBackground = tcell.StyleDefault.
			Background(ColorBgDark)
)
