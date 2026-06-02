/*
cmd/gui — Owner: Alexandre.

HTTP/WebSocket → TCP bridge for the TAP web GUI client.
This binary does two things:
 1. Serves the static files from web/ (index.html, app.js, style.css)
 2. Bridges the browser (WebSocket) to the TAP server (TCP):
    each WS connection opens its own TCP connection to the TAP server
    and relays messages bidirectionally without altering their content.

The browser speaks WebSocket; the TAP server speaks TCP line-protocol.
This bridge translates between the two transparently.
*/
package main

import (
	"net/http"
)

// main starts the HTTP server (default :8080) that serves the GUI and the WS bridge.
func main() {
	// TODO: implement
	// Register handlers, then call http.ListenAndServe(":8080", nil)
}

// serveHome serves web/index.html as the root page of the GUI.
func serveHome(w http.ResponseWriter, r *http.Request) {
	// TODO: implement
}

// handleWS upgrades the HTTP request to a WebSocket connection,
// opens a TCP connection to the TAP server, and starts bidirectional relay.
func handleWS(w http.ResponseWriter, r *http.Request) {
	// TODO: implement
	// Use gorilla/websocket or nhooyr.io/websocket for the WS upgrade
}

// relayWStoTCP copies messages from WebSocket to TCP (browser → server).
// Runs in a dedicated goroutine.
func relayWStoTCP( /* wsConn, tcpConn */ ) {
	// TODO: implement once you have chosen the WebSocket dependency
}

// relayTCPtoWS copies lines from TCP to WebSocket (server → browser).
// Runs in a dedicated goroutine.
func relayTCPtoWS( /* tcpConn, wsConn */ ) {
	// TODO: implement once you have chosen the WebSocket dependency
}
