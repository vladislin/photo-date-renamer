package extractor

import (
	"errors"
	"regexp"
	"time"
)

var ErrDateNotFound = errors.New("date not found")

type filenamePattern struct {
	re     *regexp.Regexp
	layout string
	build  func([]string) string
}

var filenamePatterns = []filenamePattern{
	{
		re:     regexp.MustCompile(`(20\d{2})-(\d{2})-(\d{2})[_ ](\d{2})-(\d{2})-(\d{2})`),
		layout: "2006-01-02 15:04:05",
		build:  func(m []string) string { return m[1] + "-" + m[2] + "-" + m[3] + " " + m[4] + ":" + m[5] + ":" + m[6] },
	},
	{
		re:     regexp.MustCompile(`(20\d{2})-(\d{2})-(\d{2})[ T](\d{2}):(\d{2}):(\d{2})`),
		layout: "2006-01-02 15:04:05",
		build:  func(m []string) string { return m[1] + "-" + m[2] + "-" + m[3] + " " + m[4] + ":" + m[5] + ":" + m[6] },
	},
	{
		re:     regexp.MustCompile(`(20\d{2})-(\d{2})-(\d{2})[ T](\d{2}):(\d{2})`),
		layout: "2006-01-02 15:04",
		build:  func(m []string) string { return m[1] + "-" + m[2] + "-" + m[3] + " " + m[4] + ":" + m[5] },
	},
	{
		re:     regexp.MustCompile(`(20\d{2})-(\d{2})-(\d{2}) (\d{2})(\d{2})`),
		layout: "2006-01-02 15:04",
		build:  func(m []string) string { return m[1] + "-" + m[2] + "-" + m[3] + " " + m[4] + ":" + m[5] },
	},
	{
		re:     regexp.MustCompile(`(20\d{2})-(\d{2})-(\d{2})`),
		layout: "2006-01-02",
		build:  func(m []string) string { return m[1] + "-" + m[2] + "-" + m[3] },
	},
	{
		re:     regexp.MustCompile(`(\d{2})\.(\d{2})\.(20\d{2})`),
		layout: "02.01.2006",
		build:  func(m []string) string { return m[1] + "." + m[2] + "." + m[3] },
	},
	{
		re:     regexp.MustCompile(`(\d{2})-(\d{2})-(20\d{2})`),
		layout: "02-01-2006",
		build:  func(m []string) string { return m[1] + "-" + m[2] + "-" + m[3] },
	},
	{
		re:     regexp.MustCompile(`(20\d{2})`),
		layout: "2006",
		build:  func(m []string) string { return m[1] },
	},
}

func DateFromFilename(filename string) (time.Time, error) {
	for _, pattern := range filenamePatterns {
		match := pattern.re.FindStringSubmatch(filename)
		if match == nil {
			continue
		}
		parsed, err := time.ParseInLocation(pattern.layout, pattern.build(match), time.Local)
		if err != nil {
			return time.Time{}, err
		}
		return parsed, nil
	}
	return time.Time{}, ErrDateNotFound
}
