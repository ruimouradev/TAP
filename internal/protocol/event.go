// event.go: formatters for the server's asynchronous EVT messages.
// Wire format: "EVT <category> <type> [data]\n".
package protocol

import "fmt"

func RoomPresenceEnter(username string) string {
	return fmt.Sprintf("EVT ROOM PRESENCE ENTER %s\n", username)
}

func RoomPresenceLeave(username string) string {
	return fmt.Sprintf("EVT ROOM PRESENCE LEAVE %s\n", username)
}

func RoomChat(username, message string) string {
	return fmt.Sprintf("EVT ROOM CHAT %s %s\n", username, message)
}

func GlobalChat(username, message string) string {
	return fmt.Sprintf("EVT GLOBAL CHAT %s %s\n", username, message)
}

func GroupInvite(leader string) string {
	return fmt.Sprintf("EVT GROUP INVITE %s\n", leader)
}

func GroupJoin(username string) string {
	return fmt.Sprintf("EVT GROUP JOIN %s\n", username)
}

func GroupLeave(username string) string {
	return fmt.Sprintf("EVT GROUP LEAVE %s\n", username)
}

func GroupChat(username, message string) string {
	return fmt.Sprintf("EVT GROUP CHAT %s %s\n", username, message)
}

func StatsPlayers(count int) string {
	return fmt.Sprintf("EVT STATS players=%d\n", count)
}
