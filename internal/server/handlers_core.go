// handlers_core.go: the core verbs - connection, movement, presence and chat.
package server

import (
	"strings"

	"the-answer-protocol/internal/game"
	"the-answer-protocol/internal/protocol"
)

// handleConnect: CONNECT <username>. p is nil here because the player is created now.
func handleConnect(h *Hub, c *Client, p *game.Player, args []string) string {
	if c.username != "" {
		return protocol.BadRequest("ALREADY_CONNECTED").Wire()
	}
	name := args[0]
	if _, err := h.world.AddPlayer(name); err != nil {
		return protocol.ErrNameInUse.Wire()
	}
	c.username = name
	np := h.world.GetPlayer(name)
	h.broadcast(np.CurrentRoom, protocol.RoomPresenceEnter(name), c)
	h.updatePlayerCount()
	return protocol.OK("connected")
}

// handleQuit: QUIT. The client closes the connection when it gets "OK bye".
// The server's readPump then sees the close and unregisters. The server never
// closes the socket itself.
func handleQuit(h *Hub, c *Client, p *game.Player, args []string) string {
	return protocol.OK("bye")
}

// handleLook: LOOK. Returns the current room state as JSON.
func handleLook(h *Hub, c *Client, p *game.Player, args []string) string {
	return protocol.OKJson(buildLook(h, h.world.GetRoom(p.CurrentRoom)))
}

// handleMove: MOVE <direction>
func handleMove(h *Hub, c *Client, p *game.Player, args []string) string {
	from := p.CurrentRoom
	// exits are keyed lowercase, so accept MOVE NORTH as well as MOVE north
	dest, err := h.world.MovePlayer(p, strings.ToLower(args[0]))
	if err != nil {
		return protocol.ErrNoExit.Wire()
	}
	h.broadcast(from, protocol.RoomPresenceLeave(c.username), c)
	h.broadcast(dest.ID, protocol.RoomPresenceEnter(c.username), c)
	return protocol.OKf("room=%s", dest.ID)
}

// handleWho: WHO. Players in this room plus the total online, as JSON.
func handleWho(h *Hub, c *Client, p *game.Player, args []string) string {
	return protocol.OKJson(protocol.WhoResponse{
		Room:   h.world.PlayersInRoom(p.CurrentRoom),
		Server: h.world.TotalPlayers(),
	})
}

// handleChat: CHAT <GLOBAL|ROOM|GROUP> <message>
func handleChat(h *Hub, c *Client, p *game.Player, args []string) string {
	scope := strings.ToUpper(args[0])
	msg := sanitize(strings.Join(args[1:], " ")) // drop control chars before broadcasting
	switch scope {
	case "GLOBAL":
		h.broadcastAll(protocol.GlobalChat(c.username, msg))
	case "ROOM":
		h.broadcast(p.CurrentRoom, protocol.RoomChat(c.username, msg), nil)
	case "GROUP":
		grp := h.groups[p.GroupID]
		if grp == nil {
			return protocol.ErrNotInGroup.Wire()
		}
		h.broadcastGroup(grp, protocol.GroupChat(c.username, msg), nil)
	default:
		return protocol.BadRequest("BAD_SCOPE").Wire()
	}
	return protocol.OK("")
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
