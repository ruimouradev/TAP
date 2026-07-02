// game_test.go: tests for the world and item operations.
package game

import "testing"

// buildTestWorld makes a tiny 2-room world by hand, no world file needed.
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
		Items:     map[string]*Item{},
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

func TestCombat(t *testing.T) {
	w := buildTestWorld()
	square := w.Rooms["loc.square"]
	square.NPCs["npc.goblin"] = &NPC{ID: "npc.goblin", Name: "Goblin", Hostile: true, HP: 1, MaxHP: 1}
	square.NPCs["npc.guard"] = &NPC{ID: "npc.guard", Name: "Guard", Hostile: false, HP: 30, MaxHP: 30}
	alice, _ := w.AddPlayer("alice")

	// attacking a non-hostile NPC is refused
	if _, err := Attack(alice, square, "Guard"); err != ErrNPCNotHostile {
		t.Errorf("attack guard err = %v, want ErrNPCNotHostile", err)
	}

	// attacking a missing NPC is refused
	if _, err := Attack(alice, square, "dragon"); err != ErrNPCNotFound {
		t.Errorf("attack dragon err = %v, want ErrNPCNotFound", err)
	}

	// one hit kills the 1-HP goblin -> victory, no counter-attack
	res, err := Attack(alice, square, "goblin")
	if err != nil {
		t.Fatalf("attack goblin: %v", err)
	}
	if !res.Defeated || res.Status != "victory" {
		t.Errorf("goblin result = %+v, want defeated victory", res)
	}
	if res.CounterDmg != 0 {
		t.Errorf("defeated NPC counter = %d, want 0", res.CounterDmg)
	}

	// the defeated NPC leaves the room (can't be hit again)
	if _, stillHere := square.NPCs["npc.goblin"]; stillHere {
		t.Error("defeated goblin still in the room")
	}
	if _, err := Attack(alice, square, "goblin"); err != ErrNPCNotFound {
		t.Errorf("re-attack defeated err = %v, want ErrNPCNotFound", err)
	}
}

func TestDefeatAndRespawn(t *testing.T) {
	w := buildTestWorld()
	square := w.Rooms["loc.square"]
	square.NPCs["npc.dragon"] = &NPC{ID: "npc.dragon", Name: "Dragon", Hostile: true, HP: 1000, MaxHP: 1000}
	alice, _ := w.AddPlayer("alice")
	alice.HP = 5 // any counter-attack (>= 10) finishes alice this round

	res, err := Attack(alice, square, "dragon")
	if err != nil {
		t.Fatalf("attack dragon: %v", err)
	}
	if !res.PlayerDied || res.Status != "defeat" {
		t.Fatalf("result = %+v, want player died, defeat", res)
	}

	RespawnPlayer(w, alice)
	if alice.HP != alice.MaxHP/2 {
		t.Errorf("respawn HP = %d, want %d", alice.HP, alice.MaxHP/2)
	}
	if alice.CurrentRoom != w.StartRoom {
		t.Errorf("respawn room = %q, want %q", alice.CurrentRoom, w.StartRoom)
	}
}

func TestQuests(t *testing.T) {
	w := buildTestWorld()
	w.Items["item.herbs"] = &Item{ID: "item.herbs", Name: "Healing Herbs"}
	w.Items["item.gold"] = &Item{ID: "item.gold", Name: "Gold"}
	w.Quests["quest.herbs"] = &QuestDef{
		ID: "quest.herbs", Description: "Bring herbs", Type: "fetch",
		TargetID: "item.herbs", Reward: "item.gold",
	}
	square := w.Rooms["loc.square"]
	square.NPCs["npc.elder"] = &NPC{ID: "npc.elder", Name: "Elder", Role: "quest-giver", QuestID: "quest.herbs"}
	square.NPCs["npc.dog"] = &NPC{ID: "npc.dog", Name: "Dog"} // no quest

	alice, _ := w.AddPlayer("alice")

	// an NPC without a quest -> ErrNoQuestAvailable
	if _, err := GetQuestFromNPC(w, alice, square, "Dog"); err != ErrNoQuestAvailable {
		t.Errorf("dog quest err = %v, want ErrNoQuestAvailable", err)
	}

	// get and start the elder's quest
	def, err := GetQuestFromNPC(w, alice, square, "Elder")
	if err != nil {
		t.Fatalf("GetQuestFromNPC: %v", err)
	}
	StartQuest(alice, def)
	pq := alice.Quests["quest.herbs"]
	if pq == nil || pq.State != QuestActive {
		t.Fatalf("quest not active after StartQuest")
	}

	// asking again is refused (already taken)
	if _, err := GetQuestFromNPC(w, alice, square, "Elder"); err != ErrNoQuestAvailable {
		t.Errorf("second request err = %v, want ErrNoQuestAvailable", err)
	}

	// not complete until the player holds the herbs
	if CheckCompletion(alice, pq) {
		t.Error("quest reported complete without the item")
	}
	alice.Inventory["item.herbs"] = &Item{ID: "item.herbs", Name: "Healing Herbs"}
	if !CheckCompletion(alice, pq) {
		t.Error("quest not complete with the item in inventory")
	}

	// completing it grants the reward and flips the state
	CompleteQuest(w, alice, pq)
	if pq.State != QuestCompleted {
		t.Error("quest state not Completed after CompleteQuest")
	}
	if _, ok := alice.Inventory["item.gold"]; !ok {
		t.Error("reward item.gold not granted")
	}
}
