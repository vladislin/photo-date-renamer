package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	applicationui "photo-date-renamer/internal/ui"
)

func main() {
	application := app.NewWithID("photo-date-renamer")
	window := application.NewWindow("Photo Date Renamer")
	window.Resize(fyne.NewSize(1000, 680))
	window.SetContent(applicationui.New(window).Content())
	window.ShowAndRun()
}
