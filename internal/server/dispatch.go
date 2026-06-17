// dispatch.go: the command router. It logs each command, enforces that the
// client is connected (except for CONNECT), runs the handler, and logs/sends
// the response. Handlers live in the handlers_*.go files and run inside the Hub
// goroutine, so they touch h.world directly without locks.
package server

import (
	"strings"
	"time"

	"the-answer-protocol/internal/protocol"
)

// HandlerFunc is the signature for every command handler. Returns the response
// line to send back (empty string = send nothing). A handler may assume the
// client is connected — dispatch guarantees it (except handleConnect).
type HandlerFunc func(h *Hub, c *Client, args []string) string

// errBadRequest covers input the RFC doesn't define (unknown verb, not
// connected, missing args). Documented in the README as our extension.
const errBadRequest = 400

// Flood detection: more than floodLimit commands from one client within
// floodWindow is logged as a possible abuse pattern (we monitor, not block).
const (
	floodWindow = time.Second
	floodLimit  = 20
)

// commandHandlers maps each verb to its handler. Built once, not per command.
var commandHandlers = map[string]HandlerFunc{
	"CONNECT":   handleConnect,
	"LOOK":      handleLook,
	"MOVE":      handleMove,
	"CHAT":      handleChat,
	"WHO":       handleWho,
	"QUIT":      handleQuit,
	"TAKE":      handleTake,
	"DROP":      handleDrop,
	"INVENTORY": handleInventory,
	"TALK":      handleTalk,
	"ATTACK":    handleAttack,
	"STATUS":    handleStatus,
	"GROUP":     handleGroup,
	"QUEST":     handleQuest,
	"QUESTS":    handleQuests,
}

// dispatch routes cmd to its handler and sends the response to c. Every command
// and response is logged; flooding is tracked for abuse monitoring.
func (h *Hub) dispatch(c *Client, cmd protocol.Command) {
	h.trackFlood(c)
	h.log.Info("command", "user", c.username, "verb", cmd.Verb, "args", cmd.Args)

	handler, ok := commandHandlers[cmd.Verb]
	if !ok {
		h.log.Warn("unknown command", "user", c.username, "verb", cmd.Verb)
		c.safeSend(protocol.Errf(errBadRequest, "UNKNOWN_COMMAND"))
		return
	}
	// every command except CONNECT needs an authenticated client
	if cmd.Verb != "CONNECT" && c.username == "" {
		h.log.Warn("response", "verb", cmd.Verb, "resp", "ERR 400 NOT_CONNECTED")
		c.safeSend(protocol.Errf(errBadRequest, "NOT_CONNECTED"))
		return
	}

	resp := handler(h, c, cmd.Args)
	if resp == "" {
		return
	}
	c.safeSend(resp)
	// log the outcome; errors at WARN so they stand out
	if strings.HasPrefix(resp, "ERR") {
		h.log.Warn("response", "user", c.username, "verb", cmd.Verb, "resp", strings.TrimRight(resp, "\n"))
	} else {
		h.log.Info("response", "user", c.username, "verb", cmd.Verb, "resp", strings.TrimRight(resp, "\n"))
	}
}

// trackFlood counts a client's commands per time window and logs a WARN when it
// crosses the limit. Runs in the Hub goroutine, so the counter needs no lock.
func (h *Hub) trackFlood(c *Client) {
	now := time.Now()
	if now.Sub(c.windowStart) > floodWindow {
		c.windowStart = now
		c.cmdCount = 0
	}
	c.cmdCount++
	if c.cmdCount == floodLimit+1 {
		h.log.Warn("possible command flood", "user", c.username, "addr", c.addr, "count", c.cmdCount, "window", floodWindow.String())
	}
}

// sanitize drops control characters from user text so a client can't inject
// control codes into the events broadcast to other players.
func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
}
