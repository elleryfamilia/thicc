package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// ThiccLogo - same as internal/dashboard/ascii_art.go
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

// MarshmallowArt - the cute mascot
var MarshmallowArt = []string{
	`  ▄▄▄▄▄▄▄▄▄  `,
	` █  ◉   ◉  █ `,
	`  ▀▀▀▀▀▀▀▀▀  `,
}

// Colors matching the dashboard
var (
	ColorMagenta = color.RGBA{255, 95, 175, 255}  // Hot pink (Color205)
	ColorCyan    = color.RGBA{0, 255, 255, 255}   // Bright cyan
	ColorBg      = color.RGBA{0, 0, 0, 0}         // Transparent background
)

func main() {
	fontPath := flag.String("font", "", "Path to TTF/OTF font file (required)")
	output := flag.String("o", "thicc-logo.png", "Output PNG file")
	fontSize := flag.Float64("size", 24, "Font size in points")
	includeMascot := flag.Bool("mascot", true, "Include the marshmallow mascot")
	padding := flag.Int("padding", 20, "Padding around the logo")
	flag.Parse()

	if *fontPath == "" {
		fmt.Fprintln(os.Stderr, "Error: -font flag is required")
		fmt.Fprintln(os.Stderr, "Usage: logo-gen -font /path/to/font.ttf [-o output.png] [-size 24]")
		fmt.Fprintln(os.Stderr, "\nRecommended: Use a Nerd Font like JetBrainsMono")
		fmt.Fprintln(os.Stderr, "  macOS: ~/Library/Fonts/JetBrainsMonoNerdFont-Regular.ttf")
		fmt.Fprintln(os.Stderr, "  Linux: ~/.local/share/fonts/JetBrainsMonoNerdFont-Regular.ttf")
		os.Exit(1)
	}

	// Load font
	fontData, err := os.ReadFile(*fontPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading font: %v\n", err)
		os.Exit(1)
	}

	f, err := opentype.Parse(fontData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing font: %v\n", err)
		os.Exit(1)
	}

	face, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    *fontSize,
		DPI:     144, // Retina
		Hinting: font.HintingFull,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating font face: %v\n", err)
		os.Exit(1)
	}
	defer face.Close()

	// Calculate dimensions
	metrics := face.Metrics()
	lineHeight := metrics.Height.Ceil()
	charWidth := font.MeasureString(face, "█").Ceil()

	// Determine content to render
	var lines []string
	if *includeMascot {
		lines = append(lines, MarshmallowArt...)
		lines = append(lines, "") // spacing
	}
	lines = append(lines, ThiccLogo...)

	// Find max width
	maxWidth := 0
	for _, line := range lines {
		w := font.MeasureString(face, line).Ceil()
		if w > maxWidth {
			maxWidth = w
		}
	}

	imgWidth := maxWidth + (*padding * 2)
	imgHeight := (lineHeight * len(lines)) + (*padding * 2)

	// Create image with transparent background
	img := image.NewRGBA(image.Rect(0, 0, imgWidth, imgHeight))

	// Draw each line
	y := *padding + metrics.Ascent.Ceil()
	for lineIdx, line := range lines {
		// Center each line
		lineWidth := font.MeasureString(face, line).Ceil()
		x := *padding + (maxWidth-lineWidth)/2

		// Determine if this is mascot or logo
		isMascot := *includeMascot && lineIdx < len(MarshmallowArt)

		// Draw character by character for color control
		for _, ch := range line {
			col := getCharColor(ch, isMascot, lineIdx)
			drawChar(img, face, x, y, ch, col)
			x += charWidth
		}
		y += lineHeight
	}

	// Save PNG
	outFile, err := os.Create(*output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output file: %v\n", err)
		os.Exit(1)
	}
	defer outFile.Close()

	if err := png.Encode(outFile, img); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding PNG: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Generated %s (%dx%d)\n", *output, imgWidth, imgHeight)
}

func getCharColor(ch rune, isMascot bool, lineIdx int) color.RGBA {
	if isMascot {
		// Eyes are cyan
		if ch == '◉' {
			return ColorCyan
		}
		return ColorMagenta
	}

	// Logo coloring
	switch ch {
	case '█':
		return ColorMagenta
	case '▒', '░':
		return ColorCyan
	case ' ':
		return color.RGBA{0, 0, 0, 0} // Transparent
	default:
		return ColorMagenta
	}
}

func drawChar(img *image.RGBA, face font.Face, x, y int, ch rune, col color.RGBA) {
	if ch == ' ' {
		return
	}

	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: face,
		Dot:  fixed.Point26_6{X: fixed.I(x), Y: fixed.I(y)},
	}
	d.DrawString(string(ch))
}
