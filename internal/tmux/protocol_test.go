package tmux

import (
	"bytes"
	"testing"
)

func TestTrimTrailingBlankLines(t *testing.T) {
	cases := map[string]string{
		"a\r\nb\r\n\r\n\r\n":                  "a\r\nb",                       // empty trailers dropped
		"a\r\n   \r\n":                        "a",                            // space-only trailer dropped
		"a\r\n\x1b[34m\x1b[39m\r\n":           "a",                            // escape-only trailer dropped
		"prompt % \r\n   \r\n   \r\n":         "prompt % ",                    // keep content, drop blank tail
		"x\r\ny":                              "x\r\ny",                       // no trailing blanks
		"":                                    "",                             // empty stays empty
		"\x1b[34m/tmp\x1b[39m  % \r\n\r\n\r\n": "\x1b[34m/tmp\x1b[39m  % ",     // real prompt line kept
	}
	for in, want := range cases {
		if got := trimTrailingBlankLines([]byte(in)); string(got) != want {
			t.Fatalf("trimTrailingBlankLines(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestUnescapeOutput(t *testing.T) {
	cases := map[string]string{
		`echo MARK\015\012`: "echo MARK\r\n",
		`a\\b`:              `a\b`,
		`\033[32mhi\033[m`:  "\x1b[32mhi\x1b[m",
		`plain`:             "plain",
	}
	for in, want := range cases {
		if got := unescapeOutput([]byte(in)); !bytes.Equal(got, []byte(want)) {
			t.Fatalf("unescapeOutput(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestClassifyLine(t *testing.T) {
	pane, data, ok := parseOutput([]byte("%output %3 hello\\015"))
	if !ok || pane != "%3" || string(data) != "hello\r" {
		t.Fatalf("parseOutput = %q,%q,%v", pane, data, ok)
	}
	if _, _, ok := parseOutput([]byte("%layout-change @3 x")); ok {
		t.Fatalf("%s", "non-%output should not parse as output")
	}
	if k, ok := beginEnd([]byte("%begin 1 2 1")); !ok || k != lineBegin {
		t.Fatalf("begin classify = %v,%v", k, ok)
	}
	if k, ok := beginEnd([]byte("%end 1 2 1")); !ok || k != lineEnd {
		t.Fatalf("end classify = %v,%v", k, ok)
	}
	if k, ok := beginEnd([]byte("%error 1 2 1")); !ok || k != lineEnd {
		t.Fatalf("error should classify as end, got %v,%v", k, ok)
	}
	if _, ok := beginEnd([]byte("%output %1 x")); ok {
		t.Fatalf("%s", "%output should not be begin/end")
	}
}

func TestStripDCS(t *testing.T) {
	if got := stripDCSPrefix([]byte("\x1bP1000p%begin 1 2 0")); string(got) != "%begin 1 2 0" {
		t.Fatalf("stripDCSPrefix = %q", got)
	}
	if got := stripDCSPrefix([]byte("%output %1 x")); string(got) != "%output %1 x" {
		t.Fatalf("no-DCS line altered: %q", got)
	}
}

func TestHexKeys(t *testing.T) {
	if got := hexKeys([]byte("A\n")); got != "41 0a" {
		t.Fatalf("hexKeys = %q, want '41 0a'", got)
	}
}
