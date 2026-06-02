/*
Package server — Owner: Rui.

File hub.go: the heart of all server concurrency.
The Hub is the ONLY goroutine that reads and mutates game state.
All clients communicate with the Hub via channels; the Hub processes
one event at a time inside its select{} loop — no mutexes needed.

"Don't communicate by sharing memory; share memory by communicating."
*/
package server

import (
	"log/slog"

	"the-answer-protocol/internal/game"
	"the-answer-protocol/internal/protocol"
)

// Hub is the central actor that owns the game world.
// It processes registrations, disconnections, and commands serially.
type Hub struct {
	register   chan *Client
	unregister chan *Client
	commands   chan incomingCmd
	clients    map[*Client]bool
	groups     map[string]*Group   // groupID → Group
	world      *game.World         // only the Hub touches this
	log        *slog.Logger
}

// incomingCmd pairs a parsed command with the client who sent it.
type incomingCmd struct {
	client *Client
	cmd    protocol.Command
}

// Group represents a player group (party).
type Group struct {
	ID      string
	Leader  string
	Members map[string]*Client // username → Client
}

// NewHub creates and initialises a Hub with the loaded game world.
func NewHub(world *game.World, log *slog.Logger) *Hub {
	return nil // TODO: implement
}

// Run is the Hub's main event loop — blocks forever, processes one event at a time.
// Call as a goroutine: go hub.Run()
func (h *Hub) Run() {
	// TODO: implement — use select on h.register, h.unregister, h.commands
}

// broadcast sends msg to every client currently in roomID, except exclude (may be nil).
func (h *Hub) broadcast(roomID string, msg string, exclude *Client) {
	// TODO: implement
}

// broadcastAll sends msg to every connected client without exception.
func (h *Hub) broadcastAll(msg string) {
	// TODO: implement
}

// broadcastGroup sends msg to every member of groupID.
func (h *Hub) broadcastGroup(groupID string, msg string) {
	// TODO: implement
}

// removeClient removes c's player from the world and broadcasts PRESENCE LEAVE.
// Must be called from inside the Hub's Run() goroutine.
func (h *Hub) removeClient(c *Client) {
	// TODO: implement — remove BEFORE broadcasting (spec requirement)
}

// updatePlayerCount broadcasts the current total player count to all clients.
func (h *Hub) updatePlayerCount() {
	// TODO: implement — use protocol.StatsPlayers + broadcastAll
}
