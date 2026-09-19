package service

import (
	"context"
	"errors"
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
	service := NewReleaseAuthorizationService(repository.NewReleaseAuthorizationRepository(db), nil)
	ctx := context.Background()

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

func newVersionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.AuditLog{}, &model.CertificateRecord{}, &model.CertificateRecordRevision{},
		&model.ReleaseAuthorization{}, &model.ReleaseAuthorizationRevision{},
	); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	return db
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
