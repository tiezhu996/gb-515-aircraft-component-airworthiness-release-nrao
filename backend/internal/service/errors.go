package service

import (
	"errors"
	"strings"
)

var (
	ErrInvalidTransition = errors.New("requested status transition is not allowed")
	ErrInvalidInput      = errors.New("business input validation failed")
	ErrUnauthorized      = errors.New("invalid username or password")
	ErrInactiveUser      = errors.New("user account is inactive")
	ErrForbidden         = errors.New("role is not permitted for this operation")
	ErrLocked            = errors.New("record is locked after review begins")
	ErrSeparationOfDuty  = errors.New("preparer and reviewer must be different users")
)

// EvidenceGateError rejects a release transition while keeping the current
// status. Blockers lists every unmet evidence condition so callers can show
// the operator exactly what must be fixed before resubmitting.
type EvidenceGateError struct {
	Blockers []string
}

func (e *EvidenceGateError) Error() string {
	return "放行前证据闸门未通过: " + strings.Join(e.Blockers, "；")
}
