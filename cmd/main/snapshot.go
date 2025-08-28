package main

import (
	"fmt"
	"log/slog"
	"os"
	"path"
	"time"

	"github.com/ssqueue/ssqueue/internal/application"
)

func loadLastSnapshot(snapshotPath string) (filename string, data []byte, err error) {
	entries, errReadDir := os.ReadDir(snapshotPath)
	if errReadDir != nil {
		return "", nil, fmt.Errorf("reading snapshot dir %q failed: %s", snapshotPath, errReadDir.Error())
	}
	if len(entries) == 0 {
		return "", nil, nil
	}
	// Find the latest snapshot by name (assuming lexicographical order corresponds to creation time)
	latest := entries[0]
	for _, entry := range entries[1:] {
		if entry.Name() > latest.Name() {
			latest = entry
		}
	}
	filepath := path.Join(snapshotPath, latest.Name())
	data, errReadFile := os.ReadFile(filepath)
	if errReadFile != nil {
		return "", nil, fmt.Errorf("reading snapshot file %q failed: %s", filepath, errReadFile.Error())
	}

	return latest.Name(), data, nil
}

func saveSnapshot(snapshotPath string, data []byte) (filename string, err error) {
	filename = fmt.Sprintf("ssq-%d.snap", time.Now().UTC().UnixNano())
	filepath := path.Join(snapshotPath, filename)
	errWrite := os.WriteFile(filepath, data, 0o644)
	if errWrite != nil {
		return "", fmt.Errorf("writing snapshot file %q failed: %s", filepath, errWrite.Error())
	}

	return filename, nil
}

func toSnapshot(snapshotPath string, app *application.Application) error {
	data, errSnapshot := app.ToSnapshot()
	if errSnapshot != nil {
		return fmt.Errorf("creating snapshot failed: %s", errSnapshot.Error())
	}

	if len(data) == 0 {
		slog.Info("no data to snapshot")
		return nil
	}

	filename, errSave := saveSnapshot(snapshotPath, data)
	if errSave != nil {
		return fmt.Errorf("saving snapshot failed: %s", errSave.Error())
	}

	slog.Info("saved snapshot", slog.String("snapshot", filename))
	return nil
}

func fromSnapshot(snapshotPath string, app *application.Application) error {
	snapshotFilename, snapshotData, errLoadSnapshot := loadLastSnapshot(snapshotPath)
	if errLoadSnapshot != nil {
		return fmt.Errorf("loading last snapshot failed: %s", errLoadSnapshot.Error())
	}

	if snapshotFilename == "" {
		slog.Info("no snapshots found")
		return nil
	}

	errSnapshot := app.FromSnapshot(snapshotData)
	if errSnapshot != nil {
		return fmt.Errorf("restoring from snapshot %q failed: %s", snapshotFilename, errSnapshot.Error())
	}

	errRemove := os.Remove(path.Join(snapshotPath, snapshotFilename))
	if errRemove != nil {
		slog.Warn("removing snapshot file failed", slog.String("snapshot", snapshotFilename), slog.String("error", errRemove.Error()))
	}

	slog.Info("restored from snapshot", slog.String("snapshot", snapshotFilename))
	return nil
}
