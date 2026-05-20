/*
Package worldfile — Owner: Alexandre.

File validate.go: referential integrity checks for the world data file.
Must be called after Load() and BEFORE the server accepts any connections.
Fail early with a clear, actionable error message if the world is broken.

Checks performed:
  - start_room exists in the rooms map
  - Every exit direction points to a room that exists
  - Every itemID referenced in a room exists in the items table
  - Every npcID referenced in a room exists in the npcs table
  - Every quest_id referenced in an NPC exists in the quests table
  - Spec minimums: ≥8 rooms, ≥3 distinct NPC roles, ≥4 items (≥2 takeable), ≥2 quests
*/
package worldfile

import "fmt"

// Validate checks the WorldFile for referential integrity and spec minimums.
// Returns a descriptive error for the first violation found.
func Validate(wf *WorldFile) error {
	_ = fmt.Errorf
	return nil // TODO: implement — call all check* helpers in order
}

// checkStartRoom verifies that start_room exists in the rooms map.
func checkStartRoom(wf *WorldFile) error {
	return nil // TODO: implement
}

// checkExits verifies every exit direction in every room points to an existing room.
func checkExits(wf *WorldFile) error {
	return nil // TODO: implement
}

// checkItemRefs verifies all itemIDs listed in room definitions exist in the items table.
func checkItemRefs(wf *WorldFile) error {
	return nil // TODO: implement
}

// checkNPCRefs verifies all npcIDs listed in room definitions exist in the npcs table.
func checkNPCRefs(wf *WorldFile) error {
	return nil // TODO: implement
}

// checkQuestRefs verifies all quest_id values in NPC definitions exist in the quests table.
func checkQuestRefs(wf *WorldFile) error {
	return nil // TODO: implement
}

// checkMinimums ensures the world meets the spec's minimum requirements:
// ≥8 rooms, ≥3 distinct NPC roles, ≥4 items (≥2 takeable), ≥2 quests.
func checkMinimums(wf *WorldFile) error {
	return nil // TODO: implement
}
