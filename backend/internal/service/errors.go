package service

import (
	"errors"
	"fmt"
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

// EvidenceGateError rejects a release transition while preserving the current
// status. Blockers lists every failed evidence check so the UI can show the
// exact 阻断项 that must be resolved before resubmitting.
type EvidenceGateError struct {
	Blockers []string
}

func (e *EvidenceGateError) Error() string {
	return fmt.Sprintf("放行前证据核验未通过，存在 %d 项阻断", len(e.Blockers))
}
