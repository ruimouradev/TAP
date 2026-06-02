/*
cmd/cli — Owner: Rui.

TAP command-line client.
Connects to the server over TCP and lets the user send commands and
receive events in real time using two concurrent goroutines:
  - readLoop: reads from the server and prints to stdout
  - writeLoop: reads from stdin and sends to the server

The user may type raw RFC commands ("MOVE north", "CHAT GLOBAL hello")
or a friendlier format translated by the CLI — document the choice in the README.
*/
package main

import (
	"net"
	"os"
)

// main connects to the TAP server and starts the interactive CLI session.
func main() {
	// TODO: implement
	// Suggested: addr := os.Args[1]  (e.g. "localhost:4242")
	_ = os.Args
	_ = net.Dial
}

// readLoop reads event/response lines from the server and prints them to stdout.
// Runs in its own goroutine; closes done when the server closes the connection.
func readLoop(conn net.Conn, done chan<- struct{}) {
	// TODO: implement
}

// writeLoop reads lines from stdin and sends them to the server.
// Returns when stdin is closed or done is signalled by readLoop.
func writeLoop(conn net.Conn, done <-chan struct{}) {
	// TODO: implement
}
