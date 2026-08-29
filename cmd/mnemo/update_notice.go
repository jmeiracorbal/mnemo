package main

import (
	"context"
	"os"
	"time"

	"github.com/jmeiracorbal/mnemo/internal/updatecheck"
)

func maybeWarnUpdate(args []string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	updatecheck.MaybeNotify(ctx, updatecheck.NotifyOptions{
		CurrentVersion: version,
		Args:           args,
		Stderr:         os.Stderr,
	})
}
