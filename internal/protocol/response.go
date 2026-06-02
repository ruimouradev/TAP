/*
Package protocol — Owner: BOTH (Rui + Alexandre).

File response.go: helper functions for formatting OK and ERR responses
that the server sends back to a client after processing a command.

Wire formats:

	OK [data]\n
	ERR <code> <message>\n
*/
package protocol

import (
	"encoding/json"
	"fmt"
)

// OK formats a success response with optional payload data.
// If data is empty, returns "OK\n".
func OK(data string) string {
	if data == "" {
		return "OK\n"
	}
	return "OK " + data + "\n"
}

// OKf formats a success response using fmt.Sprintf-style formatting.
// Útil para respostas de texto simples como OKf("room=%s", roomID).
func OKf(format string, args ...any) string {
	return OK(fmt.Sprintf(format, args...))
}

// OKJson marshals v to JSON and wraps it in an OK response line.
// Returns an ERR response line if marshalling fails.
func OKJson(v any) string {
	// json.Marshal só falha com tipos impossíveis de serializar (canais,
	// funções). Os nossos structs de resposta nunca têm esses campos, por isso
	// na prática este erro não acontece — mas tratamo-lo na mesma, em vez de
	// ignorar com "_", para o servidor nunca enviar uma linha meia-feita.
	b, err := json.Marshal(v)
	if err != nil {
		return Errf(ErrCodeSendFailed, MsgSendFailed)
	}
	return OK(string(b))
}

// Errf formats an error response: "ERR <code> <message>\n".
func Errf(code int, msg string) string {
	// O ABNF do RFC define error-code = 3DIGIT. Os nossos códigos já têm 3
	// dígitos (201..901), mas %03d garante o formato mesmo que algum dia surja
	// um código mais pequeno — fica sempre conforme o ABNF.
	return fmt.Sprintf("ERR %03d %s\n", code, msg)
}

// ── Response structs (shared contract between server handlers and GUI) ────────
//
// Rui: marshal these with OKJson() in dispatch.go.
// Alexandre: parse these fields in cmd/gui/main.go applyResponse().

// LookResponse is the JSON payload of LOOK.
// Deviation from RFC §5.1.2: RFC lists items/npcs as ID strings.
// We use objects so the GUI can show names and hostile status without extra calls.
type LookResponse struct {
	Room    LookRoom   `json:"room"`
	Players []string   `json:"players"`
	Items   []LookItem `json:"items"`
	NPCs    []LookNPC  `json:"npcs"`
}

type LookRoom struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Exits       map[string]string `json:"exits"`
}

type LookItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type LookNPC struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Hostile bool   `json:"hostile"`
}

// WhoResponse is the JSON payload of WHO.
// Deviation from RFC §5.2.2: RFC says "OK players=<count>" (plain text).
// We return JSON so the GUI shows room players and server total together.
type WhoResponse struct {
	Room   []string `json:"room"`
	Server int      `json:"server"`
}

// InventoryItem is one element in the INVENTORY JSON array.
// Deviation from RFC §5.4.3: RFC example shows an array of ID strings.
// We return objects so the GUI can show item names without a secondary lookup.
type InventoryItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// TalkResponse is the JSON payload of TALK.
// Deviation from RFC §5.4.4: RFC shows plain-text dialogue ("OK Welcome!").
// We return JSON so the GUI can display the NPC name alongside the dialogue.
type TalkResponse struct {
	NPC      string `json:"npc"`
	Dialogue string `json:"dialogue"`
}

// AttackResponse is the JSON payload of ATTACK.
// Extends RFC §5.4.5 with Counter so the GUI shows NPC counter-attack damage.
type AttackResponse struct {
	Damage     int    `json:"damage"`
	Counter    int    `json:"counter"` // counter-attack damage to player (0 if NPC defeated)
	AttackerHP int    `json:"attacker_hp"`
	TargetHP   int    `json:"target_hp"`
	Status     string `json:"status"` // "combat" | "victory" | "defeat"
}

// StatusResponse is the JSON payload of STATUS. Matches RFC §5.4.6 exactly.
type StatusResponse struct {
	HP     int    `json:"hp"`
	MaxHP  int    `json:"max_hp"`
	Status string `json:"status"` // "healthy" | "wounded" | "dead"
}

// QuestResponse is the JSON payload of QUEST. Matches RFC §5.4.7 example.
type QuestResponse struct {
	QuestID     string `json:"quest_id"`
	Description string `json:"description"`
	Type        string `json:"type"`   // "fetch" | "defeat" | "deliver"
	Target      string `json:"target"` // itemID or npcID
	Reward      string `json:"reward"` // itemID
}

// QuestsEntry is one element in the QUESTS JSON array. Matches RFC §5.4.8 example.
type QuestsEntry struct {
	QuestID     string `json:"quest_id"`
	Description string `json:"description"`
	State       string `json:"status"` // "active" | "completed"
}
