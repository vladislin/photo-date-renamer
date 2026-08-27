package model

import "time"

type DateSource string

const (
	DateSourceEXIF      DateSource = "EXIF"
	DateSourceQuickTime DateSource = "QUICKTIME"
	DateSourceFilename  DateSource = "FILENAME"
	DateSourceNone      DateSource = "NONE"
)

type Status string

const (
	StatusReady       Status = "READY"
	StatusUnprocessed Status = "UNPROCESSED"
	StatusSkipped     Status = "SKIPPED"
	StatusSuccess     Status = "SUCCESS"
	StatusError       Status = "ERROR"
)

type Mode string

const (
	ModePreview Mode = "preview"
	ModeCopy    Mode = "copy"
	ModeMove    Mode = "move"
)

type Result struct {
	SourcePath      string
	DestinationPath string
	OriginalName    string
	Date            time.Time
	DateSource      DateSource
	Status          Status
	Reason          string
	Err             error
}

type Summary struct {
	Total       int
	Ready       int
	Processed   int
	Unprocessed int
	Skipped     int
	Errors      int
}

func Summarize(results []Result) Summary {
	var summary Summary
	for _, result := range results {
		summary.Total++
		switch result.Status {
		case StatusReady:
			summary.Ready++
		case StatusSuccess:
			summary.Processed++
		case StatusUnprocessed:
			summary.Unprocessed++
		case StatusSkipped:
			summary.Skipped++
		case StatusError:
			summary.Errors++
		}
	}
	return summary
}
