// handlers_combat.go: combat commands (ATTACK / STATUS).
package server

import (
	"strings"

	"the-answer-protocol/internal/game"
	"the-answer-protocol/internal/protocol"
)

// handleAttack: ATTACK <npc> — one round of combat against a hostile NPC.
func handleAttack(h *Hub, c *Client, args []string) string {
	if len(args) < 1 {
		return protocol.Errf(errBadRequest, "MISSING_NPC")
	}
	p := h.world.GetPlayer(c.username)
	room := h.world.GetRoom(p.CurrentRoom)
	res, err := game.Attack(p, room, strings.Join(args, " "))
	if err != nil {
		if err == game.ErrNPCNotHostile {
			return protocol.Errf(protocol.ErrCodeNPCNotHostile, protocol.MsgNPCNotHostile)
		}
		return protocol.Errf(protocol.ErrCodeNPCNotFound, protocol.MsgNPCNotFound)
	}
	// let the others in the room see the combat round (player's still here)
	h.broadcast(p.CurrentRoom, protocol.RoomCombat(c.username, strings.Join(args, " "), res.Damage, res.TargetHP), c)
	// on defeat the player respawns at the start room; tell both rooms
	if res.PlayerDied {
		from := p.CurrentRoom
		game.RespawnPlayer(h.world, p)
		if from != p.CurrentRoom {
			h.broadcast(from, protocol.RoomPresenceLeave(c.username), c)
			h.broadcast(p.CurrentRoom, protocol.RoomPresenceEnter(c.username), c)
		}
	}
	h.log.Info("combat", "user", c.username, "target", strings.Join(args, " "),
		"damage", res.Damage, "counter", res.CounterDmg, "status", res.Status)
	return protocol.OKJson(protocol.AttackResponse{
		Damage:     res.Damage,
		Counter:    res.CounterDmg,
		AttackerHP: res.AttackerHP,
		TargetHP:   res.TargetHP,
		Status:     res.Status,
	})
}

// handleStatus: STATUS — the player's HP and condition.
func handleStatus(h *Hub, c *Client, args []string) string {
	p := h.world.GetPlayer(c.username)
	return protocol.OKJson(protocol.StatusResponse{
		HP:     p.HP,
		MaxHP:  p.MaxHP,
		Status: healthLabel(p),
	})
}

// healthLabel maps a player's HP to the STATUS condition string.
func healthLabel(p *game.Player) string {
	switch {
	case p.HP <= 0:
		return "dead"
	case p.HP*2 < p.MaxHP:
		return "wounded"
	default:
		return "healthy"
	}
}
