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

// RoomCombat announces one combat round to the other players in the room.
// Combat events are not in the RFC's category list. Documented as a deviation.
func RoomCombat(attacker, target string, damage, targetHP int) string {
	return fmt.Sprintf("EVT ROOM COMBAT %s %s %d %d\n", attacker, target, damage, targetHP)
}

// RoomItemTaken and RoomItemDropped tell the rest of the room that an item was
// picked up or dropped, so other clients refresh their room view instantly
// instead of polling. Not in the RFC's event list. Documented as a deviation.
func RoomItemTaken(username, itemID string) string {
	return fmt.Sprintf("EVT ROOM ITEM TAKEN %s %s\n", username, itemID)
}

func RoomItemDropped(username, itemID string) string {
	return fmt.Sprintf("EVT ROOM ITEM DROPPED %s %s\n", username, itemID)
}

func StatsPlayers(count int) string {
	return fmt.Sprintf("EVT STATS players=%d\n", count)
}
