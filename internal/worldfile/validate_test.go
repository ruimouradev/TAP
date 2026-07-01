package worldfile

import (
	"fmt"
	"strings"
	"testing"
)

func baseWorld() *WorldFile {
	rooms := map[string]RoomDef{}
	// create 8 rooms with minimal sensible links
	for i := 0; i < 8; i++ {
		id := fmt.Sprintf("r%d", i)
		rooms[id] = RoomDef{Name: id, Description: "desc", Exits: map[string]string{}, Items: []string{}, NPCs: []string{}}
	}
	// link r0 <-> r1
	r0 := rooms["r0"]
	r0.Exits = map[string]string{"north": "r1"}
	rooms["r0"] = r0
	r1 := rooms["r1"]
	r1.Exits = map[string]string{"south": "r0"}
	rooms["r1"] = r1

	items := map[string]ItemDef{
		"it1": {Name: "one", Takeable: true},
		"it2": {Name: "two", Takeable: true},
		"it3": {Name: "three", Takeable: false},
		"it4": {Name: "four", Takeable: false},
	}

	npcs := map[string]NPCDef{
		"npc1": {Name: "A", Role: "guard", HP: 10, Hostile: false, Dialogue: "hi", QuestID: "q1"},
		"npc2": {Name: "B", Role: "merchant", HP: 10, Hostile: false, Dialogue: "sell", QuestID: ""},
		"npc3": {Name: "C", Role: "enemy", HP: 10, Hostile: true, Dialogue: "grr", QuestID: ""},
	}

	quests := map[string]QuestDef{
		"q1": {Description: "fetch", Type: "fetch", TargetID: "it1", Reward: "it2"},
		"q2": {Description: "defeat", Type: "defeat", TargetID: "npc3", Reward: "it3"},
	}

	// place item and npc in r0
	r0 = rooms["r0"]
	r0.Items = []string{"it1"}
	r0.NPCs = []string{"npc1"}
	rooms["r0"] = r0

	return &WorldFile{
		StartRoom: "r0",
		Rooms:     rooms,
		Items:     items,
		NPCs:      npcs,
		Quests:    quests,
	}
}

func TestValidate_Happy(t *testing.T) {
	wf := baseWorld()
	if err := Validate(wf); err != nil {
		t.Fatalf("expected valid world, got error: %v", err)
	}
}

func TestValidate_Errors(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*WorldFile)
		want string
	}{
		{"missing start", func(w *WorldFile) { w.StartRoom = "" }, "start_room"},
		{"unknown exit", func(w *WorldFile) { r := w.Rooms["r0"]; r.Exits["east"] = "nope"; w.Rooms["r0"] = r }, "unknown room"},
		{"unknown item ref", func(w *WorldFile) { r := w.Rooms["r0"]; r.Items = []string{"baditem"}; w.Rooms["r0"] = r }, "unknown item"},
		{"unknown npc ref", func(w *WorldFile) { r := w.Rooms["r0"]; r.NPCs = []string{"badnpc"}; w.Rooms["r0"] = r }, "unknown npc"},
		{"unknown quest ref", func(w *WorldFile) { n := w.NPCs["npc1"]; n.QuestID = "badq"; w.NPCs["npc1"] = n }, "unknown quest"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wf := baseWorld()
			c.mut(wf)
			err := Validate(wf)
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(c.want)) {
				t.Fatalf("case %q: expected error containing %q, got %v", c.name, c.want, err)
			}
		})
	}
}

func TestValidate_Minimums(t *testing.T) {
	// rooms < 8
	wf := baseWorld()
	// keep a single room but clear exits/items/npcs so exit checks don't fail first
	single := wf.Rooms["r0"]
	single.Exits = map[string]string{}
	single.Items = []string{}
	single.NPCs = []string{}
	wf.Rooms = map[string]RoomDef{"r0": single}
	if err := Validate(wf); err == nil || !strings.Contains(err.Error(), "at least 8 rooms") {
		t.Fatalf("expected rooms-minimum error, got %v", err)
	}

	// roles < 3
	wf = baseWorld()
	wf.NPCs = map[string]NPCDef{"npc1": {Name: "A", Role: "guard"}}
	wf.Quests = map[string]QuestDef{}
	if err := Validate(wf); err == nil || !strings.Contains(err.Error(), "3 distinct NPC roles") {
		t.Fatalf("expected roles-minimum error, got %v", err)
	}

	// items < 4
	wf = baseWorld()
	wf.Items = map[string]ItemDef{"it1": {Name: "one", Takeable: true}, "it2": {Name: "two", Takeable: true}, "it3": {Name: "three", Takeable: false}}
	if err := Validate(wf); err == nil || !strings.Contains(err.Error(), "at least 4 items") {
		t.Fatalf("expected items-minimum error, got %v", err)
	}

	// takeable < 2
	wf = baseWorld()
	wf.Items = map[string]ItemDef{"it1": {Name: "one", Takeable: true}, "it2": {Name: "two", Takeable: false}, "it3": {Name: "three", Takeable: false}, "it4": {Name: "four", Takeable: false}}
	if err := Validate(wf); err == nil || !strings.Contains(err.Error(), "2 takeable items") {
		t.Fatalf("expected takeable-minimum error, got %v", err)
	}

	// quests < 2
	wf = baseWorld()
	wf.Quests = map[string]QuestDef{"q1": wf.Quests["q1"]}
	if err := Validate(wf); err == nil || !strings.Contains(err.Error(), "at least 2 quests") {
		t.Fatalf("expected quests-minimum error, got %v", err)
	}
}
