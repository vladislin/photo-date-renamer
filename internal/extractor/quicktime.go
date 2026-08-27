package extractor

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
	"time"
)

const quickTimeToUnixSeconds = 2_082_844_800

// DateFromQuickTime reads the creation time from the top-level moov/mvhd atom.
func DateFromQuickTime(path string) (time.Time, error) {
	file, err := os.Open(path)
	if err != nil {
		return time.Time{}, err
	}
	defer file.Close()

	moovOffset, moovSize, err := findAtom(file, 0, -1, "moov")
	if err != nil {
		return time.Time{}, err
	}
	mvhdOffset, _, err := findAtom(file, moovOffset, moovSize, "mvhd")
	if err != nil {
		return time.Time{}, err
	}
	if _, err := file.Seek(mvhdOffset, io.SeekStart); err != nil {
		return time.Time{}, err
	}
	fullBoxHeader := make([]byte, 4)
	if _, err := io.ReadFull(file, fullBoxHeader); err != nil {
		return time.Time{}, err
	}
	version := fullBoxHeader[0]
	var seconds uint64
	if version == 0 {
		creation := make([]byte, 4)
		if _, err := io.ReadFull(file, creation); err != nil {
			return time.Time{}, err
		}
		seconds = uint64(binary.BigEndian.Uint32(creation))
	} else if version == 1 {
		extra := make([]byte, 8)
		if _, err := io.ReadFull(file, extra); err != nil {
			return time.Time{}, err
		}
		seconds = binary.BigEndian.Uint64(extra)
	} else {
		return time.Time{}, errors.New("unsupported QuickTime mvhd version")
	}
	if seconds <= quickTimeToUnixSeconds {
		return time.Time{}, errors.New("invalid QuickTime creation time")
	}
	return time.Unix(int64(seconds-quickTimeToUnixSeconds), 0).UTC(), nil
}

// findAtom returns the payload offset and payload size for an ISO BMFF atom.
func findAtom(reader *os.File, start, length int64, wanted string) (int64, int64, error) {
	position := start
	var end int64 = -1
	if length >= 0 {
		end = start + length
	}
	for end < 0 || position+8 <= end {
		if _, err := reader.Seek(position, io.SeekStart); err != nil {
			return 0, 0, err
		}
		header := make([]byte, 8)
		if _, err := io.ReadFull(reader, header); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				break
			}
			return 0, 0, err
		}
		size := int64(binary.BigEndian.Uint32(header[:4]))
		headerSize := int64(8)
		if size == 1 {
			extended := make([]byte, 8)
			if _, err := io.ReadFull(reader, extended); err != nil {
				return 0, 0, err
			}
			size = int64(binary.BigEndian.Uint64(extended))
			headerSize = 16
		} else if size == 0 {
			info, err := reader.Stat()
			if err != nil {
				return 0, 0, err
			}
			size = info.Size() - position
		}
		if size < headerSize {
			return 0, 0, errors.New("invalid QuickTime atom size")
		}
		if string(header[4:8]) == wanted {
			return position + headerSize, size - headerSize, nil
		}
		position += size
	}
	return 0, 0, ErrDateNotFound
}
