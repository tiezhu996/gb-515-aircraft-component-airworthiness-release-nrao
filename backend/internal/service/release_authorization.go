package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/blueship581/aircraft-component-airworthiness-release/backend/internal/constants"
	"github.com/blueship581/aircraft-component-airworthiness-release/backend/internal/dto"
	"github.com/blueship581/aircraft-component-airworthiness-release/backend/internal/model"
	"github.com/blueship581/aircraft-component-airworthiness-release/backend/internal/repository"
	"gorm.io/gorm"
)

type ReleaseAuthorizationService interface {
	List(context.Context, dto.PageQuery) (repository.Page[model.ReleaseAuthorization], error)
	Get(context.Context, uint) (model.ReleaseAuthorization, error)
	Create(context.Context, dto.CreateReleaseAuthorization, string, string) (model.ReleaseAuthorization, error)
	Update(context.Context, uint, dto.UpdateReleaseAuthorization, string, string) (model.ReleaseAuthorization, error)
	Transition(context.Context, uint, dto.TransitionRequest, string, string, string) (model.ReleaseAuthorization, error)
	Delete(context.Context, uint, string, string) error
	StatusCounts(context.Context) (map[string]int64, error)
}

type releaseAuthorizationService struct {
	repository   repository.ReleaseAuthorizationRepository
	security     SecurityService
	parts        repository.AircraftPartRepository
	inspections  repository.InspectionTaskRepository
	certificates repository.CertificateRecordRepository
}

func NewReleaseAuthorizationService(repo repository.ReleaseAuthorizationRepository, security SecurityService, parts repository.AircraftPartRepository, inspections repository.InspectionTaskRepository, certificates repository.CertificateRecordRepository) ReleaseAuthorizationService {
	return &releaseAuthorizationService{repository: repo, security: security, parts: parts, inspections: inspections, certificates: certificates}
}

func (s *releaseAuthorizationService) List(ctx context.Context, query dto.PageQuery) (repository.Page[model.ReleaseAuthorization], error) {
	return s.repository.List(ctx, query)
}

func (s *releaseAuthorizationService) Get(ctx context.Context, id uint) (model.ReleaseAuthorization, error) {
	return s.repository.Get(ctx, id)
}

func (s *releaseAuthorizationService) Create(ctx context.Context, input dto.CreateReleaseAuthorization, actor, requestID string) (model.ReleaseAuthorization, error) {
	if err := validateReleaseAuthorizationBusinessFields(input.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.ReleaseAuthorization{}, err
	}
	item := model.ReleaseAuthorization{
		BaseModel: model.BaseModel{
			Code: strings.ToUpper(strings.TrimSpace(input.Code)), Name: strings.TrimSpace(input.Name),
			Status: model.ReleaseAuthorizationInitialStatus, Version: 1, Description: strings.TrimSpace(input.Description),
		},
		Facility: strings.TrimSpace(input.Facility), Owner: strings.TrimSpace(input.Owner),
		Category: strings.TrimSpace(input.Category), RiskLevel: input.RiskLevel,
		MetricValue: input.MetricValue, MetricUnit: strings.TrimSpace(input.MetricUnit),
		EffectiveAt: input.EffectiveAt.UTC(), Evidence: strings.TrimSpace(input.Evidence),
		RelatedCode: strings.ToUpper(strings.TrimSpace(input.RelatedCode)),
	}
	if err := s.repository.CreateVersion(ctx, &item, actor, requestID); err != nil {
		return model.ReleaseAuthorization{}, fmt.Errorf("create 放行授权: %w", err)
	}
	return s.repository.Get(ctx, item.ID)
}

func (s *releaseAuthorizationService) Update(ctx context.Context, id uint, input dto.UpdateReleaseAuthorization, actor, requestID string) (model.ReleaseAuthorization, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.ReleaseAuthorization{}, err
	}
	if current.Status != model.ReleaseAuthorizationInitialStatus {
		return model.ReleaseAuthorization{}, ErrLocked
	}
	if err := validateReleaseAuthorizationBusinessFields(current.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.ReleaseAuthorization{}, err
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
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	if err := s.repository.UpdateVersion(ctx, id, input.ExpectedVersion, &current, actor, requestID, "update", current.Status, "draft authorization fields updated"); err != nil {
		return model.ReleaseAuthorization{}, fmt.Errorf("update 放行授权: %w", err)
	}
	return s.repository.Get(ctx, id)
}

func (s *releaseAuthorizationService) Transition(ctx context.Context, id uint, input dto.TransitionRequest, actor, role, requestID string) (model.ReleaseAuthorization, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.ReleaseAuthorization{}, err
	}
	target := strings.TrimSpace(input.Status)
	if !constants.CanTransition(constants.ReleaseAuthorizationTransitions, current.Status, target) {
		return model.ReleaseAuthorization{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, current.Status, target)
	}
	if !isOperatorRole(role) {
		return model.ReleaseAuthorization{}, ErrForbidden
	}
	if target == "review" {
		current.SubmittedBy = actor
		current.ReviewedBy = ""
		current.ReviewReason = ""
	}
	if target == "approved" || target == "restricted" || target == "revoked" || target == "draft" {
		if !isReviewerRole(role) {
			return model.ReleaseAuthorization{}, ErrForbidden
		}
		if current.SubmittedBy != "" && actor == current.SubmittedBy {
			return model.ReleaseAuthorization{}, ErrSeparationOfDuty
		}
		current.ReviewedBy = actor
		current.ReviewReason = strings.TrimSpace(input.Reason)
	}
	// 放行前证据闸门：提交复核与复核批准都必须通过最新证据核验。批准时重新
	// 读取证据，确保等待复核期间的证据变化会阻断放行。拒绝时不写库，原状态保留。
	if target == "review" || target == "approved" {
		blockers, err := s.evidenceBlockers(ctx, current.RelatedCode, time.Now().UTC())
		if err != nil {
			return model.ReleaseAuthorization{}, fmt.Errorf("evidence gate 放行授权: %w", err)
		}
		if len(blockers) > 0 {
			return model.ReleaseAuthorization{}, &EvidenceGateError{Blockers: blockers}
		}
	}
	before := current.Status
	current.Status = target
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	if err := s.repository.UpdateVersion(ctx, id, input.ExpectedVersion, &current, actor, requestID, "transition", before, strings.TrimSpace(input.Reason)); err != nil {
		return model.ReleaseAuthorization{}, fmt.Errorf("transition 放行授权: %w", err)
	}
	return s.repository.Get(ctx, id)
}

func (s *releaseAuthorizationService) Delete(ctx context.Context, id uint, actor, requestID string) error {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return err
	}
	if current.Status != model.ReleaseAuthorizationInitialStatus {
		return ErrLocked
	}
	if err := s.repository.Delete(ctx, id); err != nil {
		return err
	}
	return s.security.Audit(ctx, actor, requestID, "delete", "ReleaseAuthorization", id, current.Status, "deleted", "soft deleted 放行授权")
}

func (s *releaseAuthorizationService) StatusCounts(ctx context.Context) (map[string]int64, error) {
	return s.repository.CountByStatus(ctx)
}

func validateReleaseAuthorizationBusinessFields(code, name, facility, owner string) error {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(name) == "" || strings.TrimSpace(facility) == "" || strings.TrimSpace(owner) == "" {
		return ErrInvalidInput
	}
	return nil
}

// evidenceBlockers 按关联部件编码核对部件、检查任务与证书记录：部件必须处于
// hold（暂停），最新检查任务必须 passed，最新证书必须 valid 且已到生效时间。
// 返回的每一项都是面向页面的具体阻断原因。
func (s *releaseAuthorizationService) evidenceBlockers(ctx context.Context, relatedCode string, now time.Time) ([]string, error) {
	code := strings.ToUpper(strings.TrimSpace(relatedCode))
	if code == "" {
		return []string{"放行授权缺少关联部件编码，无法核对部件、检查任务与证书记录"}, nil
	}
	blockers := make([]string, 0, 3)
	part, err := s.parts.FindByCode(ctx, code)
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		blockers = append(blockers, fmt.Sprintf("关联部件 %s 不存在", code))
	case err != nil:
		return nil, fmt.Errorf("查询关联部件 %s: %w", code, err)
	case part.Status != "hold":
		blockers = append(blockers, fmt.Sprintf("关联部件 %s 当前状态为 %s，必须处于 hold（暂停）", code, part.Status))
	}
	inspection, err := s.inspections.FindLatestByRelatedCode(ctx, code)
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		blockers = append(blockers, fmt.Sprintf("关联部件 %s 缺少检查任务记录", code))
	case err != nil:
		return nil, fmt.Errorf("查询部件 %s 的检查任务: %w", code, err)
	case inspection.Status != "passed":
		blockers = append(blockers, fmt.Sprintf("检查任务 %s 当前状态为 %s，必须通过（passed）", inspection.Code, inspection.Status))
	}
	certificate, err := s.certificates.FindLatestByRelatedCode(ctx, code)
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		blockers = append(blockers, fmt.Sprintf("关联部件 %s 缺少证书记录", code))
	case err != nil:
		return nil, fmt.Errorf("查询部件 %s 的证书记录: %w", code, err)
	default:
		if certificate.Status != "valid" {
			blockers = append(blockers, fmt.Sprintf("证书 %s 当前状态为 %s，必须有效（valid）", certificate.Code, certificate.Status))
		}
		if certificate.EffectiveAt.After(now) {
			blockers = append(blockers, fmt.Sprintf("证书 %s 生效时间为 %s，当前不在有效期内", certificate.Code, certificate.EffectiveAt.UTC().Format("2006-01-02 15:04")))
		}
	}
	return blockers, nil
}

func isOperatorRole(role string) bool {
	return role == model.RoleOperator || role == model.RoleReviewer || role == model.RoleAdmin
}

func isReviewerRole(role string) bool {
	return role == model.RoleReviewer || role == model.RoleAdmin
}
