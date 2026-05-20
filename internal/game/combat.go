/*
Package game — Owner: Rui.

File combat.go: TAP combat system.
Design decision (document in README):
  - Turn-based: each ATTACK command is one combat round.
  - No persistent "in combat" state — any command is valid between attacks.
  - Counter-attack: the NPC retaliates each round unless it is defeated.
  - Defeat: player HP reaches 0 → respawn at start room with 50% MaxHP.
  - Optional commands to design: DEFEND (reduce incoming damage), FLEE (exit combat).

Document the damage formula, initiative order, and any extra commands in the README.
*/
package game

import "errors"

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

// ErrNPCNotHostile is returned when trying to attack a non-hostile NPC.
var ErrNPCNotHostile = errors.New("npc not hostile")

// Attack executes one round of combat between player and the named NPC in room.
// Returns the full CombatResult so the server can broadcast it.
func Attack(player *Player, room *Room, npcName string) (*CombatResult, error) {
	return nil, nil // TODO: implement
}

// calculateDamage computes how much damage is dealt in one attack.
// Define your formula here and document it in the README.
func calculateDamage() int {
	return 0 // TODO: implement — decide attacker/defender stats first
}

// RespawnPlayer resets a dead player at the world's start room with 50% MaxHP.
// Called by the Hub after a player's HP reaches 0.
func RespawnPlayer(world *World, player *Player) {
	// TODO: implement
}
