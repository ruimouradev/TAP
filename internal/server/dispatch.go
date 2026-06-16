// dispatch.go: the command router. Maps each verb to a handler; handlers run
// inside the Hub goroutine, so they touch h.world directly without locks.
package server

import (
	"strings"

	"the-answer-protocol/internal/game"
	"the-answer-protocol/internal/protocol"
)

// HandlerFunc is the signature for every command handler. Returns the response
// line to send back (empty string = send nothing).
type HandlerFunc func(h *Hub, c *Client, args []string) string

// errBadRequest covers input the RFC doesn't define (unknown verb, not
// connected, missing args). Documented in the README as our extension.
const errBadRequest = 400

// dispatch routes cmd to its handler and sends the response to c.
func (h *Hub) dispatch(c *Client, cmd protocol.Command) {
	handlers := map[string]HandlerFunc{
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
	}
	handler, ok := handlers[cmd.Verb]
	if !ok {
		c.safeSend(protocol.Errf(errBadRequest, "UNKNOWN_COMMAND"))
		return
	}
	if resp := handler(h, c, cmd.Args); resp != "" {
		c.safeSend(resp)
	}
}

// handleConnect: CONNECT <username>
func handleConnect(h *Hub, c *Client, args []string) string {
	if c.username != "" {
		return protocol.Errf(errBadRequest, "ALREADY_CONNECTED")
	}
	if len(args) < 1 {
		return protocol.Errf(errBadRequest, "MISSING_USERNAME")
	}
	name := args[0]
	if _, err := h.world.AddPlayer(name); err != nil {
		return protocol.Errf(protocol.ErrCodeNameInUse, protocol.MsgNameInUse)
	}
	c.username = name
	p := h.world.GetPlayer(name)
	h.broadcast(p.CurrentRoom, protocol.RoomPresenceEnter(name), c)
	h.updatePlayerCount()
	return protocol.OK("connected")
}

// handleQuit: QUIT — the client closes the connection on "OK bye"; the server's
// readPump then sees the close and unregisters. The server never closes the
// socket itself.
func handleQuit(h *Hub, c *Client, args []string) string {
	return protocol.OK("bye")
}

// handleLook: LOOK — current room state as JSON.
func handleLook(h *Hub, c *Client, args []string) string {
	if c.username == "" {
		return protocol.Errf(errBadRequest, "NOT_CONNECTED")
	}
	p := h.world.GetPlayer(c.username)
	return protocol.OKJson(buildLook(h, h.world.GetRoom(p.CurrentRoom)))
}

// handleMove: MOVE <direction>
func handleMove(h *Hub, c *Client, args []string) string {
	if c.username == "" {
		return protocol.Errf(errBadRequest, "NOT_CONNECTED")
	}
	if len(args) < 1 {
		return protocol.Errf(protocol.ErrCodeNoExit, protocol.MsgNoExit)
	}
	p := h.world.GetPlayer(c.username)
	from := p.CurrentRoom
	dest, err := h.world.MovePlayer(p, args[0])
	if err != nil {
		return protocol.Errf(protocol.ErrCodeNoExit, protocol.MsgNoExit)
	}
	h.broadcast(from, protocol.RoomPresenceLeave(c.username), c)
	h.broadcast(dest.ID, protocol.RoomPresenceEnter(c.username), c)
	return protocol.OKf("room=%s", dest.ID)
}

// handleWho: WHO — players in this room + total online, as JSON.
func handleWho(h *Hub, c *Client, args []string) string {
	if c.username == "" {
		return protocol.Errf(errBadRequest, "NOT_CONNECTED")
	}
	p := h.world.GetPlayer(c.username)
	return protocol.OKJson(protocol.WhoResponse{
		Room:   h.world.PlayersInRoom(p.CurrentRoom),
		Server: h.world.TotalPlayers(),
	})
}

// handleChat: CHAT <GLOBAL|ROOM|GROUP> <message>
func handleChat(h *Hub, c *Client, args []string) string {
	if c.username == "" {
		return protocol.Errf(errBadRequest, "NOT_CONNECTED")
	}
	if len(args) < 2 {
		return protocol.Errf(errBadRequest, "BAD_CHAT")
	}
	scope := strings.ToUpper(args[0])
	msg := strings.Join(args[1:], " ")
	switch scope {
	case "GLOBAL":
		h.broadcastAll(protocol.GlobalChat(c.username, msg))
	case "ROOM":
		p := h.world.GetPlayer(c.username)
		h.broadcast(p.CurrentRoom, protocol.RoomChat(c.username, msg), nil)
	case "GROUP":
		// group chat needs a group system, which isn't implemented yet
		return protocol.Errf(protocol.ErrCodeNotInGroup, protocol.MsgNotInGroup)
	default:
		return protocol.Errf(errBadRequest, "BAD_SCOPE")
	}
	return protocol.OK("")
}

// handleTake: TAKE <item> — move an item from the room floor to the inventory.
// The identifier may be an ID or a multi-word display name, so we join the args.
func handleTake(h *Hub, c *Client, args []string) string {
	if c.username == "" {
		return protocol.Errf(errBadRequest, "NOT_CONNECTED")
	}
	if len(args) < 1 {
		return protocol.Errf(errBadRequest, "MISSING_ITEM")
	}
	p := h.world.GetPlayer(c.username)
	room := h.world.GetRoom(p.CurrentRoom)
	it, err := game.TakeItem(room, p, strings.Join(args, " "))
	if err != nil {
		return protocol.Errf(protocol.ErrCodeItemNotFound, protocol.MsgItemNotFound)
	}
	return protocol.OKf("took %s", it.ID)
}

// handleDrop: DROP <item> — move an item from the inventory back to the floor.
func handleDrop(h *Hub, c *Client, args []string) string {
	if c.username == "" {
		return protocol.Errf(errBadRequest, "NOT_CONNECTED")
	}
	if len(args) < 1 {
		return protocol.Errf(errBadRequest, "MISSING_ITEM")
	}
	p := h.world.GetPlayer(c.username)
	room := h.world.GetRoom(p.CurrentRoom)
	it, err := game.DropItem(p, room, strings.Join(args, " "))
	if err != nil {
		return protocol.Errf(protocol.ErrCodeItemNotFound, protocol.MsgItemNotInInv)
	}
	return protocol.OKf("dropped %s", it.ID)
}

// handleInventory: INVENTORY — the player's items as a JSON array.
func handleInventory(h *Hub, c *Client, args []string) string {
	if c.username == "" {
		return protocol.Errf(errBadRequest, "NOT_CONNECTED")
	}
	p := h.world.GetPlayer(c.username)
	items := make([]protocol.InventoryItem, 0, len(p.Inventory))
	for _, it := range p.Inventory {
		items = append(items, protocol.InventoryItem{ID: it.ID, Name: it.Name})
	}
	return protocol.OKJson(items)
}

// handleTalk: TALK <npc> — return the NPC's dialogue (matched by ID or name).
func handleTalk(h *Hub, c *Client, args []string) string {
	if c.username == "" {
		return protocol.Errf(errBadRequest, "NOT_CONNECTED")
	}
	if len(args) < 1 {
		return protocol.Errf(errBadRequest, "MISSING_NPC")
	}
	p := h.world.GetPlayer(c.username)
	room := h.world.GetRoom(p.CurrentRoom)
	npc, ok := h.world.NPCInRoom(room, strings.Join(args, " "))
	if !ok {
		return protocol.Errf(protocol.ErrCodeNPCNotFound, protocol.MsgNPCNotFound)
	}
	return protocol.OKJson(protocol.TalkResponse{NPC: npc.Name, Dialogue: npc.Dialogue})
}

// handleAttack: ATTACK <npc> — one round of combat against a hostile NPC.
func handleAttack(h *Hub, c *Client, args []string) string {
	if c.username == "" {
		return protocol.Errf(errBadRequest, "NOT_CONNECTED")
	}
	if len(args) < 1 {
		return protocol.Errf(errBadRequest, "MISSING_NPC")
	}
	p := h.world.GetPlayer(c.username)
	room := h.world.GetRoom(p.CurrentRoom)
	res, err := game.Attack(p, room, strings.Join(args, " "))
	if err != nil {
		if err == game.ErrNPCNotHostile {
			return protocol.Errf(protocol.ErrCodeNPCNotHostile, protocol.MsgNPCNotHostile)
		}
		return protocol.Errf(protocol.ErrCodeNPCNotFound, protocol.MsgNPCNotFound)
	}
	// on defeat the player respawns at the start room; tell both rooms
	if res.PlayerDied {
		from := p.CurrentRoom
		game.RespawnPlayer(h.world, p)
		if from != p.CurrentRoom {
			h.broadcast(from, protocol.RoomPresenceLeave(c.username), c)
			h.broadcast(p.CurrentRoom, protocol.RoomPresenceEnter(c.username), c)
		}
	}
	return protocol.OKJson(protocol.AttackResponse{
		Damage:     res.Damage,
		Counter:    res.CounterDmg,
		AttackerHP: res.AttackerHP,
		TargetHP:   res.TargetHP,
		Status:     res.Status,
	})
}

// handleStatus: STATUS — the player's HP and condition.
func handleStatus(h *Hub, c *Client, args []string) string {
	if c.username == "" {
		return protocol.Errf(errBadRequest, "NOT_CONNECTED")
	}
	p := h.world.GetPlayer(c.username)
	return protocol.OKJson(protocol.StatusResponse{
		HP:     p.HP,
		MaxHP:  p.MaxHP,
		Status: healthLabel(p),
	})
}

// healthLabel maps a player's HP to the STATUS condition string.
func healthLabel(p *game.Player) string {
	switch {
	case p.HP <= 0:
		return "dead"
	case p.HP*2 < p.MaxHP:
		return "wounded"
	default:
		return "healthy"
	}
}

// buildLook converts a room into the LOOK JSON payload.
func buildLook(h *Hub, room *game.Room) protocol.LookResponse {
	items := make([]protocol.LookItem, 0, len(room.Items))
	for _, it := range room.Items {
		items = append(items, protocol.LookItem{ID: it.ID, Name: it.Name})
	}
	npcs := make([]protocol.LookNPC, 0, len(room.NPCs))
	for _, n := range room.NPCs {
		npcs = append(npcs, protocol.LookNPC{ID: n.ID, Name: n.Name, Hostile: n.Hostile})
	}
	return protocol.LookResponse{
		Room: protocol.LookRoom{
			ID:          room.ID,
			Name:        room.Name,
			Description: room.Description,
			Exits:       room.Exits,
		},
		Players: h.world.PlayersInRoom(room.ID),
		Items:   items,
		NPCs:    npcs,
	}
}
