/*
Package protocol — Owner: BOTH (Rui + Alexandre).

This package is the central CONTRACT of TAP — it defines every message
type, response code, and event that flows between the server and clients.
Write this TOGETHER and lock it down BEFORE any other package is built.
Any change here affects both sides — always communicate.

File command.go: the Command struct and parsing of client lines.
Wire format: "<VERB> [arg1] [arg2...]\n"  (verb is case-insensitive)
*/
package protocol

import "errors"

// Command holds a parsed TAP command sent by a client.
type Command struct {
	Verb string   // uppercase verb, e.g. "MOVE", "LOOK", "CHAT"
	Args []string // zero or more arguments following the verb
	Raw  string   // original unmodified line from the wire (no trailing \n)
}

// ErrEmptyCommand is returned when Parse receives an empty or blank line.
var ErrEmptyCommand = errors.New("empty command")

// Parse parses a raw line (without the trailing \n) into a Command.
// The verb is normalised to uppercase. Returns ErrEmptyCommand for blank lines.
func Parse(line string) (Command, error) {
	return Command{}, nil // TODO: implement
}

// String returns the canonical wire representation: "VERB arg1 arg2".
func (c Command) String() string {
	return "" // TODO: implement
}
