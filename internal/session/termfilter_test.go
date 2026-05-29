package session

import (
	"bytes"
	"testing"
)

func TestFilterPassesPlainText(t *testing.T) {
	var f queryFilter
	in := []byte("hello\r\nworld")
	if got := f.Filter(in); !bytes.Equal(got, in) {
		t.Fatalf("got %q, want %q", got, in)
	}
}

func TestFilterStripsDA1(t *testing.T) {
	var f queryFilter
	if got := f.Filter([]byte("a\x1b[cb")); !bytes.Equal(got, []byte("ab")) {
		t.Fatalf("got %q", got)
	}
}

func TestFilterStripsDA1WithZero(t *testing.T) {
	var f queryFilter
	if got := f.Filter([]byte("\x1b[0c")); len(got) != 0 {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestFilterStripsDA2(t *testing.T) {
	var f queryFilter
	if got := f.Filter([]byte("a\x1b[>cb")); !bytes.Equal(got, []byte("ab")) {
		t.Fatalf("got %q", got)
	}
}

func TestFilterStripsXTVersion(t *testing.T) {
	var f queryFilter
	if got := f.Filter([]byte("\x1b[>q")); len(got) != 0 {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestFilterStrips996(t *testing.T) {
	var f queryFilter
	if got := f.Filter([]byte("x\x1b[?996ny")); !bytes.Equal(got, []byte("xy")) {
		t.Fatalf("got %q", got)
	}
}

func TestFilterStripsColorQueriesBEL(t *testing.T) {
	var f queryFilter
	if got := f.Filter([]byte("\x1b]10;?\x07\x1b]11;?\x07")); len(got) != 0 {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestFilterStripsColorQueriesST(t *testing.T) {
	var f queryFilter
	if got := f.Filter([]byte("\x1b]10;?\x1b\\\x1b]11;?\x1b\\")); len(got) != 0 {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestFilterKeepsCursorMoves(t *testing.T) {
	var f queryFilter
	in := []byte("\x1b[H\x1b[2J\x1b[1;1Hhi\x1b[m")
	if got := f.Filter(in); !bytes.Equal(got, in) {
		t.Fatalf("got %q, want %q", got, in)
	}
}

func TestFilterKeepsTitleAndColorSet(t *testing.T) {
	var f queryFilter
	// OSC title set and OSC 11 SET (not a query) must pass through.
	in := []byte("\x1b]0;mytitle\x07\x1b]11;rgb:0000/0000/0000\x1b\\")
	if got := f.Filter(in); !bytes.Equal(got, in) {
		t.Fatalf("got %q, want %q", got, in)
	}
}

func TestFilterSplitCSIAcrossCalls(t *testing.T) {
	var f queryFilter
	if got := f.Filter([]byte("ab\x1b[")); !bytes.Equal(got, []byte("ab")) {
		t.Fatalf("first chunk got %q, want %q", got, "ab")
	}
	if got := f.Filter([]byte("c")); len(got) != 0 {
		t.Fatalf("second chunk got %q, want empty (DA1 completed)", got)
	}
}

func TestFilterSplitKeepsRealSeq(t *testing.T) {
	var f queryFilter
	if got := f.Filter([]byte("\x1b[")); len(got) != 0 {
		t.Fatalf("partial got %q, want empty (held)", got)
	}
	if got := f.Filter([]byte("H")); !bytes.Equal(got, []byte("\x1b[H")) {
		t.Fatalf("completion got %q, want ESC[H", got)
	}
}

func TestFilterLoneEscapeThenText(t *testing.T) {
	var f queryFilter
	// A trailing lone ESC is held, then flushed as a non-query when next bytes arrive.
	if got := f.Filter([]byte("x\x1b")); !bytes.Equal(got, []byte("x")) {
		t.Fatalf("got %q", got)
	}
	if got := f.Filter([]byte("Z")); !bytes.Equal(got, []byte("\x1bZ")) {
		t.Fatalf("got %q, want ESC Z", got)
	}
}
