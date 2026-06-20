// combat.go: the combat system. Combat is turn-based - each ATTACK command is
// one round. There is no persistent "in combat" state, so any command is valid
// between rounds. The attacked NPC counter-attacks each round unless it is
// defeated; when a player's HP reaches 0 they respawn at the start room with
// half their max HP.
package game

import (
	"errors"
	"math/rand"
	"strings"
)

// CombatResult holds the outcome of one round of combat.
type CombatResult struct {
	AttackerHP int    // player HP after this round
	TargetHP   int    // NPC HP after this round
	Damage     int    // damage dealt to the NPC
	CounterDmg int    // counter-attack damage dealt to the player (0 if NPC defeated)
	Status     string // "combat", "victory", "defeat"
	Defeated   bool   // true if the NPC was defeated this round
	PlayerDied bool   // true if the player's HP reached 0
}

var (
	ErrNPCNotFound   = errors.New("npc not found")
	ErrNPCNotHostile = errors.New("npc not hostile")
)

// Damage of a single hit is a random value in [minDamage, maxDamage], for the
// player and the NPC alike. Neither has separate attack/defense stats, so the
// formula is intentionally simple - change the range to tune difficulty.
const (
	minDamage = 10
	maxDamage = 20
)

// calculateDamage returns the damage of one hit.
func calculateDamage() int {
	return minDamage + rand.Intn(maxDamage-minDamage+1)
}

// Attack runs one round of combat between player and the named NPC in room.
// The player strikes first; if the NPC survives, it counter-attacks.
func Attack(player *Player, room *Room, npcName string) (*CombatResult, error) {
	npc, ok := findNPC(room, npcName)
	if !ok {
		return nil, ErrNPCNotFound
	}
	if !npc.Hostile {
		return nil, ErrNPCNotHostile
	}

	res := &CombatResult{}

	// player hits the NPC
	res.Damage = calculateDamage()
	npc.HP -= res.Damage
	if npc.HP <= 0 {
		npc.HP = 0
		res.Defeated = true
		res.Status = "victory"
		res.TargetHP = 0
		res.AttackerHP = player.HP
		player.Defeated[npc.ID] = true // for "defeat" quests (record before removing)
		delete(room.NPCs, npc.ID)      // defeated NPC leaves the room (can't be hit/talked again)
		return res, nil
	}

	// the NPC survives and counter-attacks
	res.CounterDmg = calculateDamage()
	player.HP -= res.CounterDmg
	if player.HP <= 0 {
		player.HP = 0
		res.PlayerDied = true
		res.Status = "defeat"
	} else {
		res.Status = "combat"
	}
	res.TargetHP = npc.HP
	res.AttackerHP = player.HP
	return res, nil
}

// RespawnPlayer moves a defeated player back to the start room with half HP.
func RespawnPlayer(world *World, player *Player) {
	if room := world.Rooms[player.CurrentRoom]; room != nil {
		delete(room.Players, player.Username)
	}
	player.CurrentRoom = world.StartRoom
	player.HP = player.MaxHP / 2
	if start := world.Rooms[world.StartRoom]; start != nil {
		start.Players[player.Username] = player
	}
}

// findNPC looks up an NPC in room by ID or display name (case-insensitive).
func findNPC(room *Room, name string) (*NPC, bool) {
	for _, npc := range room.NPCs {
		if strings.EqualFold(npc.ID, name) || strings.EqualFold(npc.Name, name) {
			return npc, true
		}
	}
	return nil, false
}
