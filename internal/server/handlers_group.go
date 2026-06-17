// handlers_group.go: party management (GROUP CREATE / INVITE / JOIN / LEAVE).
package server

import (
	"strings"

	"the-answer-protocol/internal/protocol"
)

// handleGroup: GROUP <CREATE|INVITE|JOIN|LEAVE> [args] — routes to a subcommand.
func handleGroup(h *Hub, c *Client, args []string) string {
	if len(args) < 1 {
		return protocol.Errf(errBadRequest, "MISSING_SUBCOMMAND")
	}
	switch strings.ToUpper(args[0]) {
	case "CREATE":
		return handleGroupCreate(h, c, args[1:])
	case "INVITE":
		return handleGroupInvite(h, c, args[1:])
	case "JOIN":
		return handleGroupJoin(h, c, args[1:])
	case "LEAVE":
		return handleGroupLeave(h, c, args[1:])
	default:
		return protocol.Errf(errBadRequest, "BAD_SUBCOMMAND")
	}
}

// handleGroupCreate: GROUP CREATE — start a new group led by the caller.
func handleGroupCreate(h *Hub, c *Client, args []string) string {
	p := h.world.GetPlayer(c.username)
	if p.GroupID != "" {
		return protocol.Errf(protocol.ErrCodeAlreadyInGroup, protocol.MsgAlreadyInGroup)
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

// handleGroupInvite: GROUP INVITE <username> — the leader invites a player.
func handleGroupInvite(h *Hub, c *Client, args []string) string {
	p := h.world.GetPlayer(c.username)
	grp := h.groups[p.GroupID]
	if grp == nil {
		return protocol.Errf(protocol.ErrCodeNotInGroup, protocol.MsgNotInGroup)
	}
	if grp.Leader != c.username {
		return protocol.Errf(errBadRequest, "NOT_GROUP_LEADER")
	}
	if len(args) < 1 {
		return protocol.Errf(errBadRequest, "MISSING_USERNAME")
	}
	target := h.clientByUsername(args[0])
	if target == nil {
		return protocol.Errf(errBadRequest, "NO_SUCH_PLAYER")
	}
	if tp := h.world.GetPlayer(target.username); tp.GroupID != "" {
		return protocol.Errf(protocol.ErrCodeAlreadyInGroup, protocol.MsgAlreadyInGroup)
	}
	target.invitedGroup = grp.ID
	target.safeSend(protocol.GroupInvite(c.username))
	return protocol.OK("invited")
}

// handleGroupJoin: GROUP JOIN — accept a pending invite.
func handleGroupJoin(h *Hub, c *Client, args []string) string {
	p := h.world.GetPlayer(c.username)
	if p.GroupID != "" {
		return protocol.Errf(protocol.ErrCodeAlreadyInGroup, protocol.MsgAlreadyInGroup)
	}
	if c.invitedGroup == "" {
		return protocol.Errf(errBadRequest, "NO_INVITE")
	}
	grp := h.groups[c.invitedGroup]
	c.invitedGroup = ""
	if grp == nil {
		return protocol.Errf(errBadRequest, "GROUP_GONE")
	}
	grp.Members[c.username] = c
	p.GroupID = grp.ID
	h.broadcastGroup(grp, protocol.GroupJoin(c.username), c)
	h.log.Info("group joined", "user", c.username, "group", grp.ID)
	return protocol.OKf("group=%s", grp.ID)
}

// handleGroupLeave: GROUP LEAVE — leave the current group.
func handleGroupLeave(h *Hub, c *Client, args []string) string {
	p := h.world.GetPlayer(c.username)
	if p.GroupID == "" {
		return protocol.Errf(protocol.ErrCodeNotInGroup, protocol.MsgNotInGroup)
	}
	h.leaveGroup(c, p)
	h.log.Info("group left", "user", c.username)
	return protocol.OK("left")
}
