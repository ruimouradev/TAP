/*
testeFyne — throwaway sanity check.

Verify that Fyne compiles and runs on the school Linux PCs BEFORE
committing the GUI client to it. Run with: `cd testeFyne && go mod tidy && go run .`
If the window opens, Fyne works. If the build fails, the error names
the missing system library (usually X11 / OpenGL dev headers on Linux).
Delete this folder once the decision is made.
*/
package main

import (
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func main() {
	a := app.New()
	w := a.NewWindow("TAP · Fyne sanity check")

	label := widget.NewLabel("If you can read this, Fyne works on this machine.")
	button := widget.NewButton("Click me", func() {
		label.SetText("Button clicked — events work too.")
	})

	w.SetContent(container.NewVBox(label, button))
	w.ShowAndRun()
}
