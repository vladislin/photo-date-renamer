package ui

import (
	"context"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"photo-date-renamer/internal/model"
	"photo-date-renamer/internal/processor"
)

type App struct {
	window fyne.Window

	path       string
	results    []model.Result
	cancel     context.CancelFunc
	pathEntry  *widget.Entry
	mode       *widget.RadioGroup
	output     *widget.RichText
	progress   *widget.ProgressBar
	status     *widget.Label
	chooseBtn  *widget.Button
	scanButton *widget.Button
	runButton  *widget.Button
	cancelBtn  *widget.Button
}

func New(window fyne.Window) *App {
	application := &App{window: window}
	application.build()
	return application
}

func (application *App) Content() fyne.CanvasObject {
	top := container.NewBorder(nil, nil, application.chooseBtn, application.scanButton, application.pathEntry)
	actions := container.NewHBox(
		widget.NewLabel("Mode:"),
		application.mode,
		application.runButton,
		application.cancelBtn,
	)
	footer := container.NewVBox(application.progress, application.status, actions)
	return container.NewBorder(top, footer, nil, nil, application.output)
}

func (application *App) build() {
	application.pathEntry = widget.NewEntry()
	application.pathEntry.Disable()
	application.pathEntry.SetPlaceHolder("Choose a folder with photos and videos")
	application.chooseBtn = widget.NewButton("Choose folder…", application.chooseFolder)
	application.mode = widget.NewRadioGroup([]string{"Preview", "Copy", "Move"}, func(string) {
		if application.runButton != nil {
			application.refreshRunButton()
		}
	})
	application.mode.Horizontal = true
	application.mode.SetSelected("Preview")
	application.output = widget.NewRichText()
	application.output.Wrapping = fyne.TextWrapWord
	application.output.Scroll = fyne.ScrollVerticalOnly
	application.progress = widget.NewProgressBar()
	application.status = widget.NewLabel("Choose a folder to begin.")
	application.scanButton = widget.NewButton("Scan", application.scan)
	application.runButton = widget.NewButton("Execute", application.execute)
	application.runButton.Disable()
	application.cancelBtn = widget.NewButton("Cancel", application.cancelWork)
	application.cancelBtn.Disable()
}

func (application *App) chooseFolder() {
	picker := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil {
			dialog.ShowError(err, application.window)
			return
		}
		if uri == nil {
			return
		}
		application.path = uri.Path()
		application.pathEntry.SetText(application.path)
		application.results = nil
		application.clearOutput()
		application.status.SetText("Folder selected. Click Scan to preview changes.")
		application.refreshRunButton()
	}, application.window)
	picker.Show()
}

func (application *App) scan() {
	if application.path == "" {
		dialog.ShowInformation("Folder required", "Choose a folder first.", application.window)
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	application.cancel = cancel
	application.setBusy(true, "Scanning files…")
	application.clearOutput()

	go func() {
		results, err := processor.Plan(ctx, application.path)
		fyne.Do(func() {
			application.cancel = nil
			application.setBusy(false, "")
			if err != nil {
				application.status.SetText("Scan failed.")
				if err != context.Canceled {
					dialog.ShowError(err, application.window)
				}
				return
			}
			application.results = results
			application.showResults(results)
			application.status.SetText(formatSummary("Preview ready", model.Summarize(results)))
			application.refreshRunButton()
		})
	}()
}

func (application *App) execute() {
	mode := application.selectedMode()
	if mode == model.ModePreview || len(application.results) == 0 {
		return
	}
	action := "Copy"
	if mode == model.ModeMove {
		action = "Move"
	}
	message := fmt.Sprintf("%s %d planned media files? Existing files will not be overwritten.", action, actionableCount(application.results))
	dialog.ShowConfirm("Confirm operation", message, func(confirmed bool) {
		if !confirmed {
			return
		}
		ctx, cancel := context.WithCancel(context.Background())
		application.cancel = cancel
		application.setBusy(true, "Processing files…")
		go func() {
			results, err := processor.Execute(ctx, application.results, mode, func(done, total int, result model.Result) {
				fyne.Do(func() {
					if total > 0 {
						application.progress.SetValue(float64(done) / float64(total))
					}
					application.status.SetText(fmt.Sprintf("Processed %d of %d: %s", done, total, result.OriginalName))
				})
			})
			fyne.Do(func() {
				application.cancel = nil
				application.setBusy(false, "")
				application.results = results
				application.showResults(results)
				if err != nil {
					application.status.SetText("Operation stopped with an error.")
					if err != context.Canceled {
						dialog.ShowError(err, application.window)
					}
					return
				}
				application.status.SetText(formatSummary("Done", model.Summarize(results)))
				application.runButton.Disable()
			})
		}()
	}, application.window)
}

func (application *App) cancelWork() {
	if application.cancel != nil {
		application.cancel()
	}
}

func (application *App) setBusy(busy bool, status string) {
	if busy {
		application.chooseBtn.Disable()
		application.scanButton.Disable()
		application.runButton.Disable()
		application.cancelBtn.Enable()
		application.progress.SetValue(0)
	} else {
		application.chooseBtn.Enable()
		application.scanButton.Enable()
		application.cancelBtn.Disable()
		application.refreshRunButton()
	}
	if status != "" {
		application.status.SetText(status)
	}
}

func (application *App) refreshRunButton() {
	if application.selectedMode() != model.ModePreview && actionableCount(application.results) > 0 && application.cancel == nil {
		application.runButton.Enable()
	} else {
		application.runButton.Disable()
	}
}

func (application *App) selectedMode() model.Mode {
	switch application.mode.Selected {
	case "Copy":
		return model.ModeCopy
	case "Move":
		return model.ModeMove
	default:
		return model.ModePreview
	}
}

func actionableCount(results []model.Result) int {
	count := 0
	for _, result := range results {
		if result.Status == model.StatusReady || result.Status == model.StatusUnprocessed {
			count++
		}
	}
	return count
}

func (application *App) clearOutput() {
	application.output.Segments = nil
	application.output.Refresh()
}

func (application *App) showResults(results []model.Result) {
	application.output.Segments = resultSegments(results)
	application.output.Refresh()
}

func resultSegments(results []model.Result) []widget.RichTextSegment {
	if len(results) == 0 {
		return []widget.RichTextSegment{bodySegment("No files found.")}
	}
	segments := make([]widget.RichTextSegment, 0, len(results)*2)
	for _, result := range results {
		segments = append(segments, statusSegment(result.Status))
		switch result.Status {
		case model.StatusReady, model.StatusSuccess:
			segments = append(segments, bodySegment(fmt.Sprintf(" | %-9s | %s\n    → %s\n", result.DateSource, result.SourcePath, result.DestinationPath)))
		case model.StatusUnprocessed:
			segments = append(segments, bodySegment(fmt.Sprintf(" | %s\n    → %s\n", result.SourcePath, result.DestinationPath)))
		case model.StatusError:
			segments = append(segments, bodySegment(fmt.Sprintf(" | %s | %v\n", result.SourcePath, result.Err)))
		case model.StatusSkipped:
			segments = append(segments, bodySegment(fmt.Sprintf(" | %s\n", result.SourcePath)))
		}
	}
	return segments
}

func statusSegment(status model.Status) *widget.TextSegment {
	colorName := theme.ColorNameForeground
	switch status {
	case model.StatusReady, model.StatusSuccess:
		colorName = theme.ColorNameSuccess
	case model.StatusUnprocessed:
		colorName = theme.ColorNameWarning
	case model.StatusSkipped:
		colorName = theme.ColorNamePlaceHolder
	case model.StatusError:
		colorName = theme.ColorNameError
	}
	return &widget.TextSegment{
		Style: widget.RichTextStyle{
			ColorName: colorName,
			Inline:    true,
			SizeName:  theme.SizeNameText,
			TextStyle: fyne.TextStyle{Bold: true, Monospace: true},
		},
		Text: string(status),
	}
}

func bodySegment(text string) *widget.TextSegment {
	return &widget.TextSegment{
		Style: widget.RichTextStyle{
			ColorName: theme.ColorNameForeground,
			Inline:    true,
			SizeName:  theme.SizeNameText,
			TextStyle: fyne.TextStyle{Monospace: true},
		},
		Text: text,
	}
}

func formatSummary(prefix string, summary model.Summary) string {
	return fmt.Sprintf("%s — total: %d, ready: %d, processed: %d, unprocessed: %d, skipped: %d, errors: %d",
		prefix, summary.Total, summary.Ready, summary.Processed, summary.Unprocessed, summary.Skipped, summary.Errors)
}
