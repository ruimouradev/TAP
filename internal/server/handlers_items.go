// handlers_items.go: item handling and NPC dialogue (TAKE / DROP / INVENTORY / TALK).
package server

import (
	"strings"

	"the-answer-protocol/internal/game"
	"the-answer-protocol/internal/protocol"
)

// handleTake: TAKE <item> - move an item from the room floor to the inventory.
// The identifier may be an ID or a multi-word display name, so we join the args.
func handleTake(h *Hub, c *Client, p *game.Player, args []string) string {
	room := h.world.GetRoom(p.CurrentRoom)
	it, err := game.TakeItem(room, p, strings.Join(args, " "))
	if err != nil {
		return protocol.ErrItemNotFound.Wire()
	}
	h.log.Info("item taken", "user", c.username, "item", it.ID, "room", p.CurrentRoom)
	return protocol.OKf("taken=%s", it.ID)
}

// handleDrop: DROP <item> - move an item from the inventory back to the floor.
func handleDrop(h *Hub, c *Client, p *game.Player, args []string) string {
	room := h.world.GetRoom(p.CurrentRoom)
	it, err := game.DropItem(p, room, strings.Join(args, " "))
	if err != nil {
		return protocol.ErrItemNotInInv.Wire()
	}
	h.log.Info("item dropped", "user", c.username, "item", it.ID, "room", p.CurrentRoom)
	return protocol.OKf("dropped=%s", it.ID)
}

// handleInventory: INVENTORY - the player's items as a JSON array.
func handleInventory(h *Hub, c *Client, p *game.Player, args []string) string {
	items := make([]protocol.InventoryItem, 0, len(p.Inventory))
	for _, it := range p.Inventory {
		items = append(items, protocol.InventoryItem{ID: it.ID, Name: it.Name})
	}
	return protocol.OKJson(items)
}

// handleTalk: TALK <npc> - return the NPC's dialogue (matched by ID or name).
func handleTalk(h *Hub, c *Client, p *game.Player, args []string) string {
	room := h.world.GetRoom(p.CurrentRoom)
	npc, ok := h.world.NPCInRoom(room, strings.Join(args, " "))
	if !ok {
		return protocol.ErrNPCNotFound.Wire()
	}
	return protocol.OKJson(protocol.TalkResponse{NPC: npc.Name, Dialogue: npc.Dialogue})
}
