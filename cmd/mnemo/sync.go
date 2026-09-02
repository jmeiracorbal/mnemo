package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jmeiracorbal/mnemo/internal/cloudsync"
	"github.com/jmeiracorbal/mnemo/internal/store"
	"github.com/spf13/cobra"
)

// newSyncCommand builds the `mnemo sync` command tree with proper Cobra flags.
func newSyncCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "sync",
		Short: "Synchronize local memory with cloud",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = cmd.Help()
			return fmt.Errorf("a subcommand is required")
		},
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.AddCommand(
		newSyncRunCommand(),
		newSyncPushCommand(),
		newSyncPullCommand(),
		newSyncStatusCommand(),
	)
	return root
}

// syncCloudFlags are shared across run/push/pull subcommands.
type syncCloudFlags struct {
	url      string
	key      string
	clientID string
	target   string
	batch    int
	jsonOut  bool
}

func addSyncCloudFlags(cmd *cobra.Command, f *syncCloudFlags) {
	cmd.Flags().StringVar(&f.url, "url", "", "Cloud database URL (overrides config/env)")
	cmd.Flags().StringVar(&f.key, "key", "", "Cloud auth token (overrides config/env)")
	cmd.Flags().StringVar(&f.clientID, "client-id", "", "Unique client identifier (overrides config/env)")
	cmd.Flags().StringVar(&f.target, "target", store.DefaultSyncTargetKey, "Sync target key")
	cmd.Flags().IntVar(&f.batch, "batch", 0, "Batch size for mutations (default 25)")
	cmd.Flags().BoolVar(&f.jsonOut, "json", false, "Output result as JSON")
}

func resolveCloudConfig(f syncCloudFlags) (cloudsync.Config, error) {
	cfg, _ := cloudsync.ConfigFromEnv()
	if f.url != "" {
		cfg.URL = f.url
	}
	if f.key != "" {
		cfg.Key = f.key
	}
	if f.clientID != "" {
		cfg.ClientID = f.clientID
	}
	if f.target != "" {
		cfg.TargetKey = f.target
	}
	if f.batch > 0 {
		cfg.BatchSize = f.batch
	}
	return cfg.Validate()
}

func newSyncRunCommand() *cobra.Command {
	var f syncCloudFlags
	cmd := &cobra.Command{
		Use:           "run",
		Short:         "Push then pull all cloud mutations (default sync action)",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runSyncAction("run", f)
		},
	}
	addSyncCloudFlags(cmd, &f)
	return cmd
}

func newSyncPushCommand() *cobra.Command {
	var f syncCloudFlags
	cmd := &cobra.Command{
		Use:           "push",
		Short:         "Push all local mutations to cloud",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runSyncAction("push", f)
		},
	}
	addSyncCloudFlags(cmd, &f)
	return cmd
}

func newSyncPullCommand() *cobra.Command {
	var f syncCloudFlags
	cmd := &cobra.Command{
		Use:           "pull",
		Short:         "Pull cloud mutations into local store",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runSyncAction("pull", f)
		},
	}
	addSyncCloudFlags(cmd, &f)
	return cmd
}

func newSyncStatusCommand() *cobra.Command {
	var target string
	var jsonOut bool
	cmd := &cobra.Command{
		Use:           "status",
		Short:         "Show local cloud sync state",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if target == "" {
				target = store.DefaultSyncTargetKey
			}
			cfg, err := store.DefaultConfig()
			if err != nil {
				return fmt.Errorf("config error: %w", err)
			}
			s, err := store.New(cfg)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer s.Close()
			return runSyncStatus(s, target, jsonOut)
		},
	}
	cmd.Flags().StringVar(&target, "target", store.DefaultSyncTargetKey, "Sync target key")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output result as JSON")
	return cmd
}

func runSyncStatus(s *store.Store, target string, jsonOut bool) error {
	_ = s.BackfillAllSyncMutations()
	state, err := s.GetSyncState(target)
	if err != nil {
		return fmt.Errorf("sync status: %w", err)
	}
	pending, err := s.ListAllPendingSyncMutations(target, 1_000_000)
	if err != nil {
		return fmt.Errorf("sync status: %w", err)
	}
	out := map[string]any{
		"target_key":        state.TargetKey,
		"lifecycle":         state.Lifecycle,
		"pending":           len(pending),
		"last_enqueued_seq": state.LastEnqueuedSeq,
		"last_acked_seq":    state.LastAckedSeq,
		"last_pulled_seq":   state.LastPulledSeq,
	}
	if jsonOut {
		return printJSONValue(out)
	}
	fmt.Printf("Sync target:            %s\nLifecycle:              %s\nPending local mutations:%d\nLast enqueued:          %d\nLast acked:             %d\nLast pulled:            %d\n",
		state.TargetKey, state.Lifecycle, len(pending), state.LastEnqueuedSeq, state.LastAckedSeq, state.LastPulledSeq)
	return nil
}

func runSyncAction(action string, f syncCloudFlags) error {
	cfg, err := resolveCloudConfig(f)
	if err != nil {
		return handleCloudConfigError(err, f.jsonOut)
	}

	backend, err := cloudsync.NewBackend(cfg)
	if err != nil {
		return handleCloudConnError(err, f.jsonOut)
	}

	storeCfg, err := store.DefaultConfig()
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}
	s, err := store.New(storeCfg)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer s.Close()

	engine, err := cloudsync.NewEngine(s, backend, cfg)
	if err != nil {
		return fmt.Errorf("sync engine: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	var res *cloudsync.Result
	switch action {
	case "push":
		res, err = engine.Push(ctx)
	case "pull":
		res, err = engine.Pull(ctx)
	default:
		res, err = engine.Sync(ctx)
	}
	if err != nil {
		return fmt.Errorf("sync %s: %w", action, err)
	}

	if f.jsonOut {
		return printJSONValue(res)
	}
	fmt.Printf("Sync %s complete: pushed=%d pulled=%d skipped_own=%d pending=%d latest_seq=%d lifecycle=%s\n",
		action, res.Pushed, res.Pulled, res.SkippedOwn, res.Pending, res.LatestSeq, res.Lifecycle)
	return nil
}

// handleCloudConfigError surfaces a missing-config error and, in interactive
// mode, offers to run `mnemo setup cloud`.
func handleCloudConfigError(err error, jsonOut bool) error {
	if jsonOut {
		return err
	}
	fmt.Fprintf(os.Stderr, "mnemo: cloud not configured: %v\n\n", err)
	fmt.Fprintln(os.Stderr, "Run 'mnemo setup cloud' to configure cloud sync credentials.")
	os.Exit(1)
	return nil
}

// handleCloudConnError surfaces a connection failure and, in interactive mode,
// lets the user choose how to proceed.
func handleCloudConnError(err error, jsonOut bool) error {
	if jsonOut {
		return fmt.Errorf("cloud connection failed: %w", err)
	}
	fmt.Fprintf(os.Stderr, "\n✗ Cloud connection failed: %v\n\n", err)
	if !isInteractiveTTY() {
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "  [d] Delete cloud config   — disable sync, keep local data")
	fmt.Fprintln(os.Stderr, "  [r] Reconfigure           — run 'mnemo setup cloud' now")
	fmt.Fprintln(os.Stderr, "  [s] Skip for this run     — continue without cloud sync")
	fmt.Fprint(os.Stderr, "\nChoice [d/r/s]: ")

	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		os.Exit(1)
	}
	switch strings.TrimSpace(strings.ToLower(scanner.Text())) {
	case "d":
		if delErr := cloudsync.DeleteFileConfig(); delErr != nil {
			fmt.Fprintf(os.Stderr, "mnemo: delete config: %v\n", delErr)
		} else {
			fmt.Fprintln(os.Stderr, "Cloud config deleted.")
		}
		os.Exit(0)
	case "r":
		fmt.Fprintln(os.Stderr, "Run: mnemo setup cloud")
		os.Exit(0)
	default: // "s" or anything else
		os.Exit(0)
	}
	return nil
}

func isInteractiveTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func printJSONValue(v any) error {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}
	fmt.Println(string(out))
	return nil
}
