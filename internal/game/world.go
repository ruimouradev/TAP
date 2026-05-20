/*
Package game — Owner: Rui.

File world.go: core domain model — World, Room, Player, and NPC types,
plus all world-level operations (move players, add/remove players, queries).

These structs are mutated ONLY by the Hub goroutine — no mutexes needed.
Static world data (room layout, NPC definitions) is set at startup and
never changed at runtime; mutable state (player positions, HP, inventory)
lives inside Players and Rooms.
*/
package game

import "the-answer-protocol/internal/worldfile"

// World is the top-level mutable game state.
// Only the Hub goroutine touches this — no mutexes required.
type World struct {
	Rooms     map[string]*Room   // roomID → Room
	Players   map[string]*Player // username → Player
	StartRoom string             // roomID where new players spawn
	Quests    map[string]*QuestDef // questID → static quest definition
}

// Room is a discrete location in the virtual world.
type Room struct {
	ID          string
	Name        string
	Description string
	Exits       map[string]string  // direction → destination roomID
	Items       map[string]*Item   // itemID → Item (on the floor)
	NPCs        map[string]*NPC    // npcID → NPC
	Players     map[string]*Player // username → Player (currently here)
}

// Player represents a connected, authenticated player in the world.
type Player struct {
	Username    string
	CurrentRoom string               // roomID
	HP          int
	MaxHP       int
	Inventory   map[string]*Item     // itemID → Item
	Quests      map[string]*PlayerQuest
	GroupID     string               // empty if not in a group
}

// NPC represents a non-player character residing in a room.
type NPC struct {
	ID       string
	Name     string
	Role     string // e.g. "merchant", "guard", "enemy"
	HP       int
	MaxHP    int
	Hostile  bool
	Dialogue string
	QuestID  string // empty if this NPC offers no quest
}

// NewWorld builds a live World from a loaded and validated WorldFile.
// Converts static file definitions into mutable runtime structs.
func NewWorld(wf *worldfile.WorldFile) *World {
	return nil // TODO: implement
}

// GetRoom returns the room with the given ID, or nil if it does not exist.
func (w *World) GetRoom(id string) *Room {
	return nil // TODO: implement
}

// GetPlayer returns the player with the given username, or nil.
func (w *World) GetPlayer(username string) *Player {
	return nil // TODO: implement
}

// AddPlayer adds a new player to the world at the start room.
// Returns an error if the username is already taken.
func (w *World) AddPlayer(username string) (*Player, error) {
	return nil, nil // TODO: implement
}

// RemovePlayer removes the player from their current room and from the world map.
func (w *World) RemovePlayer(username string) {
	// TODO: implement
}

// MovePlayer moves player in the given direction.
// Returns the new room, or an error if the direction does not exist.
func (w *World) MovePlayer(player *Player, direction string) (*Room, error) {
	return nil, nil // TODO: implement
}

// PlayersInRoom returns a slice of usernames of all players currently in roomID.
func (w *World) PlayersInRoom(roomID string) []string {
	return nil // TODO: implement
}

// TotalPlayers returns the number of currently connected players.
func (w *World) TotalPlayers() int {
	return 0 // TODO: implement
}

// NPCInRoom finds an NPC in room by display name (case-insensitive).
// Returns nil, false if not found.
func (w *World) NPCInRoom(room *Room, name string) (*NPC, bool) {
	return nil, false // TODO: implement
}
