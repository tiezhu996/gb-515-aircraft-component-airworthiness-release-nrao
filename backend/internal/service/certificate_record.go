package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/blueship581/aircraft-component-airworthiness-release/backend/internal/constants"
	"github.com/blueship581/aircraft-component-airworthiness-release/backend/internal/dto"
	"github.com/blueship581/aircraft-component-airworthiness-release/backend/internal/model"
	"github.com/blueship581/aircraft-component-airworthiness-release/backend/internal/repository"
)

type CertificateRecordService interface {
	List(context.Context, dto.PageQuery) (repository.Page[model.CertificateRecord], error)
	Get(context.Context, uint) (model.CertificateRecord, error)
	Create(context.Context, dto.CreateCertificateRecord, string, string) (model.CertificateRecord, error)
	Update(context.Context, uint, dto.UpdateCertificateRecord, string, string) (model.CertificateRecord, error)
	Transition(context.Context, uint, dto.TransitionRequest, string, string, string) (model.CertificateRecord, error)
	Delete(context.Context, uint, string, string) error
	StatusCounts(context.Context) (map[string]int64, error)
}

type certificateRecordService struct {
	repository repository.CertificateRecordRepository
	security   SecurityService
}

func NewCertificateRecordService(repo repository.CertificateRecordRepository, security SecurityService) CertificateRecordService {
	return &certificateRecordService{repository: repo, security: security}
}

func (s *certificateRecordService) List(ctx context.Context, query dto.PageQuery) (repository.Page[model.CertificateRecord], error) {
	return s.repository.List(ctx, query)
}

func (s *certificateRecordService) Get(ctx context.Context, id uint) (model.CertificateRecord, error) {
	return s.repository.Get(ctx, id)
}

func (s *certificateRecordService) Create(ctx context.Context, input dto.CreateCertificateRecord, actor, requestID string) (model.CertificateRecord, error) {
	if err := validateCertificateRecordBusinessFields(input.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.CertificateRecord{}, err
	}
	item := model.CertificateRecord{
		BaseModel: model.BaseModel{
			Code: strings.ToUpper(strings.TrimSpace(input.Code)), Name: strings.TrimSpace(input.Name),
			Status: model.CertificateRecordInitialStatus, Version: 1, Description: strings.TrimSpace(input.Description),
		},
		Facility: strings.TrimSpace(input.Facility), Owner: strings.TrimSpace(input.Owner),
		Category: strings.TrimSpace(input.Category), RiskLevel: input.RiskLevel,
		MetricValue: input.MetricValue, MetricUnit: strings.TrimSpace(input.MetricUnit),
		EffectiveAt: input.EffectiveAt.UTC(), Evidence: strings.TrimSpace(input.Evidence),
		RelatedCode: strings.ToUpper(strings.TrimSpace(input.RelatedCode)), PreparedBy: actor,
	}
	if err := s.repository.CreateVersion(ctx, &item, actor, requestID); err != nil {
		return model.CertificateRecord{}, fmt.Errorf("create 证书记录: %w", err)
	}
	return s.repository.Get(ctx, item.ID)
}

func (s *certificateRecordService) Update(ctx context.Context, id uint, input dto.UpdateCertificateRecord, actor, requestID string) (model.CertificateRecord, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.CertificateRecord{}, err
	}
	if current.Status != model.CertificateRecordInitialStatus {
		return model.CertificateRecord{}, ErrLocked
	}
	if err := validateCertificateRecordBusinessFields(current.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.CertificateRecord{}, err
	}
	current.Name = strings.TrimSpace(input.Name)
	current.Description = strings.TrimSpace(input.Description)
	current.Facility = strings.TrimSpace(input.Facility)
	current.Owner = strings.TrimSpace(input.Owner)
	current.Category = strings.TrimSpace(input.Category)
	current.RiskLevel = input.RiskLevel
	current.MetricValue = input.MetricValue
	current.MetricUnit = strings.TrimSpace(input.MetricUnit)
	current.EffectiveAt = input.EffectiveAt.UTC()
	current.Evidence = strings.TrimSpace(input.Evidence)
	current.RelatedCode = strings.ToUpper(strings.TrimSpace(input.RelatedCode))
	current.PreparedBy = actor
	current.VerifiedBy = ""
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	if err := s.repository.UpdateVersion(ctx, id, input.ExpectedVersion, &current, actor, requestID, "update", current.Status, "draft certificate fields updated"); err != nil {
		return model.CertificateRecord{}, fmt.Errorf("update 证书记录: %w", err)
	}
	return s.repository.Get(ctx, id)
}

func (s *certificateRecordService) Transition(ctx context.Context, id uint, input dto.TransitionRequest, actor, role, requestID string) (model.CertificateRecord, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.CertificateRecord{}, err
	}
	target := strings.TrimSpace(input.Status)
	if !constants.CanTransition(constants.CertificateRecordTransitions, current.Status, target) {
		return model.CertificateRecord{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, current.Status, target)
	}
	if !isOperatorRole(role) {
		return model.CertificateRecord{}, ErrForbidden
	}
	if target == "valid" || target == "revoked" {
		if !isReviewerRole(role) {
			return model.CertificateRecord{}, ErrForbidden
		}
		if actor == current.PreparedBy {
			return model.CertificateRecord{}, ErrSeparationOfDuty
		}
		current.VerifiedBy = actor
	}
	before := current.Status
	current.Status = target
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	if err := s.repository.UpdateVersion(ctx, id, input.ExpectedVersion, &current, actor, requestID, "transition", before, strings.TrimSpace(input.Reason)); err != nil {
		return model.CertificateRecord{}, fmt.Errorf("transition 证书记录: %w", err)
	}
	return s.repository.Get(ctx, id)
}

func (s *certificateRecordService) Delete(ctx context.Context, id uint, actor, requestID string) error {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return err
	}
	if current.Status != model.CertificateRecordInitialStatus {
		return ErrLocked
	}
	if err := s.repository.Delete(ctx, id); err != nil {
		return err
	}
	return s.security.Audit(ctx, actor, requestID, "delete", "CertificateRecord", id, current.Status, "deleted", "soft deleted 证书记录")
}

func (s *certificateRecordService) StatusCounts(ctx context.Context) (map[string]int64, error) {
	return s.repository.CountByStatus(ctx)
}

func validateCertificateRecordBusinessFields(code, name, facility, owner string) error {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(name) == "" || strings.TrimSpace(facility) == "" || strings.TrimSpace(owner) == "" {
		return ErrInvalidInput
	}
	return nil
}
