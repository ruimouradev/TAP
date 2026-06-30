/*
Package worldfile.

File loader.go: reads and parses the world data file (world.json).
Converts the JSON schema into Go structs that the game package uses to
build the live world. Called at server startup before accepting clients.

The WorldFile structs here are the FILE SCHEMA - they map 1:1 to the
JSON keys. The game package converts them into its own runtime types.
*/
package worldfile

import (
	"encoding/json"
	"fmt"
	"os"
)

// WorldFile is the top-level schema of the world data file.
type WorldFile struct {
	StartRoom string              `json:"start_room"`
	Rooms     map[string]RoomDef  `json:"rooms"`
	Items     map[string]ItemDef  `json:"items"`
	NPCs      map[string]NPCDef   `json:"npcs"`
	Quests    map[string]QuestDef `json:"quests"`
}

// RoomDef is the static definition of a room as it appears in the data file.
type RoomDef struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Exits       map[string]string `json:"exits"` // direction -> roomID
	Items       []string          `json:"items"` // itemIDs present at world start
	NPCs        []string          `json:"npcs"`  // npcIDs present at world start
}

// ItemDef is the static definition of an item.
type ItemDef struct {
	Name     string `json:"name"`
	Takeable bool   `json:"takeable"`
}

// NPCDef is the static definition of an NPC.
type NPCDef struct {
	Name     string `json:"name"`
	Role     string `json:"role"` // e.g. "merchant", "guard", "enemy"
	HP       int    `json:"hp"`
	Hostile  bool   `json:"hostile"`
	Dialogue string `json:"dialogue"`
	QuestID  string `json:"quest_id"` // empty if this NPC has no quest
}

// QuestDef is the static definition of a quest.
type QuestDef struct {
	Description string `json:"description"`
	Type        string `json:"type"`   // "fetch", "defeat", "deliver"
	TargetID    string `json:"target"` // itemID or npcID
	Reward      string `json:"reward"` // itemID granted on completion
}

// Load reads and parses the world data file at path.
// Returns a *WorldFile ready to be validated, or an error.
func Load(path string) (*WorldFile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read world file %q: %w", path, err)
	}

	return parseJSON(raw)
}

// parseJSON decodes raw JSON bytes into a WorldFile struct.
func parseJSON(data []byte) (*WorldFile, error) {
	var wf WorldFile
	//    [Initialization Statement]   ; [Condition Check]
	if err := json.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("parse world file JSON: %w", err)
	}

	return &wf, nil
}
