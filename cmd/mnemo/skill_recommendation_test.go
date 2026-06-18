package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmeiracorbal/mnemo/internal/agentinit"
)

func TestPrintSkillRecommendationWhenMissing(t *testing.T) {
	var out bytes.Buffer
	printSkillRecommendation(&out, t.TempDir())
	if !strings.Contains(out.String(), "npx skills add jmeiracorbal/mnemo --skill mnemo-memory --global") {
		t.Fatalf("missing installation recommendation: %q", out.String())
	}
}

func TestPrintSkillRecommendationWhenInstalled(t *testing.T) {
	home := t.TempDir()
	path := agentinit.GlobalSkillPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("installed"), 0644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	printSkillRecommendation(&out, home)
	if out.Len() != 0 {
		t.Fatalf("unexpected recommendation for installed skill: %q", out.String())
	}
}
