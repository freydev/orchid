package tmux

import (
	"bytes"
	"fmt"
	"strings"
)

type lineKind int

const (
	lineBegin lineKind = iota
	lineEnd
)

// stripDCSPrefix removes a leading control-mode DCS marker (ESC P ... p) that
// tmux emits at the start of the stream, leaving the protocol line intact.
func stripDCSPrefix(line []byte) []byte {
	if len(line) >= 2 && line[0] == 0x1b && line[1] == 'P' {
		if i := bytes.IndexByte(line, 'p'); i >= 0 {
			return line[i+1:]
		}
	}
	return line
}

// parseOutput returns the pane id and decoded bytes for a "%output %<pane> <data>"
// line. ok is false for any other line.
func parseOutput(line []byte) (pane string, data []byte, ok bool) {
	const pfx = "%output "
	if !bytes.HasPrefix(line, []byte(pfx)) {
		return "", nil, false
	}
	rest := line[len(pfx):]
	sp := bytes.IndexByte(rest, ' ')
	if sp < 0 {
		return string(rest), nil, true
	}
	return string(rest[:sp]), unescapeOutput(rest[sp+1:]), true
}

// beginEnd classifies "%begin"/"%end"/"%error" reply-block delimiter lines.
func beginEnd(line []byte) (lineKind, bool) {
	switch {
	case bytes.HasPrefix(line, []byte("%begin")):
		return lineBegin, true
	case bytes.HasPrefix(line, []byte("%end")), bytes.HasPrefix(line, []byte("%error")):
		return lineEnd, true
	}
	return 0, false
}

// unescapeOutput decodes tmux %output escaping: \NNN octal and \\ backslash.
func unescapeOutput(in []byte) []byte {
	out := make([]byte, 0, len(in))
	for i := 0; i < len(in); i++ {
		if in[i] != '\\' || i+1 >= len(in) {
			out = append(out, in[i])
			continue
		}
		if in[i+1] == '\\' {
			out = append(out, '\\')
			i++
			continue
		}
		if isOctal(in[i+1]) {
			val := 0
			j := i + 1
			for j < len(in) && j <= i+3 && isOctal(in[j]) {
				val = val*8 + int(in[j]-'0')
				j++
			}
			out = append(out, byte(val))
			i = j - 1
			continue
		}
		out = append(out, in[i])
	}
	return out
}

func isOctal(b byte) bool { return b >= '0' && b <= '7' }

// trimTrailingBlankLines drops trailing CRLF-separated lines that render as
// blank (only spaces/tabs and/or ANSI escape sequences). tmux capture-pane pads
// output to the full pane height with blank rows; seeding those into xterm
// pushes the cursor to the bottom and leaves empty space above the content.
func trimTrailingBlankLines(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	lines := bytes.Split(b, []byte("\r\n"))
	n := len(lines)
	for n > 0 && isBlankLine(lines[n-1]) {
		n--
	}
	return bytes.Join(lines[:n], []byte("\r\n"))
}

// isBlankLine reports whether a line has no visible content after removing CSI
// escape sequences and whitespace.
func isBlankLine(line []byte) bool {
	for i := 0; i < len(line); i++ {
		c := line[i]
		if c == 0x1b { // ESC: skip a CSI sequence (ESC [ ... final 0x40-0x7e)
			i++
			if i < len(line) && line[i] == '[' {
				i++
				for i < len(line) && !(line[i] >= 0x40 && line[i] <= 0x7e) {
					i++
				}
			}
			continue
		}
		if c != ' ' && c != '\t' && c != '\r' {
			return false
		}
	}
	return true
}

// hexKeys formats bytes as space-separated lowercase hex pairs for send-keys -H.
func hexKeys(b []byte) string {
	parts := make([]string, len(b))
	for i, c := range b {
		parts[i] = fmt.Sprintf("%02x", c)
	}
	return strings.Join(parts, " ")
}
