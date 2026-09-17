package events

import "testing"

func TestCommandRequiresAction(t *testing.T) {
	if _, err := NewCommand("", map[string]string{"title": "x"}); err == nil {
		t.Fatal("expected action validation error")
	}
	command, err := NewCommand("add_observation", map[string]string{"title": "x"})
	if err != nil || command.ID == "" {
		t.Fatalf("command = %#v, err = %v", command, err)
	}
}
