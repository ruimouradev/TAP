// handlers_quest.go: quest commands (QUEST / QUESTS).
package server

import (
	"strings"

	"the-answer-protocol/internal/game"
	"the-answer-protocol/internal/protocol"
)

// handleQuest: QUEST <npc> — request a quest from a quest-giver NPC.
func handleQuest(h *Hub, c *Client, args []string) string {
	if len(args) < 1 {
		return protocol.Errf(errBadRequest, "MISSING_NPC")
	}
	p := h.world.GetPlayer(c.username)
	room := h.world.GetRoom(p.CurrentRoom)
	def, err := game.GetQuestFromNPC(h.world, p, room, strings.Join(args, " "))
	if err != nil {
		return protocol.Errf(protocol.ErrCodeNoQuestAvailable, protocol.MsgNoQuestAvailable)
	}
	game.StartQuest(p, def)
	h.log.Info("quest started", "user", c.username, "quest", def.ID)
	return protocol.OKJson(protocol.QuestResponse{
		QuestID:     def.ID,
		Description: def.Description,
		Type:        def.Type,
		Target:      def.TargetID,
		Reward:      def.Reward,
	})
}

// handleQuests: QUESTS — list the player's quests. Any active quest whose
// objective is now met is completed here (and its reward granted) before listing.
func handleQuests(h *Hub, c *Client, args []string) string {
	p := h.world.GetPlayer(c.username)
	entries := make([]protocol.QuestsEntry, 0, len(p.Quests))
	for _, pq := range game.ListQuests(p) {
		if pq.State == game.QuestActive && game.CheckCompletion(p, pq) {
			game.CompleteQuest(h.world, p, pq)
			h.log.Info("quest completed", "user", c.username, "quest", pq.Def.ID, "reward", pq.Def.Reward)
		}
		entries = append(entries, protocol.QuestsEntry{
			QuestID:     pq.Def.ID,
			Description: pq.Def.Description,
			State:       questStateLabel(pq.State),
		})
	}
	return protocol.OKJson(entries)
}

// questStateLabel maps the internal quest state to the wire string.
func questStateLabel(s game.QuestState) string {
	if s == game.QuestCompleted {
		return "completed"
	}
	return "active"
}
