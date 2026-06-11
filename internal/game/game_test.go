// game_test.go: tests for the world and item operations.
package game

import "testing"

// buildTestWorld makes a tiny 2-room world by hand (no world file needed).
func buildTestWorld() *World {
	square := &Room{
		ID: "loc.square", Name: "Square",
		Exits:   map[string]string{"north": "loc.bakery"},
		Items:   map[string]*Item{"item.herbs": {ID: "item.herbs", Name: "Healing Herbs"}},
		NPCs:    map[string]*NPC{},
		Players: map[string]*Player{},
	}
	bakery := &Room{
		ID: "loc.bakery", Name: "Bakery",
		Exits:   map[string]string{"south": "loc.square"},
		Items:   map[string]*Item{},
		NPCs:    map[string]*NPC{},
		Players: map[string]*Player{},
	}
	return &World{
		Rooms:     map[string]*Room{"loc.square": square, "loc.bakery": bakery},
		Players:   map[string]*Player{},
		StartRoom: "loc.square",
		Quests:    map[string]*QuestDef{},
	}
}

func TestAddAndMovePlayer(t *testing.T) {
	w := buildTestWorld()

	alice, err := w.AddPlayer("alice")
	if err != nil {
		t.Fatalf("AddPlayer: unexpected error %v", err)
	}
	if alice.CurrentRoom != "loc.square" {
		t.Errorf("spawn room = %q, want loc.square", alice.CurrentRoom)
	}

	// duplicate name must fail
	if _, err := w.AddPlayer("alice"); err != ErrNameInUse {
		t.Errorf("duplicate AddPlayer err = %v, want ErrNameInUse", err)
	}

	// valid move
	dest, err := w.MovePlayer(alice, "north")
	if err != nil {
		t.Fatalf("MovePlayer north: %v", err)
	}
	if dest.ID != "loc.bakery" {
		t.Errorf("moved to %q, want loc.bakery", dest.ID)
	}
	if _, here := w.Rooms["loc.square"].Players["alice"]; here {
		t.Error("alice still listed in the old room")
	}

	// invalid direction
	if _, err := w.MovePlayer(alice, "west"); err != ErrNoExit {
		t.Errorf("MovePlayer west err = %v, want ErrNoExit", err)
	}
}

func TestTakeAndDropItem(t *testing.T) {
	w := buildTestWorld()
	alice, _ := w.AddPlayer("alice")
	square := w.Rooms["loc.square"]

	// take by display name (case-insensitive, multi-word)
	it, err := TakeItem(square, alice, "healing herbs")
	if err != nil {
		t.Fatalf("TakeItem: %v", err)
	}
	if it.ID != "item.herbs" {
		t.Errorf("took %q, want item.herbs", it.ID)
	}
	if _, stillThere := square.Items["item.herbs"]; stillThere {
		t.Error("item still on the floor after TakeItem")
	}

	// taking again must fail
	if _, err := TakeItem(square, alice, "item.herbs"); err != ErrItemNotFound {
		t.Errorf("second TakeItem err = %v, want ErrItemNotFound", err)
	}

	// drop by ID puts it back in the room
	if _, err := DropItem(alice, square, "item.herbs"); err != nil {
		t.Fatalf("DropItem: %v", err)
	}
	if _, back := square.Items["item.herbs"]; !back {
		t.Error("item not back on the floor after DropItem")
	}
}
