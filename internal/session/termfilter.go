package session

// queryFilter strips terminal capability QUERY sequences that tmux emits to its
// client on attach: DA1 (ESC[c), DA2 (ESC[>c), XTVERSION (ESC[>q), the color
// scheme DSR (ESC[?996n), and OSC 10/11 foreground/background color queries.
//
// orchid's per-session PTY is a pass-through tmux-attach client, not a terminal
// emulator, so it never answers these. Left in the stream, they reach the
// browser's xterm.js, which DOES answer them — and the answers (e.g. "\e[?1;2c")
// arrive at tmux after attach negotiation has finished, so tmux forwards them to
// the shell as keystrokes, which echoes them as visible garbage like
// "1;2c0;276;0c". Stripping the queries means xterm never answers; tmux simply
// falls back to its terminfo capabilities, which is what we want anyway.
//
// A queryFilter is used by a single readLoop goroutine, so it needs no locking.
type queryFilter struct {
	pending []byte // a partial escape sequence held across Filter calls
}

// Filter returns in with capability-query sequences removed. Incomplete trailing
// sequences are buffered and resolved on the next call.
func (f *queryFilter) Filter(in []byte) []byte {
	data := in
	if len(f.pending) > 0 {
		data = append(f.pending, in...)
		f.pending = nil
	}
	out := make([]byte, 0, len(data))
	i := 0
	for i < len(data) {
		if data[i] != 0x1b {
			out = append(out, data[i])
			i++
			continue
		}
		consumed, strip, incomplete := matchSeq(data[i:])
		if incomplete {
			f.pending = append(f.pending, data[i:]...)
			return out
		}
		if !strip {
			out = append(out, data[i:i+consumed]...)
		}
		i += consumed
	}
	return out
}

// matchSeq inspects an escape sequence beginning at s[0] (== 0x1b). It returns
// how many bytes the sequence occupies, whether it should be stripped, and
// whether the buffer ends before the sequence is complete.
func matchSeq(s []byte) (consumed int, strip, incomplete bool) {
	if len(s) < 2 {
		return 0, false, true
	}
	switch s[1] {
	case '[':
		return matchCSI(s)
	case ']':
		return matchOSC(s)
	default:
		// Not a sequence we filter; emit the ESC alone and resume scanning.
		return 1, false, false
	}
}

// matchCSI parses ESC [ <prefix?> <params> <final>.
func matchCSI(s []byte) (consumed int, strip, incomplete bool) {
	j := 2
	var prefix byte
	if j < len(s) && (s[j] == '?' || s[j] == '>' || s[j] == '=') {
		prefix = s[j]
		j++
	}
	pstart := j
	for j < len(s) && ((s[j] >= '0' && s[j] <= '9') || s[j] == ';') {
		j++
	}
	if j >= len(s) {
		return 0, false, true // params not yet terminated by a final byte
	}
	final := s[j]
	params := string(s[pstart:j])
	consumed = j + 1
	switch {
	case final == 'c': // DA1 (no prefix) or DA2 (prefix '>') query
		strip = true
	case prefix == '>' && final == 'q': // XTVERSION query
		strip = true
	case prefix == '?' && final == 'n' && params == "996": // color-scheme DSR
		strip = true
	}
	return consumed, strip, false
}

// matchOSC parses ESC ] <payload> <BEL | ST> and strips only color queries
// (OSC 10/11 with a "?" payload).
func matchOSC(s []byte) (consumed int, strip, incomplete bool) {
	j := 2
	for j < len(s) {
		switch {
		case s[j] == 0x07: // BEL terminator
			return j + 1, isColorQuery(s[2:j]), false
		case s[j] == 0x1b:
			if j+1 >= len(s) {
				return 0, false, true // partial ST
			}
			if s[j+1] == '\\' { // ST terminator
				return j + 2, isColorQuery(s[2:j]), false
			}
			// Stray ESC inside OSC; treat OSC as ended here, emit verbatim.
			return j, false, false
		}
		j++
	}
	return 0, false, true // terminator not seen yet
}

func isColorQuery(payload []byte) bool {
	p := string(payload)
	return p == "10;?" || p == "11;?"
}
