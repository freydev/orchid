package runtime

import (
	"os"
	"testing"
)

func TestPathHonorsXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/cfg")
	got, err := Path()
	if err != nil || got != "/cfg/orchid/daemon.json" {
		t.Fatalf("Path = %q, %v", got, err)
	}
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/home/x")
	got, _ = Path()
	if got != "/home/x/.config/orchid/daemon.json" {
		t.Fatalf("fallback Path = %q", got)
	}
}

func TestWriteReadRemove(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if err := Write(Info{PID: 4321, Addr: "127.0.0.1:9999"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.PID != 4321 || got.Addr != "127.0.0.1:9999" {
		t.Fatalf("Read = %+v", got)
	}
	if err := Remove(); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := Read(); !os.IsNotExist(err) {
		t.Fatalf("Read after Remove = %v, want not-exist", err)
	}
	if err := Remove(); err != nil {
		t.Fatalf("Remove twice: %v", err)
	}
}

func TestAlive(t *testing.T) {
	if !Alive(os.Getpid()) {
		t.Fatal("own pid should be alive")
	}
	if Alive(0) || Alive(-1) {
		t.Fatal("invalid pids should be dead")
	}
	if Alive(4000000) {
		t.Skip("pid 4000000 unexpectedly alive; skipping")
	}
}
