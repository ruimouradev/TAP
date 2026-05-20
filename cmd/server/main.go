/*
cmd/server — Owner: Rui.

Entry point for the TAP TCP server.
Responsibilities:
  - Read the world.json path from os.Args[1]
  - Load and validate the world via worldfile.Load + worldfile.Validate
  - Create the Hub (the single actor that owns all game state)
  - Start the TCP listener on port 4242
  - Accept connections in a loop, spawning one goroutine per client
  - Handle graceful shutdown via signal.NotifyContext (Ctrl+C)
*/
package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"

	"the-answer-protocol/internal/game"
	"the-answer-protocol/internal/server"
	"the-answer-protocol/internal/worldfile"
)

// main loads the world, creates the Hub, and starts the TCP listener.
func main() {
	// TODO: implement
	_ = context.Background()
	_ = slog.Default()
	_ = os.Args
	_ = game.NewWorld
	_ = server.NewHub
	_ = worldfile.Load
	_ = signal.NotifyContext
}

// listenAndServe accepts TCP connections in a loop and spawns a Client goroutine per connection.
// Returns when ctx is cancelled (e.g. Ctrl+C).
func listenAndServe(ctx context.Context, ln net.Listener, hub *server.Hub, log *slog.Logger) {
	// TODO: implement
}
