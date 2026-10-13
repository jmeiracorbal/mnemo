package store

import (
	"testing"
)

func TestCloseOrphanedMCPSessionsClosesOnlyExitedServers(t *testing.T) {
	s := newTestStore(t)
	dir := t.TempDir()
	live, err := s.ResolveMCPInstanceSession("alpha", dir, "instance-live", 100)
	if err != nil {
		t.Fatal(err)
	}
	dead, err := s.ResolveMCPInstanceSession("alpha", dir, "instance-dead", 200)
	if err != nil {
		t.Fatal(err)
	}
	deadOther, err := s.ResolveMCPInstanceSession("beta", dir, "instance-dead", 200)
	if err != nil {
		t.Fatal(err)
	}
	alive := func(pid int) bool { return pid == 100 }

	closed, err := s.CloseOrphanedMCPSessions(alive)
	if err != nil {
		t.Fatalf("close orphaned sessions: %v", err)
	}
	if closed != 1 {
		t.Fatalf("closed instances = %d, want 1", closed)
	}
	for id, wantOpen := range map[string]bool{live: true, dead: false, deadOther: false} {
		session, err := s.GetSession(id)
		if err != nil {
			t.Fatalf("get session %s: %v", id, err)
		}
		if open := session.EndedAt == nil; open != wantOpen {
			t.Fatalf("session %s open = %v, want %v", id, open, wantOpen)
		}
	}

	closed, err = s.CloseOrphanedMCPSessions(alive)
	if err != nil || closed != 0 {
		t.Fatalf("second sweep closed=%d err=%v, want 0/nil", closed, err)
	}
}
