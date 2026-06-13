// hub.go: the heart of all server concurrency. The Hub is the ONLY goroutine
// that reads or mutates game state. Clients talk to it through channels, and it
// handles one event at a time in its select loop — so no mutexes are needed.
package server

import (
	"log/slog"
	"net"

	"the-answer-protocol/internal/game"
	"the-answer-protocol/internal/protocol"
)

// Hub is the central actor that owns the game world.
type Hub struct {
	register   chan *Client
	unregister chan *Client
	commands   chan incomingCmd
	clients    map[*Client]bool
	groups     map[string]*Group // groupID → Group (used in Bloco 4)
	world      *game.World       // only the Hub touches this
	log        *slog.Logger
}

// incomingCmd pairs a parsed command with the client who sent it.
type incomingCmd struct {
	client *Client
	cmd    protocol.Command
}

// Group represents a player group (party). Used by the Bloco 4 handlers.
type Group struct {
	ID      string
	Leader  string
	Members map[string]*Client
}

func NewHub(world *game.World, log *slog.Logger) *Hub {
	return &Hub{
		register:   make(chan *Client),
		unregister: make(chan *Client),
		commands:   make(chan incomingCmd),
		clients:    make(map[*Client]bool),
		groups:     make(map[string]*Group),
		world:      world,
		log:        log,
	}
}

// Accept wires a new connection into the Hub: registers it, then starts its
// pumps. The register send blocks until the Hub has the client, so no command
// can be processed before the client is known.
func (h *Hub) Accept(conn net.Conn) {
	c := newClient(conn, h, h.log)
	h.register <- c
	go c.readPump()
	go c.writePump()
}

// Run is the Hub's event loop: one event at a time, forever.
func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			h.clients[c] = true
			c.safeSend(protocol.OKf("hello proto=%d", 1))

		case c := <-h.unregister:
			if h.clients[c] {
				h.removeClient(c) // remove state + broadcast LEAVE
				delete(h.clients, c)
				close(c.send) // ends writePump
			}

		case ic := <-h.commands:
			// ignore commands from a client already gone (register/unregister
			// and commands arrive on different channels and may reorder)
			if h.clients[ic.client] {
				h.dispatch(ic.client, ic.cmd)
			}
		}
	}
}

// broadcast sends msg to every connected player in roomID, skipping exclude.
func (h *Hub) broadcast(roomID, msg string, exclude *Client) {
	for c := range h.clients {
		if c == exclude || c.username == "" {
			continue
		}
		if p := h.world.GetPlayer(c.username); p != nil && p.CurrentRoom == roomID {
			c.safeSend(msg)
		}
	}
}

// broadcastAll sends msg to every authenticated client.
func (h *Hub) broadcastAll(msg string) {
	for c := range h.clients {
		if c.username != "" {
			c.safeSend(msg)
		}
	}
}

// removeClient takes the player out of the world and announces the departure.
// Must run inside Run(). State is removed BEFORE the broadcast (spec).
func (h *Hub) removeClient(c *Client) {
	if c.username == "" {
		return // never finished CONNECT
	}
	p := h.world.GetPlayer(c.username)
	if p == nil {
		return
	}
	roomID := p.CurrentRoom
	h.world.RemovePlayer(c.username)
	h.broadcast(roomID, protocol.RoomPresenceLeave(c.username), c)
	h.updatePlayerCount()
}

// updatePlayerCount tells everyone the current player total.
func (h *Hub) updatePlayerCount() {
	h.broadcastAll(protocol.StatsPlayers(h.world.TotalPlayers()))
}
