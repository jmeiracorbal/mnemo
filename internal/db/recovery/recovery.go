// Package recovery creates version-addressable, consistent SQLite recovery
// points. Callers must stop the controller and reject writes before restoring.
package recovery

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	_ "modernc.org/sqlite"
)

var releasePattern = regexp.MustCompile(`^v?[0-9]+(?:\.[0-9]+){1,3}(?:-[a-zA-Z0-9.-]+)?$`)

func pointPath(dataDir, release string) (string, error) {
	if !filepath.IsAbs(dataDir) {
		return "", fmt.Errorf("recovery data directory must be absolute")
	}
	if !releasePattern.MatchString(release) {
		return "", fmt.Errorf("invalid release identifier %q", release)
	}
	return filepath.Join(dataDir, "recovery", release+".db"), nil
}

// Snapshot uses SQLite's VACUUM INTO operation rather than copying the database
// file. SQLite includes committed WAL frames in the resulting consistent DB.
func Snapshot(ctx context.Context, dataDir, release string) (string, error) {
	dest, err := pointPath(dataDir, release)
	if err != nil {
		return "", err
	}
	source := filepath.Join(dataDir, "memory.db")
	if _, err := os.Stat(source); err != nil {
		return "", fmt.Errorf("source database: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
		return "", err
	}
	pending := dest + ".pending"
	file, err := os.OpenFile(pending, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", fmt.Errorf("reserve recovery point: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(pending) }()

	db, err := sql.Open("sqlite", source)
	if err != nil {
		return "", err
	}
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(1)
	if err := checkIntegrity(ctx, db); err != nil {
		return "", fmt.Errorf("source integrity: %w", err)
	}
	if _, err := db.ExecContext(ctx, "VACUUM INTO ?", pending); err != nil {
		return "", fmt.Errorf("create WAL-consistent recovery point: %w", err)
	}
	if err := verifyFile(ctx, pending); err != nil {
		return "", fmt.Errorf("verify recovery point: %w", err)
	}
	if err := syncFile(pending); err != nil {
		return "", err
	}
	// Recovery points are immutable. Rename would silently replace an existing
	// snapshot on Unix, so publish through an exclusive hard link instead.
	// Both paths are in the same directory and filesystem.
	if err := os.Link(pending, dest); err != nil {
		return "", fmt.Errorf("commit immutable recovery point %s: %w", release, err)
	}
	if err := syncDir(filepath.Dir(dest)); err != nil {
		return "", err
	}
	return dest, nil
}

// Restore replaces memory.db atomically from a verified recovery point. The
// controller must be fully stopped. It refuses concurrent SQLite WAL holders
// and never restores unverified or incomplete snapshot files.
func Restore(ctx context.Context, dataDir, release string) error {
	point, err := pointPath(dataDir, release)
	if err != nil {
		return err
	}
	if err := verifyFile(ctx, point); err != nil {
		return fmt.Errorf("recovery point is invalid: %w", err)
	}
	dest := filepath.Join(dataDir, "memory.db")
	if _, err := os.Stat(dest); err == nil {
		db, err := sql.Open("sqlite", dest)
		if err != nil {
			return err
		}
		db.SetMaxOpenConns(1)
		var busy, total, checkpointed int
		ckErr := db.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &total, &checkpointed)
		closeErr := db.Close()
		if ckErr != nil {
			return fmt.Errorf("checkpoint before restore: %w", ckErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close database before restore: %w", closeErr)
		}
		if busy != 0 {
			return fmt.Errorf("SQLite is busy; refusing to overwrite an active WAL")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if _, err := os.Stat(dest + suffix); err == nil {
			return fmt.Errorf("SQLite sidecar %s still exists; refusing unsafe restore", dest+suffix)
		} else if !os.IsNotExist(err) {
			return err
		}
	}

	pending := dest + ".restore-pending"
	// A crash can leave a partial restore copy. Its contents are never
	// trusted: the verified immutable recovery point is copied afresh.
	if err := os.Remove(pending); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove interrupted restore copy: %w", err)
	}
	input, err := os.Open(point)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(pending, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("reserve restore file: %w", err)
	}
	defer func() { _ = os.Remove(pending) }()
	_, copyErr := io.Copy(output, input)
	syncErr := output.Sync()
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	if syncErr != nil {
		return syncErr
	}
	if closeErr != nil {
		return closeErr
	}
	// Detect corruption before changing the live database.
	if err := verifyFile(ctx, pending); err != nil {
		return fmt.Errorf("restored copy failed validation: %w", err)
	}
	if err := os.Rename(pending, dest); err != nil {
		return fmt.Errorf("activate recovered database: %w", err)
	}
	return syncDir(dataDir)
}

func Verify(ctx context.Context, dataDir, release string) error {
	path, err := pointPath(dataDir, release)
	if err != nil {
		return err
	}
	return verifyFile(ctx, path)
}

func verifyFile(ctx context.Context, path string) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	return checkIntegrity(ctx, db)
}

func checkIntegrity(ctx context.Context, db *sql.DB) error {
	var result string
	if err := db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return fmt.Errorf("SQLite integrity check failed: %s", result)
	}
	return nil
}

func syncFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

func syncDir(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}


// PruneRecoveryPoints retains the newest recovery points after a committed
// update. The active release's point is protected, and incomplete .pending
// files are discarded only after the release transaction has ended.
func PruneRecoveryPoints(dataDir, protected string, retain int) error {
	if retain < 1 {
		return fmt.Errorf("at least one recovery point must be retained")
	}
	dir := filepath.Join(dataDir, "recovery")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	type candidate struct {
		name string
		mod  int64
	}
	var points []candidate
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) == ".pending" {
			if err := os.Remove(filepath.Join(dir, name)); err != nil {
				return err
			}
			continue
		}
		if filepath.Ext(name) != ".db" {
			continue
		}
		if _, err := pointPath(dataDir, name[:len(name)-3]); err != nil {
			// Unknown files are not Mnemo recovery points and must be kept.
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		points = append(points, candidate{name: name, mod: info.ModTime().UnixNano()})
	}
	sort.Slice(points, func(i, j int) bool {
		if points[i].mod == points[j].mod {
			return points[i].name > points[j].name
		}
		return points[i].mod > points[j].mod
	})
	kept := 0
	for _, entry := range points {
		if entry.name == protected+".db" || kept < retain {
			kept++
			continue
		}
		if err := os.Remove(filepath.Join(dir, entry.name)); err != nil {
			return err
		}
	}
	return syncDir(dir)
}
