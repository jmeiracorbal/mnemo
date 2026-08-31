package store

import (
	"os"
	"regexp"
	"testing"
)

func TestStoreDoesNotOwnSchemaDDL(t *testing.T) {
	data, err := os.ReadFile("store.go")
	if err != nil {
		t.Fatalf("read store.go: %v", err)
	}
	ddl := regexp.MustCompile(`(?i)\b(CREATE\s+(TABLE|INDEX|TRIGGER|VIRTUAL\s+TABLE)|ALTER\s+TABLE|DROP\s+TABLE)\b`)
	if match := ddl.FindString(string(data)); match != "" {
		t.Fatalf("store.go must delegate schema DDL to database/migrations; found %q", match)
	}
}
