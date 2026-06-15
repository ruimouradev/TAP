// cmd/server: entry point for the TAP TCP server. Loads and validates the
// world, creates the Hub, and accepts connections until Ctrl+C.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"

	"the-answer-protocol/internal/config"
	"the-answer-protocol/internal/game"
	"the-answer-protocol/internal/server"
	"the-answer-protocol/internal/worldfile"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: server <world.json>")
		os.Exit(1)
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	wf, err := worldfile.Load(os.Args[1])
	if err != nil {
		log.Error("load world", "err", err)
		os.Exit(1)
	}
	if err := worldfile.Validate(wf); err != nil {
		log.Error("invalid world", "err", err)
		os.Exit(1)
	}

	hub := server.NewHub(game.NewWorld(wf), log)
	go hub.Run()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	addr := ":" + config.Port()

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Error("listen", "err", err)
		os.Exit(1)
	}
	defer ln.Close()
	log.Info("server listening", "addr", addr)

	listenAndServe(ctx, ln, hub, log)
}

// listenAndServe accepts connections until ctx is cancelled (Ctrl+C).
func listenAndServe(ctx context.Context, ln net.Listener, hub *server.Hub, log *slog.Logger) {
	go func() {
		<-ctx.Done()
		ln.Close() // unblocks Accept below
	}()
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				log.Info("shutting down")
				return
			default:
				log.Error("accept", "err", err)
				continue
			}
		}
		hub.Accept(conn)
	}
}
