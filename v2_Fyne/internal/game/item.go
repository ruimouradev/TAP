/*
Package game — Owner: Rui.

File item.go: Item type and TAKE/DROP logic.
Items are unique instances — they are never duplicated.
TAKE removes the item from the room floor; DROP returns it to the room.
Items can be matched by canonical ID ("item.healing_potion") or by
display name ("Healing Potion"), case-insensitively, with multi-word support.
*/
package game

import "errors"

// Item is a unique instance of a game resource.
type Item struct {
	ID   string // canonical ID, e.g. "item.healing_potion"
	Name string // display name, e.g. "Healing Potion"
}

// ErrItemNotFound is returned when an item cannot be located in a room.
var ErrItemNotFound = errors.New("item not found")

// ErrItemNotInInventory is returned when a player tries to drop an item they don't have.
var ErrItemNotInInventory = errors.New("item not in inventory")

// TakeItem removes the named item from room and adds it to the player's inventory.
// identifier can be the item's ID or its name (case-insensitive, multi-word).
func TakeItem(room *Room, player *Player, identifier string) (*Item, error) {
	return nil, nil // TODO: implement
}

// DropItem removes the named item from the player's inventory and places it in room.
// identifier can be the item's ID or its name (case-insensitive, multi-word).
func DropItem(player *Player, room *Room, identifier string) (*Item, error) {
	return nil, nil // TODO: implement
}

// FindItem searches an items map for an item matching identifier.
// Matches by ID first, then by name (case-insensitive, supports multi-word names).
func FindItem(items map[string]*Item, identifier string) (*Item, bool) {
	return nil, false // TODO: implement
}
