// dispatch.go: the command router. Each verb is described by a Command spec
// (handler + how many args it needs + whether it may run before CONNECT). The
// dispatcher enforces those rules in ONE place - flood tracking, logging, the
// authenticated-client check and the arg-count check all live here, so the
// handlers stay thin and never repeat them.
package server

import (
	"strings"
	"time"

	"the-answer-protocol/internal/game"
	"the-answer-protocol/internal/protocol"
)

// HandlerFunc handles one command. The player p is resolved once by dispatch and
// passed in (nil only for CONNECT, which has no player yet). A handler may assume
// the client is authenticated and that args has at least Command.MinArgs entries.
type HandlerFunc func(h *Hub, c *Client, p *game.Player, args []string) string

// Command describes one verb: its handler plus the preconditions dispatch checks
// before calling it.
type Command struct {
	Handler HandlerFunc
	MinArgs int    // minimum args; fewer -> ERR 400 with Usage
	Anon    bool   // may run before CONNECT (only CONNECT itself)
	Usage   string // error token when MinArgs isn't met, e.g. "MISSING_ITEM"
}

// Command-flood detection: more than floodLimit commands from one client within
// floodWindow is logged as possible abuse (we monitor, not block).
const (
	floodWindow = time.Second
	floodLimit  = 20
)

// commands is the verb -> spec table. Adding a command = one entry here plus the
// handler. Built once, not per command.
var commands = map[string]Command{
	"CONNECT":   {handleConnect, 1, true, "MISSING_USERNAME"},
	"LOOK":      {handleLook, 0, false, ""},
	"MOVE":      {handleMove, 1, false, "MISSING_DIRECTION"},
	"CHAT":      {handleChat, 2, false, "BAD_CHAT"},
	"WHO":       {handleWho, 0, false, ""},
	"QUIT":      {handleQuit, 0, false, ""},
	"TAKE":      {handleTake, 1, false, "MISSING_ITEM"},
	"DROP":      {handleDrop, 1, false, "MISSING_ITEM"},
	"INVENTORY": {handleInventory, 0, false, ""},
	"TALK":      {handleTalk, 1, false, "MISSING_NPC"},
	"ATTACK":    {handleAttack, 1, false, "MISSING_NPC"},
	"STATUS":    {handleStatus, 0, false, ""},
	"GROUP":     {handleGroup, 1, false, "MISSING_SUBCOMMAND"},
	"QUEST":     {handleQuest, 1, false, "MISSING_NPC"},
	"QUESTS":    {handleQuests, 0, false, ""},
}

// dispatch routes cmd to its handler and sends the response to c. Every command
// and response is logged; flooding is tracked for abuse monitoring.
func (h *Hub) dispatch(c *Client, cmd protocol.Command) {
	if c.cmdRate.exceeded(time.Now(), floodLimit, floodWindow) {
		h.log.Warn("possible command flood", "user", c.username, "addr", c.addr, "count", c.cmdRate.count, "window", floodWindow.String())
	}
	h.log.Info("command", "user", c.username, "verb", cmd.Verb, "args", cmd.Args)

	spec, ok := commands[cmd.Verb]
	if !ok {
		h.log.Warn("unknown command", "user", c.username, "verb", cmd.Verb)
		c.safeSend(protocol.BadRequest("UNKNOWN_COMMAND").Wire())
		return
	}
	// every command except CONNECT needs an authenticated client
	if !spec.Anon && c.username == "" {
		h.log.Warn("response", "verb", cmd.Verb, "resp", "ERR 400 NOT_CONNECTED")
		c.safeSend(protocol.BadRequest("NOT_CONNECTED").Wire())
		return
	}
	if len(cmd.Args) < spec.MinArgs {
		c.safeSend(protocol.BadRequest(spec.Usage).Wire())
		return
	}

	// resolve the player once and hand it to the handler (nil for CONNECT)
	var p *game.Player
	if c.username != "" {
		p = h.world.GetPlayer(c.username)
	}

	resp := spec.Handler(h, c, p, cmd.Args)
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
