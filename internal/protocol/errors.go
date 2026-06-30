// errors.go: RFC 42TAP errors as typed values. Bundling the numeric code and
// its message token in one value means a code can never drift from its message -
// there is a single source of truth per error.
package protocol

import "fmt"

// Error is a protocol error: an RFC code plus its stable message token.
type Error struct {
	Code int
	Msg  string
}

// Wire formats the error as a protocol line: "ERR <code> <message>\n".
// %03d because RFC error codes are always three digits.
func (e Error) Wire() string {
	return fmt.Sprintf("ERR %03d %s\n", e.Code, e.Msg)
}

// Standard errors from RFC 42TAP section 8.2.
var (
	ErrNameInUse        = Error{201, "NAME_IN_USE"}
	ErrNoExit           = Error{301, "NO_EXIT"}
	ErrNotInGroup       = Error{401, "NOT_IN_GROUP"}
	ErrAlreadyInGroup   = Error{402, "ALREADY_IN_GROUP"}
	ErrItemNotFound     = Error{404, "ITEM_NOT_FOUND"}
	ErrItemNotInInv     = Error{404, "ITEM_NOT_IN_INVENTORY"}
	ErrNPCNotFound      = Error{404, "NPC_NOT_FOUND"}
	ErrNPCNotHostile    = Error{405, "NPC_NOT_HOSTILE"}
	ErrNoQuestAvailable = Error{406, "NO_QUEST_AVAILABLE"}
	ErrSendFailed       = Error{901, "SEND_FAILED"}
)

// BadRequest builds a 400 error for input the RFC leaves undefined (unknown
// verb, not connected, missing/invalid arguments). 400 is our documented
// extension - see the README Protocol Implementation section.
func BadRequest(msg string) Error {
	return Error{400, msg}
}
