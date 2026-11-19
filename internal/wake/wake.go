// Package wake keeps the machine from idle-sleeping while agents are working.
//
// On macOS it holds a `caffeinate -i -w <orchid pid>` child: -i blocks idle
// system sleep (the display may still sleep), -w ties the assertion to orchid's
// lifetime so a crashed daemon never leaves the Mac awake forever. On systems
// without caffeinate the Keeper is a no-op.
package wake

import (
	"log"
	"os"
	"os/exec"
	"strconv"
	"sync"
)

// Keeper toggles a sleep-prevention assertion. Safe for concurrent use.
type Keeper struct {
	mu   sync.Mutex
	cmd  *exec.Cmd
	bin  string // resolved caffeinate path; "" = unsupported
	args []string
}

// New returns a Keeper backed by caffeinate, or a no-op one if it is missing.
func New() *Keeper {
	bin, err := exec.LookPath("caffeinate")
	if err != nil {
		return &Keeper{}
	}
	return &Keeper{bin: bin, args: []string{"-i", "-w", strconv.Itoa(os.Getpid())}}
}

// Set holds the assertion when on is true and releases it when false.
// Repeated calls with the same value are cheap no-ops.
func (k *Keeper) Set(on bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.bin == "" {
		return
	}
	if on {
		if k.cmd != nil {
			return
		}
		cmd := exec.Command(k.bin, k.args...)
		if err := cmd.Start(); err != nil {
			log.Printf("orchid: keep awake: %v", err)
			return
		}
		k.cmd = cmd
		go func() { _ = cmd.Wait() }() // reap; also clears an externally killed child
		log.Print("orchid: agents running, preventing idle sleep")
		return
	}
	if k.cmd == nil {
		return
	}
	_ = k.cmd.Process.Kill()
	k.cmd = nil
	log.Print("orchid: no agents running, idle sleep allowed")
}

// Active reports whether the assertion is currently held.
func (k *Keeper) Active() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.cmd != nil
}
