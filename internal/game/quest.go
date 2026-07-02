// quest.go: the quest system. Progression is per player:
// NotStarted -> Active -> Completed. Types: "fetch" and "defeat".
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
	Type        string // "fetch" or "defeat"
	TargetID    string // itemID or npcID depending on Type
	Reward      string // itemID granted on completion
}

// PlayerQuest tracks a specific player's progress on one quest.
type PlayerQuest struct {
	Def   *QuestDef
	State QuestState
}

// ErrNoQuestAvailable is returned when an NPC has no quest for this player.
var ErrNoQuestAvailable = errors.New("no quest available")

// GetQuestFromNPC returns the quest offered by the named NPC in room, if the
// player can take it. If the NPC is not in the room the error is ErrNPCNotFound,
// which the handler turns into ERR 404 as the RFC asks. If the NPC is there but
// has nothing to offer the error is ErrNoQuestAvailable.
func GetQuestFromNPC(world *World, player *Player, room *Room, npcName string) (*QuestDef, error) {
	npc, ok := findNPC(room, npcName)
	if !ok {
		return nil, ErrNPCNotFound
	}
	if npc.QuestID == "" {
		return nil, ErrNoQuestAvailable
	}
	if _, already := player.Quests[npc.QuestID]; already {
		return nil, ErrNoQuestAvailable // already active or completed
	}
	def := world.Quests[npc.QuestID]
	if def == nil {
		return nil, ErrNoQuestAvailable
	}
	return def, nil
}

// StartQuest activates a quest for the player (state Active).
func StartQuest(player *Player, def *QuestDef) {
	player.Quests[def.ID] = &PlayerQuest{Def: def, State: QuestActive}
}

// CheckCompletion reports whether the player has met the quest's objective.
//   - "fetch":  the target item is in the player's inventory
//   - "defeat": the player has defeated the target NPC
func CheckCompletion(player *Player, pq *PlayerQuest) bool {
	switch pq.Def.Type {
	case "fetch":
		_, has := player.Inventory[pq.Def.TargetID]
		return has
	case "defeat":
		return player.Defeated[pq.Def.TargetID]
	default:
		return false
	}
}

// CompleteQuest marks the quest completed and grants its reward item (if any).
func CompleteQuest(world *World, player *Player, pq *PlayerQuest) error {
	pq.State = QuestCompleted
	if pq.Def.Reward == "" {
		return nil
	}
	reward, ok := world.Items[pq.Def.Reward]
	if !ok {
		return nil // reward item not in the catalog, nothing to grant
	}
	player.Inventory[reward.ID] = &Item{ID: reward.ID, Name: reward.Name}
	return nil
}

// ListQuests returns all of the player's quests (active and completed).
func ListQuests(player *Player) []*PlayerQuest {
	out := make([]*PlayerQuest, 0, len(player.Quests))
	for _, pq := range player.Quests {
		out = append(out, pq)
	}
	return out
}
