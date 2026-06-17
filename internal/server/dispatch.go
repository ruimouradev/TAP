// dispatch.go: the command router. Maps each verb to a handler; handlers run
// inside the Hub goroutine, so they touch h.world directly without locks.
package server

import (
	"strings"
	"time"

	"the-answer-protocol/internal/game"
	"the-answer-protocol/internal/protocol"
)

// HandlerFunc is the signature for every command handler. Returns the response
// line to send back (empty string = send nothing).
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

// dispatch routes cmd to its handler and sends the response to c. Every command
// and response is logged; flooding is tracked for abuse monitoring.
func (h *Hub) dispatch(c *Client, cmd protocol.Command) {
	h.trackFlood(c)
	h.log.Info("command", "user", c.username, "verb", cmd.Verb, "args", cmd.Args)

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
		"GROUP":     handleGroup,
		"QUEST":     handleQuest,
		"QUESTS":    handleQuests,
	}
	handler, ok := handlers[cmd.Verb]
	if !ok {
		h.log.Warn("unknown command", "user", c.username, "verb", cmd.Verb)
		c.safeSend(protocol.Errf(errBadRequest, "UNKNOWN_COMMAND"))
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
		p := h.world.GetPlayer(c.username)
		grp := h.groups[p.GroupID]
		if grp == nil {
			return protocol.Errf(protocol.ErrCodeNotInGroup, protocol.MsgNotInGroup)
		}
		h.broadcastGroup(grp, protocol.GroupChat(c.username, msg), nil)
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
	h.log.Info("item taken", "user", c.username, "item", it.ID, "room", p.CurrentRoom)
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
	h.log.Info("item dropped", "user", c.username, "item", it.ID, "room", p.CurrentRoom)
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
	h.log.Info("combat", "user", c.username, "target", strings.Join(args, " "),
		"damage", res.Damage, "counter", res.CounterDmg, "status", res.Status)
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

// handleGroup: GROUP <CREATE|INVITE|JOIN|LEAVE> [args] — party management.
func handleGroup(h *Hub, c *Client, args []string) string {
	if c.username == "" {
		return protocol.Errf(errBadRequest, "NOT_CONNECTED")
	}
	if len(args) < 1 {
		return protocol.Errf(errBadRequest, "MISSING_SUBCOMMAND")
	}
	switch strings.ToUpper(args[0]) {
	case "CREATE":
		return handleGroupCreate(h, c, args[1:])
	case "INVITE":
		return handleGroupInvite(h, c, args[1:])
	case "JOIN":
		return handleGroupJoin(h, c, args[1:])
	case "LEAVE":
		return handleGroupLeave(h, c, args[1:])
	default:
		return protocol.Errf(errBadRequest, "BAD_SUBCOMMAND")
	}
}

// handleGroupCreate: GROUP CREATE — start a new group led by the caller.
func handleGroupCreate(h *Hub, c *Client, args []string) string {
	p := h.world.GetPlayer(c.username)
	if p.GroupID != "" {
		return protocol.Errf(protocol.ErrCodeAlreadyInGroup, protocol.MsgAlreadyInGroup)
	}
	h.groups[c.username] = &Group{
		ID:      c.username,
		Leader:  c.username,
		Members: map[string]*Client{c.username: c},
	}
	p.GroupID = c.username
	h.log.Info("group created", "user", c.username, "group", c.username)
	return protocol.OKf("group=%s", c.username)
}

// handleGroupInvite: GROUP INVITE <username> — the leader invites a player.
func handleGroupInvite(h *Hub, c *Client, args []string) string {
	p := h.world.GetPlayer(c.username)
	grp := h.groups[p.GroupID]
	if grp == nil {
		return protocol.Errf(protocol.ErrCodeNotInGroup, protocol.MsgNotInGroup)
	}
	if grp.Leader != c.username {
		return protocol.Errf(errBadRequest, "NOT_GROUP_LEADER")
	}
	if len(args) < 1 {
		return protocol.Errf(errBadRequest, "MISSING_USERNAME")
	}
	target := h.clientByUsername(args[0])
	if target == nil {
		return protocol.Errf(errBadRequest, "NO_SUCH_PLAYER")
	}
	if tp := h.world.GetPlayer(target.username); tp.GroupID != "" {
		return protocol.Errf(protocol.ErrCodeAlreadyInGroup, protocol.MsgAlreadyInGroup)
	}
	target.invitedGroup = grp.ID
	target.safeSend(protocol.GroupInvite(c.username))
	return protocol.OK("invited")
}

// handleGroupJoin: GROUP JOIN — accept a pending invite.
func handleGroupJoin(h *Hub, c *Client, args []string) string {
	p := h.world.GetPlayer(c.username)
	if p.GroupID != "" {
		return protocol.Errf(protocol.ErrCodeAlreadyInGroup, protocol.MsgAlreadyInGroup)
	}
	if c.invitedGroup == "" {
		return protocol.Errf(errBadRequest, "NO_INVITE")
	}
	grp := h.groups[c.invitedGroup]
	c.invitedGroup = ""
	if grp == nil {
		return protocol.Errf(errBadRequest, "GROUP_GONE")
	}
	grp.Members[c.username] = c
	p.GroupID = grp.ID
	h.broadcastGroup(grp, protocol.GroupJoin(c.username), c)
	h.log.Info("group joined", "user", c.username, "group", grp.ID)
	return protocol.OKf("group=%s", grp.ID)
}

// handleGroupLeave: GROUP LEAVE — leave the current group.
func handleGroupLeave(h *Hub, c *Client, args []string) string {
	p := h.world.GetPlayer(c.username)
	if p.GroupID == "" {
		return protocol.Errf(protocol.ErrCodeNotInGroup, protocol.MsgNotInGroup)
	}
	h.leaveGroup(c, p)
	h.log.Info("group left", "user", c.username)
	return protocol.OK("left")
}

// handleQuest: QUEST <npc> — request a quest from a quest-giver NPC.
func handleQuest(h *Hub, c *Client, args []string) string {
	if c.username == "" {
		return protocol.Errf(errBadRequest, "NOT_CONNECTED")
	}
	if len(args) < 1 {
		return protocol.Errf(errBadRequest, "MISSING_NPC")
	}
	p := h.world.GetPlayer(c.username)
	room := h.world.GetRoom(p.CurrentRoom)
	def, err := game.GetQuestFromNPC(h.world, p, room, strings.Join(args, " "))
	if err != nil {
		return protocol.Errf(// handleGroupJoin: GROUP JOIN — accept a pending invite.
func handleGroupJoin(h *Hub, c *Client, args []string) string {
	p := h.world.GetPlayer(c.username)
	if p.GroupID != "" {
		return protocol.Errf(protocol.ErrCodeAlreadyInGroup, protocol.MsgAlreadyInGroup)
	}
	if c.invitedGroup == "" {
		return protocol.Errf(errBadRequest, "NO_INVITE")
	}
	grp := h.groups[c.invitedGroup]
	c.invitedGroup = ""
	if grp == nil {
		return protocol.Errf(errBadRequest, "GROUP_GONE")
	}
	grp.Members[c.username] = c
	p.GroupID = grp.ID
	h.broadcastGroup(grp, protocol.GroupJoin(c.username), c)
	h.log.Info("group joined", "user", c.username, "group", grp.ID)
	return protocol.OKf("group=%s", grp.ID)
}

// handleGroupLeave: GROUP LEAVE — leave the current group.
func handleGroupLeave(h *Hub, c *Client, args []string) string {
	p := h.world.GetPlayer(c.username)
	if p.GroupID == "" {
		return protocol.Errf(protocol.ErrCodeNotInGroup, protocol.MsgNotInGroup)
	}
	h.leaveGroup(c, p)
	h.log.Info("group left", "user", c.username)
	return protocol.OK("left")
}

// handleQuest: QUEST <npc> — request a quest from a quest-giver NPC.
func handleQuest(h *Hub, c *Client, args []string) string {
	if c.username == "" {
		return protocol.Errf(errBadRequest, "NOT_CONNECTED")
	}
	if len(args) < 1 {
		return protocol.Errf(errBadRequest, "MISSING_NPC")
	}protocol.ErrCodeNoQuestAvailable, protocol.MsgNoQuestAvailable)
	}
	game.StartQuest(p, def)
	h.log.Info("quest started", "user", c.username, "quest", def.ID)
	return protocol.OKJson(protocol.QuestResponse{
		QuestID:     def.ID,
		Description: def.Description,
		Type:        def.Type,
		Target:      def.TargetID,
		Reward:      def.Reward,
	})
}

// handleQuests: QUESTS — list the player's quests. Any active quest whose
// objective is now met is completed here (and its reward granted) before listing.
func handleQuests(h *Hub, c *Client, args []string) string {
	if c.username == "" {
		return protocol.Errf(errBadRequest, "NOT_CONNECTED")
	}
	p := h.world.GetPlayer(c.username)
	entries := make([]protocol.QuestsEntry, 0, len(p.Quests))
	for _, pq := range game.ListQuests(p) {
		if pq.State == game.QuestActive && game.CheckCompletion(p, pq) {
			game.CompleteQuest(h.world, p, pq)
			h.log.Info("quest completed", "user", c.username, "quest", pq.Def.ID, "reward", pq.Def.Reward)
		}
		entries = append(entries, protocol.QuestsEntry{
			QuestID:     pq.Def.ID,
			Description: pq.Def.Description,
			State:       questStateLabel(pq.State),
		})
	}
	return protocol.OKJson(entries)
}

// questStateLabel maps the internal quest state to the wire string.
func questStateLabel(s game.QuestState) string {
	if s == game.QuestCompleted {
		return "completed"
	}
	return "active"
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
