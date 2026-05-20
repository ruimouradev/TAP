/*
Package server — Owner: Rui.

File dispatch.go: TAP command router.
Maintains a map of verb → HandlerFunc and routes each incoming Command
to the correct handler. All handlers run inside the Hub's goroutine,
so they can read/mutate hub.world directly without any locks.
*/
package server

import "the-answer-protocol/internal/protocol"

// HandlerFunc is the signature for all command handlers.
// Runs inside the Hub's goroutine — safe to access hub.world directly.
// Returns the response string to write back to the client.
type HandlerFunc func(h *Hub, c *Client, args []string) string

// dispatch routes cmd to the registered handler and writes the response to c.
// Responds with ERR for unknown verbs.
func (h *Hub) dispatch(c *Client, cmd protocol.Command) {
	// TODO: implement — build the handlers map and look up cmd.Verb
}

// ── Connection ────────────────────────────────────────────────────────────────

// handleConnect handles: CONNECT <username>
// Registers the player in the world; validates username uniqueness.
// Sends the initial greeting and broadcasts PRESENCE ENTER to the start room.
func handleConnect(h *Hub, c *Client, args []string) string {
	return "" // TODO: implement
}

// handleQuit handles: QUIT
// Removes the player gracefully and closes the connection.
func handleQuit(h *Hub, c *Client, args []string) string {
	return "" // TODO: implement
}

// ── World exploration ─────────────────────────────────────────────────────────

// handleLook handles: LOOK
// Returns the current room state (room info, players, items, NPCs) as JSON.
func handleLook(h *Hub, c *Client, args []string) string {
	return "" // TODO: implement
}

// handleMove handles: MOVE <direction>
// Moves the player, broadcasts PRESENCE LEAVE to the old room and
// PRESENCE ENTER to the new room, returns "OK room=<new-room-id>".
func handleMove(h *Hub, c *Client, args []string) string {
	return "" // TODO: implement
}

// handleWho handles: WHO
// Returns players in current room AND total server count as JSON.
// Subject example: OK {"room":["alice","bob"],"server":5}
// Note: this deviates from the RFC ("OK players=<count>") — documented in README.
func handleWho(h *Hub, c *Client, args []string) string {
	return "" // TODO: implement
}

// ── Communication ─────────────────────────────────────────────────────────────

// handleChat handles: CHAT <scope> <message>
// scope is GLOBAL, ROOM, or GROUP.
// Broadcasts the appropriate EVT to the correct audience.
func handleChat(h *Hub, c *Client, args []string) string {
	return "" // TODO: implement
}

// ── Group management ──────────────────────────────────────────────────────────

// handleGroup handles: GROUP CREATE | INVITE <user> | JOIN <leader> | LEAVE
// Routes to the specific sub-command handler.
func handleGroup(h *Hub, c *Client, args []string) string {
	return "" // TODO: implement
}

// handleGroupCreate handles: GROUP CREATE
// Creates a new group with the current player as leader.
func handleGroupCreate(h *Hub, c *Client) string {
	return "" // TODO: implement
}

// handleGroupInvite handles: GROUP INVITE <username>
// Sends EVT GROUP INVITE to the target player.
func handleGroupInvite(h *Hub, c *Client, target string) string {
	return "" // TODO: implement
}

// handleGroupJoin handles: GROUP JOIN <leader-name>
// Adds the player to an existing group and broadcasts EVT GROUP JOIN.
func handleGroupJoin(h *Hub, c *Client, leaderName string) string {
	return "" // TODO: implement
}

// handleGroupLeave handles: GROUP LEAVE
// Removes the player from their group and broadcasts EVT GROUP LEAVE.
func handleGroupLeave(h *Hub, c *Client) string {
	return "" // TODO: implement
}

// ── Items ─────────────────────────────────────────────────────────────────────

// handleTake handles: TAKE <item-identifier>
// Removes the item from the room and adds it to the player's inventory.
func handleTake(h *Hub, c *Client, args []string) string {
	return "" // TODO: implement
}

// handleDrop handles: DROP <item-identifier>
// Removes the item from the player's inventory and places it in the room.
func handleDrop(h *Hub, c *Client, args []string) string {
	return "" // TODO: implement
}

// handleInventory handles: INVENTORY
// Returns the player's current items as a JSON array.
func handleInventory(h *Hub, c *Client, args []string) string {
	return "" // TODO: implement
}

// ── NPCs ──────────────────────────────────────────────────────────────────────

// handleTalk handles: TALK <npc-name>
// Returns NPC interaction as JSON.
// Subject example: OK {"npc":"guard","dialogue":"Stay safe, traveler."}
// Note: subject examples show JSON format; RFC shows plain string — we follow the subject.
func handleTalk(h *Hub, c *Client, args []string) string {
	return "" // TODO: implement
}

// ── Combat ────────────────────────────────────────────────────────────────────

// handleAttack handles: ATTACK <npc-name>
// Executes one round of combat and returns the result as JSON.
// Broadcasts combat events to room players.
func handleAttack(h *Hub, c *Client, args []string) string {
	return "" // TODO: implement
}

// handleStatus handles: STATUS
// Returns the player's current HP and combat status as JSON.
func handleStatus(h *Hub, c *Client, args []string) string {
	return "" // TODO: implement
}

// ── Quests ────────────────────────────────────────────────────────────────────

// handleQuest handles: QUEST <npc-name>
// Returns quest information from the NPC if a quest is available for this player.
func handleQuest(h *Hub, c *Client, args []string) string {
	return "" // TODO: implement
}

// handleQuests handles: QUESTS
// Returns a JSON list of all the player's active and completed quests.
func handleQuests(h *Hub, c *Client, args []string) string {
	return "" // TODO: implement
}
