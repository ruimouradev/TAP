// client.go: one connected player's TCP connection, with a read and a write
// goroutine. The send channel is buffered so the Hub never blocks on a slow client.
package server

import (
	"bufio"
	"io"
	"log/slog"
	"net"

	"the-answer-protocol/internal/protocol"
)

// max number of queued messages per client
const sendBufferSize = 32

// Client represents a single connected player's TCP connection.
type Client struct {
	conn         net.Conn
	hub          *Hub
	send         chan string // buffered, writePump drains it
	username     string      // empty before CONNECT succeeds
	invitedGroup string      // pending group invite (group ID), empty if none
	addr         string      // remote IP:port, kept for logging after close
	log          *slog.Logger

	// command flood detection (only touched inside the Hub goroutine, so no lock)
	cmdRate rateWindow
}

func newClient(conn net.Conn, hub *Hub, log *slog.Logger) *Client {
	return &Client{
		conn: conn,
		hub:  hub,
		send: make(chan string, sendBufferSize),
		addr: conn.RemoteAddr().String(),
		log:  log,
	}
}

// readPump reads lines from the socket and forwards parsed commands to the Hub.
// When the connection ends it tells the Hub to unregister this client.
func (c *Client) readPump() {
	defer func() { c.hub.unregister <- c }()

	sc := bufio.NewScanner(c.conn)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024) // allow long lines
	for sc.Scan() {
		cmd, err := protocol.Parse(sc.Text())
		if err != nil {
			continue // blank line
		}
		c.hub.commands <- incomingCmd{client: c, cmd: cmd}
	}
}

// writePump drains the send channel to the socket. Ends when send is closed.
func (c *Client) writePump() {
	defer c.conn.Close()
	for msg := range c.send {
		if _, err := io.WriteString(c.conn, msg); err != nil {
			return
		}
	}
}

// safeSend queues msg without ever blocking the Hub. A full buffer means the
// client is too slow, so we drop the connection. Cleanup then runs via readPump.
func (c *Client) safeSend(msg string) {
	select {
	case c.send <- msg:
	default:
		c.conn.Close()
	}
}
