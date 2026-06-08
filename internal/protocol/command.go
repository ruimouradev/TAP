// Package protocol is the shared wire contract between server and clients:
// commands, responses and events, all line-based ("<VERB> [args]\n").
//
// command.go: the Command type and parsing of client input.
package protocol

import (
	"errors"
	"strings"
)

// Command is a parsed client command.
type Command struct {
	Verb string   // uppercased verb
	Args []string // arguments after the verb
	Raw  string   // original line, no trailing \n
}

var ErrEmptyCommand = errors.New("empty command")

// Parse turns a wire line into a Command. Verb is uppercased; args keep case.
func Parse(line string) (Command, error) {
	clean := strings.TrimRight(line, "\r") // tolerate CRLF
	fields := strings.Fields(clean)
	if len(fields) == 0 {
		return Command{}, ErrEmptyCommand
	}
	// CHAT and friends keep their text split across Args; handlers use Raw.
	return Command{
		Verb: strings.ToUpper(fields[0]),
		Args: fields[1:],
		Raw:  clean,
	}, nil
}

// String rebuilds the "VERB arg1 arg2" form.
func (c Command) String() string {
	if len(c.Args) == 0 {
		return c.Verb
	}
	return c.Verb + " " + strings.Join(c.Args, " ")
}
