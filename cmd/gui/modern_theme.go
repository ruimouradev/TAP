package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// ModernTheme uses a pastel/bright palette
type ModernTheme struct {
	fyne.Theme
}

func (j *ModernTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.RGBA{R: 250, G: 244, B: 233, A: 255} // warm paper
	case theme.ColorNameButton:
		return color.RGBA{R: 255, G: 225, B: 204, A: 255} // peach
	case theme.ColorNameDisabledButton:
		return color.RGBA{R: 228, G: 216, B: 205, A: 255}
	case theme.ColorNameDisabled:
		return color.RGBA{R: 150, G: 135, B: 124, A: 255}
	case theme.ColorNameHover:
		return color.RGBA{R: 255, G: 214, B: 179, A: 255}
	case theme.ColorNameInputBackground:
		return color.RGBA{R: 255, G: 251, B: 245, A: 255}
	case theme.ColorNamePlaceHolder:
		return color.RGBA{R: 151, G: 132, B: 118, A: 255}
	case theme.ColorNamePrimary:
		return color.RGBA{R: 234, G: 116, B: 95, A: 255} // coral
	case theme.ColorNameForeground:
		return color.RGBA{R: 62, G: 55, B: 48, A: 255} // deep brown-gray
	case theme.ColorNameShadow:
		return color.RGBA{R: 119, G: 104, B: 96, A: 100}
	case theme.ColorNameSelection:
		return color.RGBA{R: 255, G: 236, B: 214, A: 255}
	}
	return j.Theme.Color(name, variant)
}

func (j *ModernTheme) Font(style fyne.TextStyle) fyne.Resource {
	return j.Theme.Font(style)
}
