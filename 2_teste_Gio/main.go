/*
testeGio — throwaway sanity check.

Verify that Gio compiles and runs on the school Linux PCs BEFORE
committing the GUI client to it. Run with: `cd testeGio && go mod tidy && go run .`
If the window opens, Gio works. If the build fails, the error names
the missing system library (usually OpenGL / Wayland / X11 dev headers).
Delete this folder once the decision is made.
*/
package main

import (
	"log"
	"os"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

func main() {
	go func() {
		w := new(app.Window)
		w.Option(app.Title("TAP · Gio sanity check"))
		if err := run(w); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func run(w *app.Window) error {
	th := material.NewTheme()
	var btn widget.Clickable
	var ops op.Ops
	label := "If you can read this, Gio works on this machine."

	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			if btn.Clicked(gtx) {
				label = "Button clicked — events work too."
			}
			layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(material.Body1(th, label).Layout),
				layout.Rigid(material.Button(th, &btn, "Click me").Layout),
			)
			e.Frame(gtx.Ops)
		}
	}
}
