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

	// Background - Deep purple/black
	ColorBgDark  = tcell.Color16  // #000000 - Pure black
	ColorBgPanel = tcell.Color233 // #121212 - Very dark gray

	// Text colors
	ColorTextBright = tcell.ColorWhite
	ColorTextDim    = tcell.Color245 // #8A8A8A - Medium gray
	ColorTextMuted  = tcell.Color240 // #585858 - Dark gray
)

// Pre-defined styles for dashboard elements
var (
	// Title style - big bold pink
	StyleTitle = tcell.StyleDefault.
			Foreground(ColorMagenta).
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
			Foreground(ColorCyan)

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
			Foreground(ColorMagenta)

	StyleBorderDim = tcell.StyleDefault.
			Foreground(ColorMagentaDark)

	// Version and footer
	StyleVersion = tcell.StyleDefault.
			Foreground(ColorTextMuted)

	StyleFooterHint = tcell.StyleDefault.
			Foreground(ColorTextDim)

	StyleFooterKey = tcell.StyleDefault.
			Foreground(ColorYellow).
			Bold(true)

	// ASCII art - gradient effect
	StyleArtPrimary = tcell.StyleDefault.
			Foreground(ColorMagenta)

	StyleArtSecondary = tcell.StyleDefault.
				Foreground(ColorCyan)

	StyleArtAccent = tcell.StyleDefault.
			Foreground(ColorYellow)

	// Section header
	StyleSectionHeader = tcell.StyleDefault.
				Foreground(ColorCyan).
				Bold(true)

	// Background fill
	StyleBackground = tcell.StyleDefault.
			Background(ColorBgDark)
)
