package processor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"photo-date-renamer/internal/extractor"
	"photo-date-renamer/internal/model"
)

const filenameLayout = "2006-01-02_150405000"

var supportedExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".heic": true, ".mov": true,
}

type ProgressFunc func(done, total int, result model.Result)

func Plan(ctx context.Context, root string) ([]model.Result, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", root)
	}

	resultsRoot := filepath.Join(root, "results")
	var paths []string
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			if samePath(path, resultsRoot) {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)

	reserved := make(map[string]bool)
	results := make([]model.Result, 0, len(paths))
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		result := planFile(path, root, reserved)
		results = append(results, result)
	}
	return results, nil
}

func planFile(path, root string, reserved map[string]bool) model.Result {
	name := filepath.Base(path)
	result := model.Result{SourcePath: path, OriginalName: name}
	extension := strings.ToLower(filepath.Ext(name))
	if !supportedExtensions[extension] {
		result.Status = model.StatusSkipped
		result.Reason = "unsupported extension"
		return result
	}

	date, source, err := extractDate(path, extension)
	if err != nil {
		result.Status = model.StatusUnprocessed
		result.DateSource = model.DateSourceNone
		result.Reason = "no supported date in metadata or filename"
		result.DestinationPath = uniqueUnprocessedPath(filepath.Join(root, "results", "unprocessed"), name, reserved)
		return result
	}

	result.Date = date
	result.DateSource = source
	result.Status = model.StatusReady
	processedDir := filepath.Join(root, "results", "processed")
	result.DestinationPath, result.Date = uniqueDatedPath(processedDir, extension, date, reserved)
	return result
}

func extractDate(path, extension string) (time.Time, model.DateSource, error) {
	if extension == ".jpg" || extension == ".jpeg" || extension == ".png" || extension == ".heic" {
		if date, err := extractor.DateFromEXIF(path); err == nil {
			return date, model.DateSourceEXIF, nil
		}
	}
	if extension == ".mov" {
		if date, err := extractor.DateFromQuickTime(path); err == nil {
			return date, model.DateSourceQuickTime, nil
		}
	}
	if date, err := extractor.DateFromFilename(filepath.Base(path)); err == nil {
		return date, model.DateSourceFilename, nil
	}
	return time.Time{}, "", extractor.ErrDateNotFound
}

func uniqueDatedPath(dir, extension string, date time.Time, reserved map[string]bool) (string, time.Time) {
	for {
		candidate := filepath.Join(dir, date.Format(filenameLayout)+extension)
		if isAvailable(candidate, reserved) {
			reserved[pathKey(candidate)] = true
			return candidate, date
		}
		date = date.Add(time.Millisecond)
	}
}

func uniqueUnprocessedPath(dir, name string, reserved map[string]bool) string {
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for index := 0; ; index++ {
		candidateName := name
		if index > 0 {
			candidateName = fmt.Sprintf("%s (%d)%s", stem, index, ext)
		}
		candidate := filepath.Join(dir, candidateName)
		if isAvailable(candidate, reserved) {
			reserved[pathKey(candidate)] = true
			return candidate
		}
	}
}

func isAvailable(path string, reserved map[string]bool) bool {
	if reserved[pathKey(path)] {
		return false
	}
	_, err := os.Stat(path)
	return errors.Is(err, os.ErrNotExist)
}

func Execute(ctx context.Context, results []model.Result, mode model.Mode, progress ProgressFunc) ([]model.Result, error) {
	if mode != model.ModeCopy && mode != model.ModeMove {
		return nil, fmt.Errorf("invalid execution mode: %s", mode)
	}
	updated := append([]model.Result(nil), results...)
	total := 0
	for _, result := range updated {
		if result.Status == model.StatusReady || result.Status == model.StatusUnprocessed {
			total++
		}
	}
	done := 0
	for index := range updated {
		if err := ctx.Err(); err != nil {
			return updated, err
		}
		result := &updated[index]
		if result.Status != model.StatusReady && result.Status != model.StatusUnprocessed {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(result.DestinationPath), 0o755); err != nil {
			result.Status, result.Err = model.StatusError, err
		} else if err := transfer(result.SourcePath, result.DestinationPath, mode); err != nil {
			result.Status, result.Err = model.StatusError, err
		} else {
			result.Status = model.StatusSuccess
		}
		done++
		if progress != nil {
			progress(done, total, *result)
		}
	}
	if err := writeLog(projectRoot(updated), updated, mode); err != nil {
		return updated, err
	}
	return updated, nil
}

func transfer(source, destination string, mode model.Mode) error {
	if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
		if err == nil {
			return fmt.Errorf("destination already exists: %s", destination)
		}
		return err
	}
	if mode == model.ModeMove {
		if err := os.Rename(source, destination); err == nil {
			return nil
		}
	}
	if err := copyFile(source, destination); err != nil {
		return err
	}
	if mode == model.ModeMove {
		if err := os.Remove(source); err != nil {
			return fmt.Errorf("copied but could not remove source: %w", err)
		}
	}
	return nil
}

func copyFile(source, destination string) (returnErr error) {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := out.Close(); returnErr == nil {
			returnErr = closeErr
		}
		if returnErr != nil {
			_ = os.Remove(destination)
		}
	}()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	if err := out.Sync(); err != nil {
		return err
	}
	return os.Chtimes(destination, info.ModTime(), info.ModTime())
}

func writeLog(root string, results []model.Result, mode model.Mode) error {
	if root == "." || root == "" {
		return nil
	}
	metadataDir := filepath.Join(root, "results", "metadata")
	if err := os.MkdirAll(metadataDir, 0o755); err != nil {
		return err
	}
	file, err := os.Create(filepath.Join(metadataDir, "log.txt"))
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := fmt.Fprintf(file, "Mode: %s\nGenerated: %s\n\n", mode, time.Now().Format(time.RFC3339)); err != nil {
		return err
	}
	for _, result := range results {
		errorText := ""
		if result.Err != nil {
			errorText = " | " + result.Err.Error()
		}
		if _, err := fmt.Fprintf(file, "%s | %s | %s | %s%s\n", result.Status, result.DateSource, result.SourcePath, result.DestinationPath, errorText); err != nil {
			return err
		}
	}
	return file.Sync()
}

func projectRoot(results []model.Result) string {
	for _, result := range results {
		if result.DestinationPath != "" {
			// destination is <root>/results/(processed|unprocessed)/file
			return filepath.Dir(filepath.Dir(filepath.Dir(result.DestinationPath)))
		}
	}
	return ""
}

func samePath(left, right string) bool { return pathKey(left) == pathKey(right) }
func pathKey(path string) string       { return strings.ToLower(filepath.Clean(path)) }
