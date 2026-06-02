/*
Package protocol — Owner: BOTH (Rui + Alexandre).

File event.go: formatting functions for every EVT message the server
sends asynchronously to relevant clients.

General wire format: "EVT <category> <type> [data]\n"

Categories defined by RFC 42TAP:

	ROOM   — presence and chat scoped to a single room
	GLOBAL — server-wide chat
	GROUP  — group membership and chat
	STATS  — aggregate counters (player count)
*/
package protocol

import "fmt"

// RoomPresenceEnter returns "EVT ROOM PRESENCE ENTER <username>\n".
// Sent to all players in a room when someone enters.
func RoomPresenceEnter(username string) string {
	return fmt.Sprintf("EVT ROOM PRESENCE ENTER %s\n", username)
}

// RoomPresenceLeave returns "EVT ROOM PRESENCE LEAVE <username>\n".
// Sent AFTER removing the player from the room (spec requirement).
func RoomPresenceLeave(username string) string {
	return fmt.Sprintf("EVT ROOM PRESENCE LEAVE %s\n", username)
}

// RoomChat returns "EVT ROOM CHAT <username> <message>\n".
func RoomChat(username, message string) string {
	return fmt.Sprintf("EVT ROOM CHAT %s %s\n", username, message)
}

// GlobalChat returns "EVT GLOBAL CHAT <username> <message>\n".
func GlobalChat(username, message string) string {
	return fmt.Sprintf("EVT GLOBAL CHAT %s %s\n", username, message)
}

// GroupInvite returns "EVT GROUP INVITE <leader>\n".
// Sent to the invited player.
func GroupInvite(leader string) string {
	return fmt.Sprintf("EVT GROUP INVITE %s\n", leader)
}

// GroupJoin returns "EVT GROUP JOIN <username>\n".
// Sent to all group members when someone joins.
func GroupJoin(username string) string {
	return fmt.Sprintf("EVT GROUP JOIN %s\n", username)
}

// GroupLeave returns "EVT GROUP LEAVE <username>\n".
// Sent to remaining group members when someone leaves.
func GroupLeave(username string) string {
	return fmt.Sprintf("EVT GROUP LEAVE %s\n", username)
}

// GroupChat returns "EVT GROUP CHAT <username> <message>\n".
func GroupChat(username, message string) string {
	return fmt.Sprintf("EVT GROUP CHAT %s %s\n", username, message)
}

// StatsPlayers returns "EVT STATS players=<count>\n".
// Broadcast to all clients whenever the player count changes.
func StatsPlayers(count int) string {
	return fmt.Sprintf("EVT STATS players=%d\n", count)
}
