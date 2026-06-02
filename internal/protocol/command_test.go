/*
Package protocol — Owner: BOTH (Rui + Alexandre).

File command_test.go: tests for Parse() and for the response/event formatters.

This file is a reference for the table-driven test style — the idiomatic way
to test in Go. Run it with:

	go test ./internal/protocol

The same pattern (a slice of cases + a loop with t.Run) works for the world
loader, the combat formula, etc. Reuse it across the project.
*/
package protocol

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	// Each case describes an input and the expected result. Adding a new case
	// is just adding one line — that is what makes this style practical.
	cases := []struct {
		name     string
		line     string
		wantVerb string
		wantArgs []string
		wantErr  bool
	}{
		{"simple command", "LOOK", "LOOK", []string{}, false},
		{"lowercase verb becomes uppercase", "look", "LOOK", []string{}, false},
		{"single argument", "MOVE north", "MOVE", []string{"north"}, false},
		{"free text is split into args", "CHAT GLOBAL Hello everyone", "CHAT", []string{"GLOBAL", "Hello", "everyone"}, false},
		{"multi-word item name", "TAKE Loaf of Bread", "TAKE", []string{"Loaf", "of", "Bread"}, false},
		{"extra spaces are ignored", "   MOVE    north   ", "MOVE", []string{"north"}, false},
		{"trailing carriage return is stripped", "LOOK\r", "LOOK", []string{}, false},
		{"empty line returns error", "", "", nil, true},
		{"whitespace-only returns error", "    ", "", nil, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Parse(c.line)

			// Error case: confirm the error came back and stop here.
			if c.wantErr {
				if err == nil {
					t.Fatalf("Parse(%q): expected an error, got none", c.line)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%q): unexpected error: %v", c.line, err)
			}
			if got.Verb != c.wantVerb {
				t.Errorf("Verb = %q, want %q", got.Verb, c.wantVerb)
			}
			if !reflect.DeepEqual(got.Args, c.wantArgs) {
				t.Errorf("Args = %#v, want %#v", got.Args, c.wantArgs)
			}
		})
	}
}

func TestCommandString(t *testing.T) {
	cases := []struct {
		name string
		cmd  Command
		want string
	}{
		{"no arguments", Command{Verb: "LOOK"}, "LOOK"},
		{"with argument", Command{Verb: "MOVE", Args: []string{"north"}}, "MOVE north"},
		{"multiple arguments", Command{Verb: "CHAT", Args: []string{"ROOM", "hi"}}, "CHAT ROOM hi"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.cmd.String(); got != c.want {
				t.Errorf("String() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestResponses(t *testing.T) {
	// Confirm each response ends in "\n" and matches the RFC format.
	if got := OK(""); got != "OK\n" {
		t.Errorf(`OK("") = %q, want "OK\n"`, got)
	}
	if got := OK("room=loc.bakery"); got != "OK room=loc.bakery\n" {
		t.Errorf("OK(data) = %q", got)
	}
	if got := OKf("room=%s", "loc.bakery"); got != "OK room=loc.bakery\n" {
		t.Errorf("OKf = %q", got)
	}
	if got := Errf(ErrCodeNoExit, MsgNoExit); got != "ERR 301 NO_EXIT\n" {
		t.Errorf("Errf = %q, want \"ERR 301 NO_EXIT\\n\"", got)
	}

	// OKJson must produce valid JSON inside the "OK ...\n" envelope.
	got := OKJson(WhoResponse{Room: []string{"alice", "bob"}, Server: 5})
	want := `OK {"room":["alice","bob"],"server":5}` + "\n"
	if got != want {
		t.Errorf("OKJson:\n got  %q\n want %q", got, want)
	}
}

func TestEvents(t *testing.T) {
	cases := []struct {
		got  string
		want string
	}{
		{RoomPresenceEnter("alice"), "EVT ROOM PRESENCE ENTER alice\n"},
		{RoomPresenceLeave("alice"), "EVT ROOM PRESENCE LEAVE alice\n"},
		{RoomChat("alice", "hi there"), "EVT ROOM CHAT alice hi there\n"},
		{GlobalChat("bob", "hello"), "EVT GLOBAL CHAT bob hello\n"},
		{GroupInvite("alice"), "EVT GROUP INVITE alice\n"},
		{GroupJoin("bob"), "EVT GROUP JOIN bob\n"},
		{GroupLeave("bob"), "EVT GROUP LEAVE bob\n"},
		{GroupChat("alice", "team msg"), "EVT GROUP CHAT alice team msg\n"},
		{StatsPlayers(5), "EVT STATS players=5\n"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("event = %q, want %q", c.got, c.want)
		}
	}
}
