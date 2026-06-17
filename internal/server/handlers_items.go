// handlers_items.go: item handling and NPC dialogue (TAKE / DROP / INVENTORY / TALK).
package server

import (
	"strings"

	"the-answer-protocol/internal/game"
	"the-answer-protocol/internal/protocol"
)

// handleTake: TAKE <item> — move an item from the room floor to the inventory.
// The identifier may be an ID or a multi-word display name, so we join the args.
func handleTake(h *Hub, c *Client, args []string) string {
	if len(args) < 1 {
		return protocol.Errf(errBadRequest, "MISSING_ITEM")
	}
	p := h.world.GetPlayer(c.username)
	room := h.world.GetRoom(p.CurrentRoom)
	it, err := game.TakeItem(room, p, strings.Join(args, " "))
	if err != nil {
		return protocol.Errf(protocol.ErrCodeItemNotFound, protocol.MsgItemNotFound)
	}
	h.log.Info("item taken", "user", c.username, "item", it.ID, "room", p.CurrentRoom)
	return protocol.OKf("took %s", it.ID)
}

// handleDrop: DROP <item> — move an item from the inventory back to the floor.
func handleDrop(h *Hub, c *Client, args []string) string {
	if len(args) < 1 {
		return protocol.Errf(errBadRequest, "MISSING_ITEM")
	}
	p := h.world.GetPlayer(c.username)
	room := h.world.GetRoom(p.CurrentRoom)
	it, err := game.DropItem(p, room, strings.Join(args, " "))
	if err != nil {
		return protocol.Errf(protocol.ErrCodeItemNotFound, protocol.MsgItemNotInInv)
	}
	h.log.Info("item dropped", "user", c.username, "item", it.ID, "room", p.CurrentRoom)
	return protocol.OKf("dropped %s", it.ID)
}

// handleInventory: INVENTORY — the player's items as a JSON array.
func handleInventory(h *Hub, c *Client, args []string) string {
	p := h.world.GetPlayer(c.username)
	items := make([]protocol.InventoryItem, 0, len(p.Inventory))
	for _, it := range p.Inventory {
		items = append(items, protocol.InventoryItem{ID: it.ID, Name: it.Name})
	}
	return protocol.OKJson(items)
}

// handleTalk: TALK <npc> — return the NPC's dialogue (matched by ID or name).
func handleTalk(h *Hub, c *Client, args []string) string {
	if len(args) < 1 {
		return protocol.Errf(errBadRequest, "MISSING_NPC")
	}
	p := h.world.GetPlayer(c.username)
	room := h.world.GetRoom(p.CurrentRoom)
	npc, ok := h.world.NPCInRoom(room, strings.Join(args, " "))
	if !ok {
		return protocol.Errf(protocol.ErrCodeNPCNotFound, protocol.MsgNPCNotFound)
	}
	return protocol.OKJson(protocol.TalkResponse{NPC: npc.Name, Dialogue: npc.Dialogue})
}
