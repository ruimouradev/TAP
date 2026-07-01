/*
Package worldfile.

File validate.go: referential integrity checks for the world data file.
Must be called after Load() and BEFORE the server accepts any connections.
Fail early with a clear, actionable error message if the world is broken.

Checks performed:
  - start_room exists in the rooms map
  - Every exit direction points to a room that exists
  - Every itemID referenced in a room exists in the items table
  - Every npcID referenced in a room exists in the npcs table
  - Every quest_id referenced in an NPC exists in the quests table
  - Spec minimums: >=8 rooms, >=3 distinct NPC roles, >=4 items (>=2 takeable), >=2 quests
*/
package worldfile

import "fmt"

// Validate checks the WorldFile for referential integrity and spec minimums.
// Returns a descriptive error for the first violation found.
func Validate(wf *WorldFile) error {
	if wf == nil {
		return fmt.Errorf("worldfile is nil")
	}

	if err := checkStartRoom(wf); err != nil {
		return err
	}
	if err := checkExits(wf); err != nil {
		return err
	}
	if err := checkItemRefs(wf); err != nil {
		return err
	}
	if err := checkNPCRefs(wf); err != nil {
		return err
	}
	if err := checkQuestRefs(wf); err != nil {
		return err
	}
	if err := checkQuestTargets(wf); err != nil {
		return err 
	}
	if err := checkMinimums(wf); err != nil {
		return err
	}

	return nil
}

// checkStartRoom verifies that start_room exists in the rooms map.
func checkStartRoom(wf *WorldFile) error {
	if wf.StartRoom == "" {
		return fmt.Errorf("start_room is empty")
	}
	if _, ok := wf.Rooms[wf.StartRoom]; !ok {
		return fmt.Errorf("start_room %q not found in rooms", wf.StartRoom)
	}
	return nil
}

// checkExits verifies every exit direction in every room points to an existing room.
func checkExits(wf *WorldFile) error {
	for roomID, room := range wf.Rooms {
		if room.Exits == nil {
			continue
		}
		for dir, target := range room.Exits {
			if _, ok := wf.Rooms[target]; !ok {
				return fmt.Errorf("room %q has exit %q -> unknown room %q", roomID, dir, target)
			}
		}
	}
	return nil
}

// checkItemRefs verifies all itemIDs listed in room definitions exist in the items table.
func checkItemRefs(wf *WorldFile) error {
	for roomID, room := range wf.Rooms {
		if room.Items == nil {
			continue
		}
		for _, itemID := range room.Items {
			if _, ok := wf.Items[itemID]; !ok {
				return fmt.Errorf("room %q references unknown item %q", roomID, itemID)
			}
		}
	}
	return nil
}

// checkNPCRefs verifies all npcIDs listed in room definitions exist in the npcs table.
func checkNPCRefs(wf *WorldFile) error {
	for roomID, room := range wf.Rooms {
		if room.NPCs == nil {
			continue
		}
		for _, npcID := range room.NPCs {
			if _, ok := wf.NPCs[npcID]; !ok {
				return fmt.Errorf("room %q references unknown npc %q", roomID, npcID)
			}
		}
	}
	return nil
}

// checkQuestRefs verifies all quest_id values in NPC definitions exist in the quests table.
func checkQuestRefs(wf *WorldFile) error {
	for npcID, npc := range wf.NPCs {
		if npc.QuestID == "" {
			continue
		}
		if _, ok := wf.Quests[npc.QuestID]; !ok {
			return fmt.Errorf("npc %q references unknown quest %q", npcID, npc.QuestID)
		}
	}
	return nil
}

// checkQuestTargets verifies every quest's target and reward reference an
// existing item or NPC.
func checkQuestTargets(wf *WorldFile) error {
	for id, q := range wf.Quests {
		if q.Reward != "" {
			if _, ok := wf.Items[q.Reward]; !ok {
				return fmt.Errorf("quest %q has unknown reward item %q", id, q.Reward)
			}
		}
		_, isItem := wf.Items[q.TargetID]
		_, isNPC := wf.NPCs[q.TargetID]
		if !isItem && !isNPC {
			return fmt.Errorf("quest %q has unknown target %q", id, q.TargetID)
		}
	}
	return nil
}

// checkMinimums ensures the world meets the spec's minimum requirements:
// >=8 rooms, >=3 distinct NPC roles, >=4 items (>=2 takeable), >=2 quests.
func checkMinimums(wf *WorldFile) error {
	if len(wf.Rooms) < 8 {
		return fmt.Errorf("world must have at least 8 rooms (have %d)", len(wf.Rooms))
	}

	// distinct NPC roles
	roles := make(map[string]struct{})
	for _, npc := range wf.NPCs {
		if npc.Role != "" {
			roles[npc.Role] = struct{}{}
		}
	}
	if len(roles) < 3 {
		return fmt.Errorf("world must contain at least 3 distinct NPC roles (have %d)", len(roles))
	}

	// items count and takeable
	if len(wf.Items) < 4 {
		return fmt.Errorf("world must contain at least 4 items (have %d)", len(wf.Items))
	}
	takeable := 0
	for _, it := range wf.Items {
		if it.Takeable {
			takeable++
		}
	}
	if takeable < 2 {
		return fmt.Errorf("world must contain at least 2 takeable items (have %d)", takeable)
	}

	if len(wf.Quests) < 2 {
		return fmt.Errorf("world must contain at least 2 quests (have %d)", len(wf.Quests))
	}

	return nil
}
