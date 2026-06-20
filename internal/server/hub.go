// hub.go: the heart of all server concurrency. The Hub is the ONLY goroutine
// that reads or mutates game state. Clients talk to it through channels, and it
// handles one event at a time in its select loop - so no mutexes are needed.
package server

import (
	"log/slog"
	"net"
	"time"

	"the-answer-protocol/internal/game"
	"the-answer-protocol/internal/protocol"
)

// Rapid-connection detection: more than connLimit connections from one IP within
// connWindow is logged as a possible abuse pattern (we monitor, not block).
const (
	connWindow = 10 * time.Second
	connLimit  = 5
)

// Hub is the central actor that owns the game world.
type Hub struct {
	register   chan *Client
	unregister chan *Client
	commands   chan incomingCmd
	clients    map[*Client]struct{} // set of connected clients
	groups     map[string]*Group    // groupID -> Group (empty until groups exist)
	world      *game.World          // only the Hub touches this
	log        *slog.Logger

	// rapid-connection tracking per IP (only touched in the Hub goroutine, no lock)
	connRate map[string]*rateWindow
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
	Members map[string]*Client
}

func NewHub(world *game.World, log *slog.Logger) *Hub {
	return &Hub{
		register:   make(chan *Client),
		unregister: make(chan *Client),
		commands:   make(chan incomingCmd),
		clients:    make(map[*Client]struct{}),
		groups:     make(map[string]*Group),
		world:      world,
		log:        log,
		connRate:   make(map[string]*rateWindow),
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
			h.clients[c] = struct{}{}
			h.trackRapidConnections(c)
			h.log.Info("client connected", "addr", c.addr)
			c.safeSend(protocol.OKf("hello proto=%d", 1))

		case c := <-h.unregister:
			if _, ok := h.clients[c]; ok {
				h.removeClient(c) // remove state + broadcast LEAVE
				delete(h.clients, c)
				close(c.send) // ends writePump
				h.log.Info("client disconnected", "addr", c.addr, "user", c.username)
			}

		case ic := <-h.commands:
			// ignore commands from a client already gone (register/unregister
			// and commands arrive on different channels and may reorder)
			if _, ok := h.clients[ic.client]; ok {
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
	if p.GroupID != "" {
		h.leaveGroup(c, p)
	}
	roomID := p.CurrentRoom
	h.world.RemovePlayer(c.username)
	h.broadcast(roomID, protocol.RoomPresenceLeave(c.username), c)
	h.updatePlayerCount()
}

// clientByUsername returns the connected client with username, or nil.
func (h *Hub) clientByUsername(username string) *Client {
	for c := range h.clients {
		if c.username == username {
			return c
		}
	}
	return nil
}

// broadcastGroup sends msg to every member of grp, skipping exclude.
func (h *Hub) broadcastGroup(grp *Group, msg string, exclude *Client) {
	for _, m := range grp.Members {
		if m == exclude {
			continue
		}
		m.safeSend(msg)
	}
}

// leaveGroup removes c from their group and announces it to the rest. If the
// leader leaves or the group becomes empty, the whole group is disbanded.
func (h *Hub) leaveGroup(c *Client, p *game.Player) {
	grp := h.groups[p.GroupID]
	p.GroupID = ""
	if grp == nil {
		return
	}
	delete(grp.Members, c.username)
	if grp.Leader == c.username || len(grp.Members) == 0 {
		for name, m := range grp.Members {
			if mp := h.world.GetPlayer(name); mp != nil {
				mp.GroupID = ""
			}
			m.safeSend(protocol.GroupLeave(c.username))
		}
		delete(h.groups, grp.ID)
		return
	}
	h.broadcastGroup(grp, protocol.GroupLeave(c.username), nil)
}

// updatePlayerCount tells everyone the current player total.
func (h *Hub) updatePlayerCount() {
	h.broadcastAll(protocol.StatsPlayers(h.world.TotalPlayers()))
}

// trackRapidConnections counts connections per IP in a sliding window and logs a
// WARN when one IP crosses the limit (abuse monitoring, not blocking). Runs in
// the Hub goroutine on register, so the maps need no lock.
func (h *Hub) trackRapidConnections(c *Client) {
	host, _, err := net.SplitHostPort(c.addr)
	if err != nil {
		host = c.addr // fall back to the raw address if there's no port
	}
	w := h.connRate[host]
	if w == nil {
		w = &rateWindow{}
		h.connRate[host] = w
	}
	if w.exceeded(time.Now(), connLimit, connWindow) {
		h.log.Warn("possible rapid connections", "addr", host, "count", w.count, "window", connWindow.String())
	}
}
