package protocol

import (
	"strings"
	"testing"
)

// FuzzParse throws arbitrary lines at Parse to prove two things: it never panics
// on hostile input, and any line it accepts yields a non-empty, upper-cased verb.
// The seed corpus also runs under plain `go test`. Exhaustive fuzzing:
//
//	go test -fuzz=FuzzParse ./internal/protocol
func FuzzParse(f *testing.F) {
	for _, seed := range []string{
		"CONNECT alice", "MOVE north", "CHAT GLOBAL hi there",
		"TAKE Loaf of Bread", "", "   ", "\r\n", "lowercase verb",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, line string) {
		cmd, err := Parse(line)
		if err != nil {
			return // rejecting input is fine, it just must not panic
		}
		if cmd.Verb == "" {
			t.Fatalf("Parse(%q) succeeded but Verb is empty", line)
		}
		if cmd.Verb != strings.ToUpper(cmd.Verb) {
			t.Fatalf("Parse(%q) Verb %q is not upper-cased", line, cmd.Verb)
		}
	})
}
