package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// RetroTheme implements a "Terminal" look with high-contrast colors
type RetroTheme struct {
	fyne.Theme
}

func (r *RetroTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.RGBA{R: 0, G: 0, B: 0, A: 255} // Pure Black
	case theme.ColorNameButton:
		return color.RGBA{R: 0, G: 45, B: 10, A: 255} // phosphor panel
	case theme.ColorNameDisabledButton:
		return color.RGBA{R: 0, G: 35, B: 5, A: 255}
	case theme.ColorNameDisabled:
		return color.RGBA{R: 0, G: 110, B: 0, A: 255}
	case theme.ColorNameHover:
		return color.RGBA{R: 0, G: 70, B: 0, A: 255}
	case theme.ColorNameInputBackground:
		return color.RGBA{R: 0, G: 18, B: 0, A: 255}
	case theme.ColorNamePlaceHolder:
		return color.RGBA{R: 0, G: 155, B: 0, A: 255}
	case theme.ColorNamePrimary:
		return color.RGBA{R: 0, G: 255, B: 0, A: 255} // Neon Green
	case theme.ColorNameForeground:
		return color.RGBA{R: 0, G: 235, B: 0, A: 255} // Pale Green
	case theme.ColorNameShadow:
		return color.RGBA{R: 0, G: 40, B: 0, A: 160}
	case theme.ColorNameSelection:
		return color.RGBA{R: 0, G: 85, B: 0, A: 255}
	}
	return r.Theme.Color(name, variant)
}

// Font returns the default font, but using a custom theme ensures 
// that all components react to the palette changes above.
func (r *RetroTheme) Font(style fyne.TextStyle) fyne.Resource {
	return r.Theme.Font(style)
}
