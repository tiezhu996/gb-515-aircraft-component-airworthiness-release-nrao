package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/blueship581/aircraft-component-airworthiness-release/backend/internal/dto"
	"github.com/blueship581/aircraft-component-airworthiness-release/backend/internal/model"
	"github.com/blueship581/aircraft-component-airworthiness-release/backend/internal/repository"
	"gorm.io/gorm"
)

func TestEvidenceGateBlocksSubmitUntilEvidenceReady(t *testing.T) {
	db := newVersionTestDB(t)
	service := newReleaseAuthorizationService(db)
	ctx := context.Background()

	created, err := service.Create(ctx, authorizationInput("AUTH-GATE-01"), "operator", "gate-create-1")
	if err != nil {
		t.Fatalf("create authorization: %v", err)
	}
	submit := dto.TransitionRequest{Status: "review", ExpectedVersion: created.Version, Reason: "requesting release review"}

	_, err = service.Transition(ctx, created.ID, submit, "operator", model.RoleOperator, "gate-submit-denied")
	blockers := assertEvidenceGateError(t, err, 3)
	for _, keyword := range []string{"关联部件", "检查任务", "证书记录"} {
		if !blockersContain(blockers, keyword) {
			t.Fatalf("expected blocker mentioning %q, got %v", keyword, blockers)
		}
	}
	assertAuthorizationState(t, service, created.ID, "draft", 1, 1)

	// Seed evidence in the wrong states: every condition must be reported.
	seedReleaseEvidence(t, db, created.RelatedCode)
	setPartStatus(t, db, created.RelatedCode, "inspection")
	setInspectionStatus(t, db, created.RelatedCode, "running")
	setCertificateState(t, db, created.RelatedCode, "expired", time.Now().UTC().Add(48*time.Hour))

	_, err = service.Transition(ctx, created.ID, submit, "operator", model.RoleOperator, "gate-submit-denied-2")
	blockers = assertEvidenceGateError(t, err, 4)
	for _, keyword := range []string{"hold", "passed", "valid", "有效期"} {
		if !blockersContain(blockers, keyword) {
			t.Fatalf("expected blocker mentioning %q, got %v", keyword, blockers)
		}
	}
	assertAuthorizationState(t, service, created.ID, "draft", 1, 1)

	// Bring every condition into shape and the same submit succeeds.
	setPartStatus(t, db, created.RelatedCode, "hold")
	setInspectionStatus(t, db, created.RelatedCode, "passed")
	setCertificateState(t, db, created.RelatedCode, "valid", time.Now().UTC().Add(-time.Hour))

	review, err := service.Transition(ctx, created.ID, submit, "operator", model.RoleOperator, "gate-submit-3")
	if err != nil {
		t.Fatalf("submit with complete evidence: %v", err)
	}
	if review.Status != "review" || review.Version != 2 || review.SubmittedBy != "operator" {
		t.Fatalf("unexpected review state: %#v", review)
	}
}

func TestEvidenceGateRechecksEvidenceAtApproval(t *testing.T) {
	db := newVersionTestDB(t)
	service := newReleaseAuthorizationService(db)
	ctx := context.Background()

	created, err := service.Create(ctx, authorizationInput("AUTH-GATE-02"), "operator", "gate2-create-1")
	if err != nil {
		t.Fatalf("create authorization: %v", err)
	}
	seedReleaseEvidence(t, db, created.RelatedCode)
	review, err := service.Transition(ctx, created.ID, dto.TransitionRequest{
		Status: "review", ExpectedVersion: created.Version, Reason: "evidence complete",
	}, "operator", model.RoleOperator, "gate2-submit-2")
	if err != nil {
		t.Fatalf("submit authorization: %v", err)
	}

	// Evidence drifts while the authorization waits in review: the part
	// leaves hold, so the approval must be blocked and the status kept.
	setPartStatus(t, db, created.RelatedCode, "inspection")
	approval := dto.TransitionRequest{Status: "approved", ExpectedVersion: review.Version, Reason: "release review passed"}
	_, err = service.Transition(ctx, review.ID, approval, "reviewer", model.RoleReviewer, "gate2-approve-denied")
	blockers := assertEvidenceGateError(t, err, 1)
	if !blockersContain(blockers, "hold") {
		t.Fatalf("expected hold blocker, got %v", blockers)
	}
	assertAuthorizationState(t, service, created.ID, "review", 2, 2)

	// Restoring the evidence lets the same approval through.
	setPartStatus(t, db, created.RelatedCode, "hold")
	approved, err := service.Transition(ctx, review.ID, approval, "reviewer", model.RoleReviewer, "gate2-approve-3")
	if err != nil {
		t.Fatalf("approve with restored evidence: %v", err)
	}
	if approved.Status != "approved" || approved.Version != 3 || approved.ReviewedBy != "reviewer" {
		t.Fatalf("unexpected approved state: %#v", approved)
	}
}

func TestEvidenceGateLeavesOtherTransitionsUntouched(t *testing.T) {
	db := newVersionTestDB(t)
	service := newReleaseAuthorizationService(db)
	ctx := context.Background()

	created, err := service.Create(ctx, authorizationInput("AUTH-GATE-03"), "operator", "gate3-create-1")
	if err != nil {
		t.Fatalf("create authorization: %v", err)
	}
	seedReleaseEvidence(t, db, created.RelatedCode)
	review, err := service.Transition(ctx, created.ID, dto.TransitionRequest{
		Status: "review", ExpectedVersion: created.Version, Reason: "evidence complete",
	}, "operator", model.RoleOperator, "gate3-submit-2")
	if err != nil {
		t.Fatalf("submit authorization: %v", err)
	}

	// Evidence breaks after submission; restriction and return-to-draft do
	// not release the part, so they must stay available.
	setInspectionStatus(t, db, created.RelatedCode, "failed")
	returned, err := service.Transition(ctx, review.ID, dto.TransitionRequest{
		Status: "draft", ExpectedVersion: review.Version, Reason: "inspection reopened, back to draft",
	}, "reviewer", model.RoleReviewer, "gate3-return-3")
	if err != nil {
		t.Fatalf("return to draft: %v", err)
	}
	if returned.Status != "draft" {
		t.Fatalf("unexpected returned state: %#v", returned)
	}

	setInspectionStatus(t, db, created.RelatedCode, "passed")
	reviewAgain, err := service.Transition(ctx, returned.ID, dto.TransitionRequest{
		Status: "review", ExpectedVersion: returned.Version, Reason: "inspection passed again",
	}, "operator", model.RoleOperator, "gate3-submit-4")
	if err != nil {
		t.Fatalf("resubmit authorization: %v", err)
	}
	setInspectionStatus(t, db, created.RelatedCode, "failed")
	restricted, err := service.Transition(ctx, reviewAgain.ID, dto.TransitionRequest{
		Status: "restricted", ExpectedVersion: reviewAgain.Version, Reason: "restrict release scope",
	}, "reviewer", model.RoleReviewer, "gate3-restrict-5")
	if err != nil {
		t.Fatalf("restrict authorization: %v", err)
	}
	if restricted.Status != "restricted" {
		t.Fatalf("unexpected restricted state: %#v", restricted)
	}
}

func TestApprovalLeavesSingleSuccessForRepeatAndConcurrentCalls(t *testing.T) {
	db := newVersionTestDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("unwrap sql db: %v", err)
	}
	// Serialize access through one connection so concurrent approvals are
	// decided purely by the optimistic lock, not by sqlite busy handling.
	sqlDB.SetMaxOpenConns(1)
	service := newReleaseAuthorizationService(db)
	ctx := context.Background()

	created, err := service.Create(ctx, authorizationInput("AUTH-GATE-04"), "operator", "gate4-create-1")
	if err != nil {
		t.Fatalf("create authorization: %v", err)
	}
	seedReleaseEvidence(t, db, created.RelatedCode)
	review, err := service.Transition(ctx, created.ID, dto.TransitionRequest{
		Status: "review", ExpectedVersion: created.Version, Reason: "evidence complete",
	}, "operator", model.RoleOperator, "gate4-submit-2")
	if err != nil {
		t.Fatalf("submit authorization: %v", err)
	}

	approval := dto.TransitionRequest{Status: "approved", ExpectedVersion: review.Version, Reason: "independent release review"}
	var wg sync.WaitGroup
	results := make([]error, 2)
	for index := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := service.Transition(ctx, review.ID, approval, "reviewer", model.RoleReviewer, "gate4-approve-concurrent")
			results[i] = err
		}(index)
	}
	wg.Wait()
	successes := 0
	for _, err := range results {
		if err == nil {
			successes++
			continue
		}
		if !errors.Is(err, ErrInvalidTransition) && !errors.Is(err, repository.ErrVersionConflict) {
			t.Fatalf("unexpected concurrent approval error: %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("expected exactly one concurrent approval to succeed, got %d", successes)
	}
	assertAuthorizationState(t, service, created.ID, "approved", 3, 3)

	// A repeated approval after the successful one is rejected as an invalid
	// transition and the released state is preserved.
	_, err = service.Transition(ctx, review.ID, dto.TransitionRequest{
		Status: "approved", ExpectedVersion: 3, Reason: "duplicate approval attempt",
	}, "reviewer", model.RoleReviewer, "gate4-approve-repeat")
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("repeated approval must be rejected, got %v", err)
	}
	assertAuthorizationState(t, service, created.ID, "approved", 3, 3)
	assertAuditChain(t, db, "ReleaseAuthorization", created.ID, 3)
}

func assertEvidenceGateError(t *testing.T, err error, expectedBlockers int) []string {
	t.Helper()
	var gateErr *EvidenceGateError
	if !errors.As(err, &gateErr) {
		t.Fatalf("expected EvidenceGateError, got %v", err)
	}
	if len(gateErr.Blockers) != expectedBlockers {
		t.Fatalf("expected %d blockers, got %v", expectedBlockers, gateErr.Blockers)
	}
	return gateErr.Blockers
}

func blockersContain(blockers []string, keyword string) bool {
	for _, blocker := range blockers {
		if strings.Contains(blocker, keyword) {
			return true
		}
	}
	return false
}

func assertAuthorizationState(t *testing.T, service ReleaseAuthorizationService, id uint, status string, version uint, revisions int) {
	t.Helper()
	current, err := service.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("reload authorization: %v", err)
	}
	if current.Status != status || current.Version != version || len(current.Revisions) != revisions {
		t.Fatalf("expected status=%s version=%d revisions=%d, got status=%s version=%d revisions=%d",
			status, version, revisions, current.Status, current.Version, len(current.Revisions))
	}
}

func setPartStatus(t *testing.T, db *gorm.DB, code, status string) {
	t.Helper()
	if err := db.Model(&model.AircraftPart{}).Where("code = ?", code).Update("status", status).Error; err != nil {
		t.Fatalf("set part status: %v", err)
	}
}

func setInspectionStatus(t *testing.T, db *gorm.DB, relatedCode, status string) {
	t.Helper()
	if err := db.Model(&model.InspectionTask{}).Where("related_code = ?", relatedCode).Update("status", status).Error; err != nil {
		t.Fatalf("set inspection status: %v", err)
	}
}

func setCertificateState(t *testing.T, db *gorm.DB, relatedCode, status string, effectiveAt time.Time) {
	t.Helper()
	if err := db.Model(&model.CertificateRecord{}).Where("related_code = ?", relatedCode).
		Updates(map[string]any{"status": status, "effective_at": effectiveAt}).Error; err != nil {
		t.Fatalf("set certificate state: %v", err)
	}
}
