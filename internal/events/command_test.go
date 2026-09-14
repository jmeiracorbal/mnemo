package events

import "testing"

func TestCommandRequiresCanonicalExecutionIdentity(t *testing.T) {
	if _, err := NewCommand("memory.save", "project", "", map[string]string{"title": "x"}); err == nil {
		t.Fatal("expected execution-key validation error")
	}
	command, err := NewCommand("memory.save", "project", "execution", map[string]string{"title": "x"})
	if err != nil || command.ID == "" {
		t.Fatalf("command = %#v, err = %v", command, err)
	}
}
