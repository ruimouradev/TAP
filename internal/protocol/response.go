/*
Package protocol — Owner: BOTH (Rui + Alexandre).

File response.go: helper functions for formatting OK and ERR responses
that the server sends back to a client after processing a command.

Wire formats:
  OK [data]\n
  ERR <code> <message>\n
*/
package protocol

// OK formats a success response with optional payload data.
// If data is empty, returns "OK\n".
func OK(data string) string {
	return "" // TODO: implement
}

// OKf formats a success response using fmt.Sprintf-style formatting.
func OKf(format string, args ...any) string {
	return "" // TODO: implement
}

// OKJson marshals v to JSON and wraps it in an OK response line.
// Returns an ERR response line if marshalling fails.
func OKJson(v any) string {
	return "" // TODO: implement
}

// Errf formats an error response: "ERR <code> <message>\n".
func Errf(code int, msg string) string {
	return "" // TODO: implement
}
