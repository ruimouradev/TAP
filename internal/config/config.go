// Package config holds settings shared by the server and the clients.
package config

import "os"

// DefaultPort is the TCP port used when TAP_PORT is not set.
// Change it here and the server, the CLI and the GUI all follow.
const DefaultPort = "4300"

// Port returns the port to use: TAP_PORT if set, otherwise DefaultPort.
func Port() string {
	if p := os.Getenv("TAP_PORT"); p != "" {
		return p
	}
	return DefaultPort
}
