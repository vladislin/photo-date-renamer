# Photo Date Renamer

A safe Windows desktop application that scans photos and videos, reads their creation date from metadata or the filename, and copies or moves them into consistently named result folders.

## Features

- Preview before changing anything.
- Readable rich-text preview with color-coded statuses.
- Copy or move modes.
- EXIF dates from JPEG, PNG and HEIC images.
- QuickTime/MOV container creation dates.
- Filename date fallback for JPG, JPEG, PNG, HEIC and MOV.
- Collision-safe names such as `2024-08-15_143052000.jpg`.
- Unsupported files are left untouched.
- The generated `results` directory is excluded from future scans.
- Operation log at `results/metadata/log.txt`.

## Run during development

```shell
go run ./cmd/renamer
```

Preview status colors adapt to the active light or dark theme:

- `READY` and `SUCCESS` — green.
- `UNPROCESSED` — orange.
- `SKIPPED` — gray.
- `ERROR` — red.

## Verify changes

```shell
go vet ./...
go build ./cmd/renamer
```

After building, run the application and check the changed workflow manually with the sample media folder in Preview mode.

## Build for Windows 11

Build on Windows with Go and the Fyne prerequisites installed:

```powershell
go install fyne.io/tools/cmd/fyne@latest
fyne package -os windows -icon Icon.png -release
```

An `Icon.png` can be added later. For a plain executable without packaging metadata:

```powershell
go build -ldflags="-H windowsgui -s -w" -o dist\PhotoDateRenamer.exe .\cmd\renamer
```

## Date priority

1. EXIF date for images.
2. QuickTime container creation time for MOV.
3. Date embedded in the filename.

The metadata decoder supports JPEG, PNG and HEIC containers. Files without a usable metadata date fall back to their filename.
