// item.go: the Item type and the TAKE and DROP logic. Items are unique
// instances, matched by ID or by display name. Matching ignores case and
// multi-word names are supported.
package game

import (
	"errors"
	"strings"
)

// Item is a unique instance of a game resource.
type Item struct {
	ID   string // canonical ID, like "item.healing_potion"
	Name string // display name, like "Healing Potion"
}

var (
	ErrItemNotFound       = errors.New("item not found")
	ErrItemNotInInventory = errors.New("item not in inventory")
)

// FindItem matches an item by ID first, then by display name, ignoring case.
func FindItem(items map[string]*Item, identifier string) (*Item, bool) {
	if it, ok := items[identifier]; ok {
		return it, true
	}
	for _, it := range items {
		if strings.EqualFold(it.Name, identifier) {
			return it, true
		}
	}
	return nil, false
}

// TakeItem moves an item from the room floor into the player's inventory.
func TakeItem(room *Room, player *Player, identifier string) (*Item, error) {
	it, ok := FindItem(room.Items, identifier)
	if !ok {
		return nil, ErrItemNotFound
	}
	delete(room.Items, it.ID)
	player.Inventory[it.ID] = it
	return it, nil
}

// DropItem moves an item from the player's inventory back to the room floor.
func DropItem(player *Player, room *Room, identifier string) (*Item, error) {
	it, ok := FindItem(player.Inventory, identifier)
	if !ok {
		return nil, ErrItemNotInInventory
	}
	delete(player.Inventory, it.ID)
	room.Items[it.ID] = it
	return it, nil
}
