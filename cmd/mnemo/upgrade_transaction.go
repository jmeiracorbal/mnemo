package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"context"

	"github.com/jmeiracorbal/mnemo/internal/db/recovery"
	"github.com/jmeiracorbal/mnemo/internal/store"
)

type upgradeTransaction struct {
	Binary   string `json:"binary"`
	Backup   string `json:"backup"`
	Snapshot string `json:"snapshot,omitempty"`
	Phase    string `json:"phase"`
	Recovered bool `json:"recovered,omitempty"`
	PreviousReady bool `json:"previous_ready,omitempty"`
}

func transactionPath() (string, error) {
	cfg, err := store.DefaultConfig()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfg.DataDir, "upgrade.transaction"), nil
}

func readUpgradeTransaction() (*upgradeTransaction, error) {
	path, err := transactionPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var tx upgradeTransaction
	if err := json.Unmarshal(data, &tx); err != nil {
		return nil, fmt.Errorf("invalid upgrade transaction: %w", err)
	}
	if tx.Phase != "prepared" && tx.Phase != "committed" {
		return nil, fmt.Errorf("invalid upgrade transaction phase %q", tx.Phase)
	}
	if !filepath.IsAbs(tx.Binary) || !filepath.IsAbs(tx.Backup) ||
		filepath.Dir(tx.Binary) != filepath.Dir(tx.Backup) ||
		!strings.HasPrefix(filepath.Base(tx.Backup), filepath.Base(tx.Binary)+".rollback.") {
		return nil, fmt.Errorf("invalid upgrade transaction paths")
	}
	return &tx, nil
}

func persistUpgradeTransaction(tx upgradeTransaction, initial bool) error {
	path, err := transactionPath()
	if err != nil {
		return err
	}
	data, err := json.Marshal(tx)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	dest := path + ".writing"
	if initial {
		if _,err:=os.Lstat(path);err==nil {
			return fmt.Errorf("upgrade transaction already exists")
		} else if !errors.Is(err,os.ErrNotExist) {
			return err
		}
	}
	// An orphaned incomplete write has no authority to replace the journal.
	if err := os.Remove(dest); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	f, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(dest) }()
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if initial {
		if err:=os.Link(dest,path);err!=nil{return err}
	} else {
		if err := os.Rename(dest, path); err != nil {return err}
	}
	return syncTransactionDirectory(filepath.Dir(path))
}

func syncTransactionDirectory(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

func activeTransactionInstaller(binary string) (bool, error) {
	data, err := os.ReadFile(binary + ".upgrading")
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return false, nil
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false, nil
	}
	err = p.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM), nil
}

func beginUpgradeTransaction(binary, backup string) error {
	previous, err := readUpgradeTransaction()
	if err != nil { return err }
	if previous != nil { return fmt.Errorf("unresolved upgrade transaction already exists") }
	binary, err = filepath.Abs(binary)
	if err != nil {
		return err
	}
	backup, err = filepath.Abs(backup)
	if err != nil {
		return err
	}
	backupFile,err:=os.Open(backup)
	if err!=nil{return fmt.Errorf("preserved binary: %w",err)}
	info,err:=backupFile.Stat()
	if err!=nil{_ = backupFile.Close();return err}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0111==0 {
		_ = backupFile.Close()
		return fmt.Errorf("preserved executable is not a regular executable")
	}
	if err:=backupFile.Sync();err!=nil{_ = backupFile.Close();return err}
	if err:=backupFile.Close();err!=nil{return err}
	if err:=syncTransactionDirectory(filepath.Dir(backup));err!=nil{return err}
	if filepath.Dir(binary) != filepath.Dir(backup) || !strings.HasPrefix(filepath.Base(backup),filepath.Base(binary)+".rollback.") {
		return fmt.Errorf("upgrade backup must be in the executable directory")
	}
	return persistUpgradeTransaction(upgradeTransaction{Binary: binary, Backup: backup, Phase: "prepared"}, true)
}

func setUpgradeSnapshot(release string) error {
	tx, err := readUpgradeTransaction()
	if err != nil {
		return err
	}
	if tx == nil || tx.Phase != "prepared" || tx.Snapshot != "" {
		return fmt.Errorf("no uncommitted upgrade transaction without a recovery point")
	}
	cfg, err := store.DefaultConfig()
	if err != nil {
		return err
	}
	if err := recovery.Verify(context.Background(), cfg.DataDir, release); err != nil {
		return err
	}
	tx.Snapshot = release
	return persistUpgradeTransaction(*tx, false)
}

func commitUpgradeTransaction() error {
	tx, err := readUpgradeTransaction()
	if err != nil {
		return err
	}
	if tx == nil || tx.Phase != "prepared" {
		return fmt.Errorf("no prepared upgrade transaction")
	}
	tx.Phase="committed"
	return persistUpgradeTransaction(*tx,false)
}

func finalizeUpgradeTransaction(tx upgradeTransaction) error {
	path, err := transactionPath()
	if err != nil {
		return err
	}
	cfg, err := store.DefaultConfig()
	if err != nil {
		return err
	}
	// Only a committed new release replaces the previous-version slot.
	// An interrupted uncommitted release restores the existing slot unchanged.
	if !tx.Recovered && !tx.PreviousReady {
		pending := tx.Binary+".previous.pending"
		if err:=os.Remove(pending);err!=nil && !errors.Is(err,os.ErrNotExist){return err}
		if err:=os.Link(tx.Backup,pending);err!=nil{return fmt.Errorf("preserve previous executable: %w",err)}
		if err:=os.Rename(pending,tx.Binary+".previous");err!=nil{return err}
		if err:=syncTransactionDirectory(filepath.Dir(tx.Binary));err!=nil{return err}
		tx.PreviousReady=true
		if err:=persistUpgradeTransaction(tx,false);err!=nil{return err}
	}
	// Always release the write fence after the committed phase is durable.
	for _, file := range []string{
		filepath.Join(cfg.DataDir, "upgrade.maintenance"),
		tx.Binary + ".upgrading",
		tx.Binary + ".recovering",
		tx.Backup,
		tx.Binary + ".previous.pending",
		path + ".writing",
	} {
		if err := os.Remove(file); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("finish upgrade %s: %w", file, err)
		}
	}
	if err:=recovery.PruneRecoveryPoints(cfg.DataDir,tx.Snapshot,3);err!=nil {
		return fmt.Errorf("prune completed release recovery points: %w",err)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := syncTransactionDirectory(cfg.DataDir); err != nil {
		return err
	}
	return syncTransactionDirectory(filepath.Dir(tx.Binary))
}

func recoverUpgradeTransaction(force bool) (bool, error) {
	tx, err := readUpgradeTransaction()
	if err != nil || tx == nil {
		return false, err
	}
	if force {
		// Force is reserved for the installer that owns the marker, after it
		// has stopped the managed controller. Never let another process
		// override a live installer and restore SQLite underneath it.
		data, err := os.ReadFile(tx.Binary + ".upgrading")
		if err != nil {
			return false, fmt.Errorf("recovery owner marker: %w", err)
		}
		owner, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil || owner != os.Getppid() {
			return false, fmt.Errorf("forced recovery is restricted to the owning installer")
		}
	} else {
		active, err := activeTransactionInstaller(tx.Binary)
		if err != nil {
			return false, err
		}
		if active {
			return false, nil
		}
	}
	if tx.Phase == "committed" {
		return false, finalizeUpgradeTransaction(*tx)
	}
	cfg, err := store.DefaultConfig()
	if err != nil {
		return false, err
	}
	if tx.Snapshot != "" {
		if err := recovery.Restore(context.Background(), cfg.DataDir, tx.Snapshot); err != nil {
			return false, fmt.Errorf("restore previous SQLite state: %w", err)
		}
	}
	// Keep the backup intact until the transaction is durably marked
	// committed. Retrying after an abrupt exit is always idempotent.
	pending := tx.Binary + ".recovering"
	if err := os.Remove(pending); err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if err := os.Link(tx.Backup, pending); err != nil {
		return false, fmt.Errorf("prepare old executable: %w", err)
	}
	if err := os.Rename(pending, tx.Binary); err != nil {
		return false, fmt.Errorf("restore old executable: %w", err)
	}
	if err := syncTransactionDirectory(filepath.Dir(tx.Binary)); err != nil {
		return false, err
	}
	tx.Phase = "committed"
	tx.Recovered = true
	if err := persistUpgradeTransaction(*tx, false); err != nil {
		return false, err
	}
	if err := finalizeUpgradeTransaction(*tx); err != nil {
		return false, err
	}
	return true, nil
}

func recoverControllerStartup() error {
	recovered, err := recoverUpgradeTransaction(false)
	if err != nil {
		return fmt.Errorf("incomplete Mnemo update requires recovery: %w", err)
	}
	if recovered {
		return fmt.Errorf("interrupted Mnemo update recovered; restarting previous controller")
	}
	// A fence without a durable journal cannot be identified as committed
	// or recoverable. Refuse service startup instead of processing writes.
	tx, err := readUpgradeTransaction()
	if err != nil {
		return err
	}
	if tx == nil {
		cfg, err := store.DefaultConfig()
		if err != nil {
			return err
		}
		if _, err := os.Stat(filepath.Join(cfg.DataDir, "upgrade.maintenance")); err == nil {
			return fmt.Errorf("unresolved SQLite maintenance marker without an upgrade transaction")
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func runControllerTransaction(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mnemo controller transaction <begin|snapshot|commit|finalize|recover>")
	}
	switch args[0] {
	case "begin":
		if len(args) != 3 || !strings.HasPrefix(args[1], "--binary=") || !strings.HasPrefix(args[2], "--backup=") {
			return fmt.Errorf("usage: mnemo controller transaction begin --binary=PATH --backup=PATH")
		}
		return beginUpgradeTransaction(strings.TrimPrefix(args[1], "--binary="), strings.TrimPrefix(args[2], "--backup="))
	case "snapshot":
		if len(args) != 2 || !strings.HasPrefix(args[1], "--release=") {
			return fmt.Errorf("usage: mnemo controller transaction snapshot --release=ID")
		}
		return setUpgradeSnapshot(strings.TrimPrefix(args[1], "--release="))
	case "commit":
		if len(args) != 1 {return fmt.Errorf("usage: mnemo controller transaction commit")}
		return commitUpgradeTransaction()
	case "finalize":
		if len(args)!=1 {return fmt.Errorf("usage: mnemo controller transaction finalize")}
		tx,err:=readUpgradeTransaction()
		if err!=nil{return err}
		if tx==nil || tx.Phase!="committed" {return fmt.Errorf("no committed transaction")}
		return finalizeUpgradeTransaction(*tx)
	case "recover":
		if len(args)>2 || (len(args)==2 && args[1]!="--force") {return fmt.Errorf("usage: mnemo controller transaction recover [--force]")}
		_,err:=recoverUpgradeTransaction(len(args)==2)
		return err
	default:
		return fmt.Errorf("unknown controller transaction operation %q", args[0])
	}
}
