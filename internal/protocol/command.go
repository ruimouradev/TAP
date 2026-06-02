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

import (
	"errors"
	"strings"
)

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
	// Alguns clientes (telnet, Windows) terminam linhas com "\r\n". O scanner
	// do servidor já tira o "\n", mas o "\r" pode sobrar — limpamo-lo aqui.
	clean := strings.TrimRight(line, "\r")

	// strings.Fields parte por qualquer run de espaços/tabs e ignora extremidades,
	// por isso "  MOVE   north " vira ["MOVE", "north"] sem campos vazios.
	fields := strings.Fields(clean)
	if len(fields) == 0 {
		return Command{}, ErrEmptyCommand
	}

	// O RFC diz que o verbo é case-insensitive; normalizamos para maiúsculas
	// uma única vez aqui, para o dispatcher poder comparar sem se preocupar.
	verb := strings.ToUpper(fields[0])

	// Nota de design: comandos com texto livre (CHAT GLOBAL <msg>) ficam com a
	// mensagem partida em vários Args. Quem precisa do texto inteiro volta a
	// juntá-lo com strings.Join(args, " "), ou usa o campo Raw. Mantemos o Parse
	// genérico de propósito — não tem de conhecer cada comando.
	return Command{
		Verb: verb,
		Args: fields[1:],
		Raw:  clean,
	}, nil
}

// String returns the canonical wire representation: "VERB arg1 arg2".
func (c Command) String() string {
	if len(c.Args) == 0 {
		return c.Verb
	}
	return c.Verb + " " + strings.Join(c.Args, " ")
}
