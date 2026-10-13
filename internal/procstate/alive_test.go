package procstate

import (
	"os"
	"os/exec"
	"testing"
)

func TestAlive(t *testing.T) {
	if !Alive(os.Getpid()) {
		t.Fatal("current process reported as not alive")
	}
	if Alive(0) || Alive(-1) {
		t.Fatal("invalid pid reported as alive")
	}
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	if err := cmd.Run(); err != nil {
		t.Fatalf("run child: %v", err)
	}
	if Alive(cmd.Process.Pid) {
		t.Fatalf("exited child %d reported as alive", cmd.Process.Pid)
	}
}
