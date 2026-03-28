package vt10x

import (
	"unicode"
	"unicode/utf8"
)

type VTStrip struct {
	VT
}

func NewStrip() *VTStrip {
	t := &VTStrip{
		VT{
			dest: &State{},
			rwc:  nil,
		},
	}
	t.init()
	return t
}

// Strip returns in with all VT10x escape sequences stripped.  An error is
// also returned if one or more of the stripped escape sequences are invalid.
func (t *VTStrip) Strip(in []byte) ([]byte, error) {
	var locked bool
	defer func() {
		if locked {
			t.dest.unlock()
		}
	}()
	out := make([]byte, len(in))
	nout := 0
	s := string(in)
	for i, w := 0, 0; i < len(s); i += w {
		c, sz := utf8.DecodeRuneInString(s[i:])
		w = sz
		if c == unicode.ReplacementChar && sz == 1 {
			t.dest.logln("invalid utf8 sequence")
			break
		}
		if !locked {
			t.dest.lock()
			locked = true
		}

		// put rune for parsing and update state
		isPrintable := t.dest.put(c)
		if isPrintable {
			copy(out[nout:nout+w], in[i:i+w])
			nout += w
		}
	}
	return out[:nout], nil
}
