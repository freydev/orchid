package wake

import (
	"os/exec"
	"testing"
)

// Uses `sleep` as a stand-in for caffeinate so the test never touches the
// real power-management state.
func TestKeeperToggle(t *testing.T) {
	bin, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("no sleep binary")
	}
	k := &Keeper{bin: bin, args: []string{"60"}}

	k.Set(true)
	if !k.Active() {
		t.Fatal("expected active after Set(true)")
	}
	first := k.cmd
	k.Set(true)
	if k.cmd != first {
		t.Fatal("Set(true) twice must not spawn a second process")
	}
	k.Set(false)
	if k.Active() {
		t.Fatal("expected inactive after Set(false)")
	}
	k.Set(false) // idempotent
}

func TestKeeperNoop(t *testing.T) {
	k := &Keeper{}
	k.Set(true)
	if k.Active() {
		t.Fatal("no-op keeper must never be active")
	}
}
