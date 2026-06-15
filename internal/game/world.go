// world.go: the core domain model (World, Room, Player, NPC) and the
// world-level operations. Only the Hub goroutine mutates these — no locks.
package game

import (
	"errors"
	"strings"

	"the-answer-protocol/internal/worldfile"
)

// World is the top-level mutable game state.
type World struct {
	Rooms     map[string]*Room     // roomID → Room
	Players   map[string]*Player   // username → Player
	StartRoom string               // where new players spawn
	Quests    map[string]*QuestDef // questID → static definition
}

// Room is a location in the world.
type Room struct {
	ID          string
	Name        string
	Description string
	Exits       map[string]string  // direction → destination roomID
	Items       map[string]*Item   // itemID → Item on the floor
	NPCs        map[string]*NPC    // npcID → NPC
	Players     map[string]*Player // username → Player currently here
}

// Player is a connected, authenticated player.
type Player struct {
	Username    string
	CurrentRoom string
	HP          int
	MaxHP       int
	Inventory   map[string]*Item
	Quests      map[string]*PlayerQuest
	GroupID     string // empty if not in a group
}

// NPC is a non-player character living in a room.
type NPC struct {
	ID       string
	Name     string
	Role     string // "merchant", "guard", "enemy", ...
	HP       int
	MaxHP    int
	Hostile  bool
	Dialogue string
	QuestID  string // empty if it offers no quest
}

var (
	ErrNameInUse = errors.New("name in use")
	ErrNoExit    = errors.New("no such exit")
)

// NewWorld builds a live World from a parsed world file, turning the static
// definitions into mutable runtime structs.
func NewWorld(wf *worldfile.WorldFile) *World {
	w := &World{
		Rooms:     make(map[string]*Room, len(wf.Rooms)),
		Players:   make(map[string]*Player),
		StartRoom: wf.StartRoom,
		Quests:    make(map[string]*QuestDef, len(wf.Quests)),
	}

	for id, q := range wf.Quests {
		w.Quests[id] = &QuestDef{
			ID:          id,
			Description: q.Description,
			Type:        q.Type,
			TargetID:    q.TargetID,
			Reward:      q.Reward,
		}
	}

	for id, rd := range wf.Rooms {
		room := &Room{
			ID:          id,
			Name:        rd.Name,
			Description: rd.Description,
			Exits:       rd.Exits,
			Items:       make(map[string]*Item),
			NPCs:        make(map[string]*NPC),
			Players:     make(map[string]*Player),
		}
		for _, itemID := range rd.Items {
			if def, ok := wf.Items[itemID]; ok {
				room.Items[itemID] = &Item{ID: itemID, Name: def.Name}
			}
		}
		for _, npcID := range rd.NPCs {
			if def, ok := wf.NPCs[npcID]; ok {
				room.NPCs[npcID] = &NPC{
					ID:       npcID,
					Name:     def.Name,
					Role:     def.Role,
					HP:       def.HP,
					MaxHP:    def.HP,
					Hostile:  def.Hostile,
					Dialogue: def.Dialogue,
					QuestID:  def.QuestID,
				}
			}
		}
		w.Rooms[id] = room
	}
	return w
}

// GetRoom returns the room with id, or nil.
func (w *World) GetRoom(id string) *Room { return w.Rooms[id] }

// GetPlayer returns the player with username, or nil.
func (w *World) GetPlayer(username string) *Player { return w.Players[username] }

// AddPlayer spawns a new player at the start room. Fails if the name is taken.
func (w *World) AddPlayer(username string) (*Player, error) {
	if _, taken := w.Players[username]; taken {
		return nil, ErrNameInUse
	}
	p := &Player{
		Username:    username,
		CurrentRoom: w.StartRoom,
		HP:          100,
		MaxHP:       100,
		Inventory:   make(map[string]*Item),
		Quests:      make(map[string]*PlayerQuest),
	}
	w.Players[username] = p
	if room := w.Rooms[w.StartRoom]; room != nil {
		room.Players[username] = p
	}
	return p, nil
}

// RemovePlayer takes the player out of their room and the world.
func (w *World) RemovePlayer(username string) {
	p := w.Players[username]
	if p == nil {
		return
	}
	if room := w.Rooms[p.CurrentRoom]; room != nil {
		delete(room.Players, username)
	}
	delete(w.Players, username)
}

// MovePlayer moves the player one step in direction. Returns the new room,
// or ErrNoExit if there is no exit that way.
func (w *World) MovePlayer(player *Player, direction string) (*Room, error) {
	here := w.Rooms[player.CurrentRoom]
	if here == nil {
		return nil, ErrNoExit
	}
	destID, ok := here.Exits[direction]
	if !ok {
		return nil, ErrNoExit
	}
	dest := w.Rooms[destID]
	if dest == nil {
		return nil, ErrNoExit
	}
	delete(here.Players, player.Username)
	dest.Players[player.Username] = player
	player.CurrentRoom = destID
	return dest, nil
}

// PlayersInRoom lists the usernames currently in roomID.
func (w *World) PlayersInRoom(roomID string) []string {
	room := w.Rooms[roomID]
	if room == nil {
		return nil
	}
	names := make([]string, 0, len(room.Players))
	for name := range room.Players {
		names = append(names, name)
	}
	return names
}

// TotalPlayers is the number of connected players.
func (w *World) TotalPlayers() int { return len(w.Players) }

// NPCInRoom finds an NPC in room by ID or display name (case-insensitive).
func (w *World) NPCInRoom(room *Room, name string) (*NPC, bool) {
	for _, npc := range room.NPCs {
		if strings.EqualFold(npc.ID, name) || strings.EqualFold(npc.Name, name) {
			return npc, true
		}
	}
	return nil, false
}
