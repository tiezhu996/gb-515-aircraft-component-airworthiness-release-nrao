package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/blueship581/aircraft-component-airworthiness-release/backend/internal/dto"
	"github.com/blueship581/aircraft-component-airworthiness-release/backend/internal/model"
	"github.com/blueship581/aircraft-component-airworthiness-release/backend/internal/repository"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestCertificateVersionChainRequiresIndependentReviewer(t *testing.T) {
	db := newVersionTestDB(t)
	service := NewCertificateRecordService(repository.NewCertificateRecordRepository(db), nil)
	ctx := context.Background()

	created, err := service.Create(ctx, certificateInput("CERT-TEST-01"), "operator", "cert-create-1")
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	if created.Version != 1 || created.PreparedBy != "operator" || len(created.Revisions) != 1 {
		t.Fatalf("unexpected prepared certificate: %#v", created)
	}
	if revision := created.Revisions[0]; revision.Actor != "operator" || revision.RequestID != "cert-create-1" {
		t.Fatalf("missing create attribution: %#v", revision)
	}

	transition := dto.TransitionRequest{Status: "valid", ExpectedVersion: created.Version, Reason: "independent airworthiness review passed"}
	if _, err := service.Transition(ctx, created.ID, transition, "operator", model.RoleOperator, "cert-operator-denied"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("operator publication must be forbidden, got %v", err)
	}
	if _, err := service.Transition(ctx, created.ID, transition, "operator", model.RoleReviewer, "cert-same-user-denied"); !errors.Is(err, ErrSeparationOfDuty) {
		t.Fatalf("same-user review must be rejected, got %v", err)
	}

	verified, err := service.Transition(ctx, created.ID, transition, "reviewer", model.RoleReviewer, "cert-review-2")
	if err != nil {
		t.Fatalf("verify certificate: %v", err)
	}
	if verified.Status != "valid" || verified.Version != 2 || verified.VerifiedBy != "reviewer" || len(verified.Revisions) != 2 {
		t.Fatalf("unexpected verified certificate: %#v", verified)
	}
	latest := verified.Revisions[1]
	if latest.Actor != "reviewer" || latest.RequestID != "cert-review-2" || latest.Status != "valid" {
		t.Fatalf("invalid publication revision: %#v", latest)
	}

	update := updateCertificateInput(verified)
	if _, err := service.Update(ctx, verified.ID, update, "operator", "cert-late-edit"); !errors.Is(err, ErrLocked) {
		t.Fatalf("published certificate must be immutable, got %v", err)
	}
	assertAuditChain(t, db, "CertificateRecord", created.ID, 2)
}

func TestAuthorizationVersionChainEnforcesDualControl(t *testing.T) {
	db := newVersionTestDB(t)
	service := newReleaseService(db)
	ctx := context.Background()
	seedReleaseEvidence(t, db, "PART-101")

	created, err := service.Create(ctx, authorizationInput("AUTH-TEST-01"), "operator", "auth-create-1")
	if err != nil {
		t.Fatalf("create authorization: %v", err)
	}
	review, err := service.Transition(ctx, created.ID, dto.TransitionRequest{
		Status: "review", ExpectedVersion: created.Version, Reason: "inspection evidence complete",
	}, "operator", model.RoleOperator, "auth-submit-2")
	if err != nil {
		t.Fatalf("submit authorization: %v", err)
	}
	if review.SubmittedBy != "operator" || review.Status != "review" || len(review.Revisions) != 2 {
		t.Fatalf("unexpected review submission: %#v", review)
	}

	approval := dto.TransitionRequest{Status: "approved", ExpectedVersion: review.Version, Reason: "independent release review passed"}
	if _, err := service.Transition(ctx, review.ID, approval, "operator", model.RoleOperator, "auth-operator-denied"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("operator approval must be forbidden, got %v", err)
	}
	if _, err := service.Transition(ctx, review.ID, approval, "operator", model.RoleReviewer, "auth-same-user-denied"); !errors.Is(err, ErrSeparationOfDuty) {
		t.Fatalf("same-user approval must be rejected, got %v", err)
	}

	approved, err := service.Transition(ctx, review.ID, approval, "reviewer", model.RoleReviewer, "auth-approve-3")
	if err != nil {
		t.Fatalf("approve authorization: %v", err)
	}
	if approved.Status != "approved" || approved.Version != 3 || approved.ReviewedBy != "reviewer" || len(approved.Revisions) != 3 {
		t.Fatalf("unexpected approved authorization: %#v", approved)
	}
	latest := approved.Revisions[2]
	if latest.Actor != "reviewer" || latest.RequestID != "auth-approve-3" || latest.Status != "approved" {
		t.Fatalf("invalid approval revision: %#v", latest)
	}
	if _, err := service.Update(ctx, approved.ID, updateAuthorizationInput(approved), "operator", "auth-late-edit"); !errors.Is(err, ErrLocked) {
		t.Fatalf("reviewed authorization must be immutable, got %v", err)
	}
	assertAuditChain(t, db, "ReleaseAuthorization", created.ID, 3)
}

func TestReleaseEvidenceGateBlocksSubmission(t *testing.T) {
	db := newVersionTestDB(t)
	service := newReleaseService(db)
	ctx := context.Background()

	created, err := service.Create(ctx, authorizationInput("AUTH-GATE-01"), "operator", "gate-create-1")
	if err != nil {
		t.Fatalf("create authorization: %v", err)
	}
	submit := dto.TransitionRequest{Status: "review", ExpectedVersion: created.Version, Reason: "submit for release review"}

	// 完全缺少证据时部件、检查、证书三项全部阻断。
	_, err = service.Transition(ctx, created.ID, submit, "operator", model.RoleOperator, "gate-submit-empty")
	assertGateBlockers(t, err, 3)
	assertAuthorizationState(t, service, ctx, created.ID, "draft", 1)

	seedReleaseEvidence(t, db, "PART-101")

	// 部件未处于暂停。
	setEvidenceStatus(t, db, &model.AircraftPart{}, "PART-101", "inspection")
	_, err = service.Transition(ctx, created.ID, submit, "operator", model.RoleOperator, "gate-submit-part")
	assertGateBlockers(t, err, 1)

	// 部件已暂停但检查未通过。
	setEvidenceStatus(t, db, &model.AircraftPart{}, "PART-101", "hold")
	setEvidenceStatus(t, db, &model.InspectionTask{}, "INSP-PART-101", "running")
	_, err = service.Transition(ctx, created.ID, submit, "operator", model.RoleOperator, "gate-submit-inspection")
	assertGateBlockers(t, err, 1)

	// 检查已通过但证书草稿未发布。
	setEvidenceStatus(t, db, &model.InspectionTask{}, "INSP-PART-101", "passed")
	setEvidenceStatus(t, db, &model.CertificateRecord{}, "CERT-PART-101", "draft")
	_, err = service.Transition(ctx, created.ID, submit, "operator", model.RoleOperator, "gate-submit-cert-draft")
	assertGateBlockers(t, err, 1)

	// 证书有效但未到生效时间，不在有效期内。
	setEvidenceStatus(t, db, &model.CertificateRecord{}, "CERT-PART-101", "valid")
	if err := db.Model(&model.CertificateRecord{}).Where("code = ?", "CERT-PART-101").
		Update("effective_at", time.Now().UTC().Add(2*time.Hour)).Error; err != nil {
		t.Fatalf("shift certificate effective time: %v", err)
	}
	_, err = service.Transition(ctx, created.ID, submit, "operator", model.RoleOperator, "gate-submit-cert-window")
	assertGateBlockers(t, err, 1)

	// 证据全部满足后提交成功，且此前被阻断的尝试没有写入任何版本或审计。
	if err := db.Model(&model.CertificateRecord{}).Where("code = ?", "CERT-PART-101").
		Update("effective_at", time.Now().UTC().Add(-time.Hour)).Error; err != nil {
		t.Fatalf("restore certificate effective time: %v", err)
	}
	review, err := service.Transition(ctx, created.ID, submit, "operator", model.RoleOperator, "gate-submit-ok")
	if err != nil {
		t.Fatalf("submit with complete evidence: %v", err)
	}
	if review.Status != "review" || review.Version != 2 || len(review.Revisions) != 2 {
		t.Fatalf("unexpected review submission: %#v", review)
	}
	assertAuditChain(t, db, "ReleaseAuthorization", created.ID, 2)
}

func TestReleaseApprovalRevalidatesEvidence(t *testing.T) {
	db := newVersionTestDB(t)
	service := newReleaseService(db)
	ctx := context.Background()
	seedReleaseEvidence(t, db, "PART-101")

	created, err := service.Create(ctx, authorizationInput("AUTH-GATE-02"), "operator", "gate2-create-1")
	if err != nil {
		t.Fatalf("create authorization: %v", err)
	}
	review, err := service.Transition(ctx, created.ID, dto.TransitionRequest{
		Status: "review", ExpectedVersion: created.Version, Reason: "evidence complete",
	}, "operator", model.RoleOperator, "gate2-review-2")
	if err != nil {
		t.Fatalf("submit authorization: %v", err)
	}
	approval := dto.TransitionRequest{Status: "approved", ExpectedVersion: review.Version, Reason: "independent release review passed"}

	// 等待复核期间部件被解除暂停，批准必须被重新核验阻断。
	setEvidenceStatus(t, db, &model.AircraftPart{}, "PART-101", "released")
	_, err = service.Transition(ctx, review.ID, approval, "reviewer", model.RoleReviewer, "gate2-approve-part")
	assertGateBlockers(t, err, 1)
	assertAuthorizationState(t, service, ctx, review.ID, "review", review.Version)

	// 部件恢复但证书被吊销，批准仍然阻断。
	setEvidenceStatus(t, db, &model.AircraftPart{}, "PART-101", "hold")
	setEvidenceStatus(t, db, &model.CertificateRecord{}, "CERT-PART-101", "revoked")
	_, err = service.Transition(ctx, review.ID, approval, "reviewer", model.RoleReviewer, "gate2-approve-cert")
	assertGateBlockers(t, err, 1)
	assertAuthorizationState(t, service, ctx, review.ID, "review", review.Version)

	// 证据恢复后批准成功。
	setEvidenceStatus(t, db, &model.CertificateRecord{}, "CERT-PART-101", "valid")
	approved, err := service.Transition(ctx, review.ID, approval, "reviewer", model.RoleReviewer, "gate2-approve-3")
	if err != nil {
		t.Fatalf("approve with restored evidence: %v", err)
	}
	if approved.Status != "approved" || approved.Version != 3 || approved.ReviewedBy != "reviewer" {
		t.Fatalf("unexpected approved authorization: %#v", approved)
	}
	assertAuditChain(t, db, "ReleaseAuthorization", created.ID, 3)
}

func TestConcurrentApprovalLeavesSingleSuccess(t *testing.T) {
	db := newVersionTestDB(t)
	service := newReleaseService(db)
	ctx := context.Background()
	seedReleaseEvidence(t, db, "PART-101")

	created, err := service.Create(ctx, authorizationInput("AUTH-GATE-03"), "operator", "gate3-create-1")
	if err != nil {
		t.Fatalf("create authorization: %v", err)
	}
	review, err := service.Transition(ctx, created.ID, dto.TransitionRequest{
		Status: "review", ExpectedVersion: created.Version, Reason: "evidence complete",
	}, "operator", model.RoleOperator, "gate3-review-2")
	if err != nil {
		t.Fatalf("submit authorization: %v", err)
	}

	const attempts = 4
	results := make(chan error, attempts)
	for i := 0; i < attempts; i++ {
		go func(attempt int) {
			_, err := service.Transition(ctx, review.ID, dto.TransitionRequest{
				Status: "approved", ExpectedVersion: review.Version, Reason: "concurrent release review",
			}, fmt.Sprintf("reviewer-%d", attempt), model.RoleReviewer, fmt.Sprintf("gate3-approve-%d", attempt))
			results <- err
		}(i)
	}
	successes := 0
	for i := 0; i < attempts; i++ {
		err := <-results
		if err == nil {
			successes++
			continue
		}
		if !errors.Is(err, repository.ErrVersionConflict) && !errors.Is(err, ErrInvalidTransition) {
			t.Fatalf("unexpected concurrent approval error: %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("expected exactly one successful approval, got %d", successes)
	}
	final, err := service.Get(ctx, review.ID)
	if err != nil {
		t.Fatalf("reload authorization: %v", err)
	}
	if final.Status != "approved" || final.Version != review.Version+1 || len(final.Revisions) != 3 {
		t.Fatalf("concurrent approvals must leave a single version chain: %#v", final)
	}
	assertAuditChain(t, db, "ReleaseAuthorization", created.ID, 3)
}

func assertGateBlockers(t *testing.T, err error, expected int) {
	t.Helper()
	var gateErr *EvidenceGateError
	if !errors.As(err, &gateErr) {
		t.Fatalf("expected evidence gate rejection, got %v", err)
	}
	if len(gateErr.Blockers) != expected {
		t.Fatalf("expected %d blockers, got %v", expected, gateErr.Blockers)
	}
}

func assertAuthorizationState(t *testing.T, service ReleaseAuthorizationService, ctx context.Context, id uint, status string, version uint) {
	t.Helper()
	current, err := service.Get(ctx, id)
	if err != nil {
		t.Fatalf("reload authorization: %v", err)
	}
	if current.Status != status || current.Version != version {
		t.Fatalf("rejected transition must preserve state, got status=%s version=%d", current.Status, current.Version)
	}
}

func newVersionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sqlite handle: %v", err)
	}
	// 单连接串行化内存 SQLite，并发测试才能稳定命中乐观锁而不是驱动级锁错误。
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(
		&model.AuditLog{}, &model.AircraftPart{}, &model.InspectionTask{},
		&model.CertificateRecord{}, &model.CertificateRecordRevision{},
		&model.ReleaseAuthorization{}, &model.ReleaseAuthorizationRevision{},
	); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	return db
}

func newReleaseService(db *gorm.DB) ReleaseAuthorizationService {
	return NewReleaseAuthorizationService(
		repository.NewReleaseAuthorizationRepository(db), nil,
		repository.NewAircraftPartRepository(db),
		repository.NewInspectionTaskRepository(db),
		repository.NewCertificateRecordRepository(db),
	)
}

// seedReleaseEvidence 写入一条完整可放行的证据链：部件暂停、检查通过、证书有效且已生效。
func seedReleaseEvidence(t *testing.T, db *gorm.DB, partCode string) {
	t.Helper()
	part := model.AircraftPart{
		BaseModel: model.BaseModel{Code: partCode, Name: "Evidence part", Status: "hold", Version: 1},
		Facility:  "Hangar 2", Owner: "Airworthiness team", Category: "engine", RiskLevel: "high",
		EffectiveAt: time.Now().UTC().Add(-time.Hour), RelatedCode: "REL-" + partCode,
	}
	inspection := model.InspectionTask{
		BaseModel: model.BaseModel{Code: "INSP-" + partCode, Name: "Evidence inspection", Status: "passed", Version: 1},
		Facility:  "Hangar 2", Owner: "Inspection team", Category: "engine", RiskLevel: "medium",
		EffectiveAt: time.Now().UTC().Add(-time.Hour), RelatedCode: partCode,
	}
	certificate := model.CertificateRecord{
		BaseModel: model.BaseModel{Code: "CERT-" + partCode, Name: "Evidence certificate", Status: "valid", Version: 1},
		Facility:  "Hangar 2", Owner: "Certificate desk", Category: "engine", RiskLevel: "medium",
		EffectiveAt: time.Now().UTC().Add(-time.Hour), RelatedCode: partCode, PreparedBy: "operator", VerifiedBy: "reviewer",
	}
	for _, record := range []any{&part, &inspection, &certificate} {
		if err := db.Create(record).Error; err != nil {
			t.Fatalf("seed release evidence: %v", err)
		}
	}
}

func setEvidenceStatus(t *testing.T, db *gorm.DB, entity any, code, status string) {
	t.Helper()
	if err := db.Model(entity).Where("code = ?", code).Update("status", status).Error; err != nil {
		t.Fatalf("update evidence status: %v", err)
	}
}

func certificateInput(code string) dto.CreateCertificateRecord {
	return dto.CreateCertificateRecord{
		Code: code, Name: "Turbine certificate", Description: "controlled certificate",
		Facility: "Hangar 2", Owner: "Airworthiness team", Category: "engine",
		RiskLevel: "high", MetricValue: 100, MetricUnit: "percent",
		EffectiveAt: time.Now().UTC(), Evidence: "inspection report IR-101", RelatedCode: "PART-101",
	}
}

func authorizationInput(code string) dto.CreateReleaseAuthorization {
	return dto.CreateReleaseAuthorization{
		Code: code, Name: "Component release", Description: "controlled release",
		Facility: "Hangar 2", Owner: "Release desk", Category: "engine",
		RiskLevel: "high", MetricValue: 100, MetricUnit: "percent",
		EffectiveAt: time.Now().UTC(), Evidence: "certificate CERT-101", RelatedCode: "PART-101",
	}
}

func updateCertificateInput(item model.CertificateRecord) dto.UpdateCertificateRecord {
	return dto.UpdateCertificateRecord{
		ExpectedVersion: item.Version, Name: item.Name, Description: item.Description,
		Facility: item.Facility, Owner: item.Owner, Category: item.Category, RiskLevel: item.RiskLevel,
		MetricValue: item.MetricValue, MetricUnit: item.MetricUnit, EffectiveAt: item.EffectiveAt,
		Evidence: item.Evidence, RelatedCode: item.RelatedCode,
	}
}

func updateAuthorizationInput(item model.ReleaseAuthorization) dto.UpdateReleaseAuthorization {
	return dto.UpdateReleaseAuthorization{
		ExpectedVersion: item.Version, Name: item.Name, Description: item.Description,
		Facility: item.Facility, Owner: item.Owner, Category: item.Category, RiskLevel: item.RiskLevel,
		MetricValue: item.MetricValue, MetricUnit: item.MetricUnit, EffectiveAt: item.EffectiveAt,
		Evidence: item.Evidence, RelatedCode: item.RelatedCode,
	}
}

func assertAuditChain(t *testing.T, db *gorm.DB, entityType string, entityID uint, expected int64) {
	t.Helper()
	var count int64
	if err := db.Model(&model.AuditLog{}).Where("entity_type = ? AND entity_id = ?", entityType, entityID).Count(&count).Error; err != nil {
		t.Fatalf("count audits: %v", err)
	}
	if count != expected {
		t.Fatalf("expected %d atomic audits, got %d", expected, count)
	}
}
