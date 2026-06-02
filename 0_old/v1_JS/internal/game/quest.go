/*
Package game — Owner: Rui.

File quest.go: TAP quest system.
Quest progression per player: NotStarted → Active → Completed.
Supported quest types (define in world.json): "fetch", "defeat", "deliver".

Design decisions to document in the README:
  - How objectives are tracked and validated (auto vs. manual completion)
  - How rewards are granted (item added to inventory)
  - Quest dependencies / chains (if any)
*/
package game

import "errors"

// QuestState represents the lifecycle stage of a quest for one player.
type QuestState int

const (
	QuestNotStarted QuestState = iota
	QuestActive
	QuestCompleted
)

// QuestDef is the static definition of a quest loaded from world.json.
type QuestDef struct {
	ID          string
	Description string
	Type        string // "fetch", "defeat", "deliver"
	TargetID    string // itemID or npcID depending on Type
	Reward      string // itemID granted on completion
}

// PlayerQuest tracks a specific player's progress on one quest.
type PlayerQuest struct {
	Def      *QuestDef
	State    QuestState
	Progress int // e.g. items collected or enemies defeated so far
}

// ErrNoQuestAvailable is returned when an NPC has no quest for this player.
var ErrNoQuestAvailable = errors.New("no quest available")

// GetQuestFromNPC returns the QuestDef for npcName if a quest is available
// for this player (not already active or completed). Returns ErrNoQuestAvailable otherwise.
func GetQuestFromNPC(player *Player, room *Room, npcName string) (*QuestDef, error) {
	return nil, nil // TODO: implement
}

// StartQuest activates a quest for the player, setting its state to Active.
func StartQuest(player *Player, def *QuestDef) {
	// TODO: implement
}

// CheckCompletion returns true if the player has fully met the quest's objectives.
func CheckCompletion(player *Player, pq *PlayerQuest) bool {
	return false // TODO: implement
}

// CompleteQuest finalises the quest: grants the reward item and sets state to Completed.
func CompleteQuest(world *World, player *Player, pq *PlayerQuest) error {
	return nil // TODO: implement
}

// ListQuests returns all of the player's quests (active and completed).
func ListQuests(player *Player) []*PlayerQuest {
	return nil // TODO: implement
}
