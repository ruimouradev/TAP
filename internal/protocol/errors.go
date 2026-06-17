// errors.go: RFC 42TAP error codes and messages.
package protocol

// Error codes defined by RFC 42TAP section 8.2.
const (
	ErrCodeNameInUse        = 201
	ErrCodeNoExit           = 301
	ErrCodeNotInGroup       = 401
	ErrCodeAlreadyInGroup   = 402
	ErrCodeItemNotFound     = 404
	ErrCodeNPCNotFound      = 404
	ErrCodeNPCNotHostile    = 405
	ErrCodeNoQuestAvailable = 406
	ErrCodeSendFailed       = 901
)

// Standard error message strings matching the RFC.
const (
	MsgNameInUse        = "NAME_IN_USE"
	MsgNoExit           = "NO_EXIT"
	MsgNotInGroup       = "NOT_IN_GROUP"
	MsgAlreadyInGroup   = "ALREADY_IN_GROUP"
	MsgItemNotFound     = "ITEM_NOT_FOUND"
	MsgItemNotInInv     = "ITEM_NOT_IN_INVENTORY"
	MsgNPCNotFound      = "NPC_NOT_FOUND"
	MsgNPCNotHostile    = "NPC_NOT_HOSTILE"
	MsgNoQuestAvailable = "NO_QUEST_AVAILABLE"
	MsgSendFailed       = "SEND_FAILED"
)
