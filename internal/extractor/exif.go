package extractor

import (
	"errors"
	"os"
	"time"

	"github.com/evanoberholster/imagemeta"
)

func DateFromEXIF(path string) (time.Time, error) {
	file, err := os.Open(path)
	if err != nil {
		return time.Time{}, err
	}
	defer file.Close()

	metadata, err := imagemeta.Decode(file)
	if err != nil {
		return time.Time{}, err
	}
	date := metadata.SelectedDate()
	if date.IsZero() {
		return time.Time{}, errors.New("EXIF date not found")
	}
	return date, nil
}
