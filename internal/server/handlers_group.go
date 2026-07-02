// handlers_group.go: party management. GROUP CREATE, INVITE, JOIN and LEAVE.
package server

import (
	"strings"

	"the-answer-protocol/internal/game"
	"the-answer-protocol/internal/protocol"
)

// handleGroup: GROUP <CREATE|INVITE|JOIN|LEAVE> [args]. Routes to a subcommand.
func handleGroup(h *Hub, c *Client, p *game.Player, args []string) string {
	switch strings.ToUpper(args[0]) {
	case "CREATE":
		return handleGroupCreate(h, c, p, args[1:])
	case "INVITE":
		return handleGroupInvite(h, c, p, args[1:])
	case "JOIN":
		return handleGroupJoin(h, c, p, args[1:])
	case "LEAVE":
		return handleGroupLeave(h, c, p, args[1:])
	default:
		return protocol.BadRequest("BAD_SUBCOMMAND").Wire()
	}
}

// handleGroupCreate: GROUP CREATE. Starts a new group led by the caller.
func handleGroupCreate(h *Hub, c *Client, p *game.Player, args []string) string {
	if p.GroupID != "" {
		return protocol.ErrAlreadyInGroup.Wire()
	}
	h.groups[c.username] = &Group{
		ID:      c.username,
		Leader:  c.username,
		Members: map[string]*Client{c.username: c},
	}
	p.GroupID = c.username
	h.log.Info("group created", "user", c.username, "group", c.username)
	return protocol.OKf("group=%s", c.username)
}

// handleGroupInvite: GROUP INVITE <username>. The leader invites a player.
func handleGroupInvite(h *Hub, c *Client, p *game.Player, args []string) string {
	grp := h.groups[p.GroupID]
	if grp == nil {
		return protocol.ErrNotInGroup.Wire()
	}
	if grp.Leader != c.username {
		return protocol.BadRequest("NOT_GROUP_LEADER").Wire()
	}
	if len(args) < 1 {
		return protocol.BadRequest("MISSING_USERNAME").Wire()
	}
	target := h.clientByUsername(args[0])
	if target == nil {
		return protocol.BadRequest("NO_SUCH_PLAYER").Wire()
	}
	if tp := h.world.GetPlayer(target.username); tp.GroupID != "" {
		return protocol.ErrAlreadyInGroup.Wire()
	}
	target.invitedGroup = grp.ID
	target.safeSend(protocol.GroupInvite(c.username))
	return protocol.OK("invited")
}

// handleGroupJoin: GROUP JOIN. Accepts a pending invite.
func handleGroupJoin(h *Hub, c *Client, p *game.Player, args []string) string {
	if p.GroupID != "" {
		return protocol.ErrAlreadyInGroup.Wire()
	}
	if c.invitedGroup == "" {
		return protocol.BadRequest("NO_INVITE").Wire()
	}
	grp := h.groups[c.invitedGroup]
	c.invitedGroup = ""
	if grp == nil {
		return protocol.BadRequest("GROUP_GONE").Wire()
	}
	grp.Members[c.username] = c
	p.GroupID = grp.ID
	h.broadcastGroup(grp, protocol.GroupJoin(c.username), c)
	h.log.Info("group joined", "user", c.username, "group", grp.ID)
	return protocol.OKf("group=%s", grp.ID)
}

// handleGroupLeave: GROUP LEAVE. Leaves the current group.
func handleGroupLeave(h *Hub, c *Client, p *game.Player, args []string) string {
	if p.GroupID == "" {
		return protocol.ErrNotInGroup.Wire()
	}
	h.leaveGroup(c, p)
	h.log.Info("group left", "user", c.username)
	return protocol.OK("left")
}
