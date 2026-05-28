/*
Package server — Owner: Rui.

File client.go: represents a single connected player's TCP connection.
Each Client has two goroutines:
  - readPump: reads lines from the socket and forwards them to the Hub via channels.
  - writePump: drains the send channel and writes messages to the socket.

The send channel is buffered — if a client is slow, the Hub never blocks.
If the buffer fills up the client is disconnected to protect the server.
*/
package server

import (
	"bufio"
	"log/slog"
	"net"
)

// sendBufferSize is the maximum number of queued messages per client.
// If the buffer fills the client is considered dead and disconnected.
const sendBufferSize = 32

// Client represents a single connected player's TCP connection.
type Client struct {
	conn     net.Conn
	hub      *Hub
	send     chan string   // buffered; writePump drains this
	username string        // empty before CONNECT succeeds
	log      *slog.Logger
}

// newClient creates a Client wrapping an accepted TCP connection.
func newClient(conn net.Conn, hub *Hub, log *slog.Logger) *Client {
	return nil // TODO: implement
}

// readPump reads lines from the TCP connection and forwards them to the Hub.
// When the connection closes, sends unregister to the Hub and returns.
// Call as a goroutine: go c.readPump()
func (c *Client) readPump() {
	// TODO: implement
	// Use bufio.Scanner; on scan error → hub.unregister <- c
	_ = bufio.NewScanner(nil)
}

// writePump drains the send channel and writes each message to the TCP connection.
// Returns when the send channel is closed.
// Call as a goroutine: go c.writePump()
func (c *Client) writePump() {
	// TODO: implement
}

// safeSend sends msg to the client's send channel without blocking.
// If the buffer is full it closes the connection and triggers cleanup.
func (c *Client) safeSend(msg string) {
	// TODO: implement — use select with default to avoid blocking the Hub
}
