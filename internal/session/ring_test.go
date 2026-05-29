package session

import (
	"bytes"
	"testing"
)

func TestRingBufferKeepsTail(t *testing.T) {
	r := newRingBuffer(4)
	r.Write([]byte("ab"))
	if got := r.Snapshot(); !bytes.Equal(got, []byte("ab")) {
		t.Fatalf("got %q, want %q", got, "ab")
	}
	r.Write([]byte("cdef")) // total "abcdef", max 4 -> keep "cdef"
	if got := r.Snapshot(); !bytes.Equal(got, []byte("cdef")) {
		t.Fatalf("got %q, want %q", got, "cdef")
	}
}

func TestRingBufferSnapshotIsCopy(t *testing.T) {
	r := newRingBuffer(8)
	r.Write([]byte("xy"))
	snap := r.Snapshot()
	snap[0] = 'Z'
	if got := r.Snapshot(); !bytes.Equal(got, []byte("xy")) {
		t.Fatalf("snapshot mutation leaked into buffer: %q", got)
	}
}
