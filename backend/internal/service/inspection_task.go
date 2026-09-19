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

type InspectionTaskService interface {
	List(context.Context, dto.PageQuery) (repository.Page[model.InspectionTask], error)
	Get(context.Context, uint) (model.InspectionTask, error)
	Create(context.Context, dto.CreateInspectionTask, string, string) (model.InspectionTask, error)
	Update(context.Context, uint, dto.UpdateInspectionTask, string, string) (model.InspectionTask, error)
	Transition(context.Context, uint, dto.TransitionRequest, string, string) (model.InspectionTask, error)
	Delete(context.Context, uint, string, string) error
	StatusCounts(context.Context) (map[string]int64, error)
}

type inspectionTaskService struct {
	repository repository.InspectionTaskRepository
	security   SecurityService
}

func NewInspectionTaskService(repo repository.InspectionTaskRepository, security SecurityService) InspectionTaskService {
	return &inspectionTaskService{repository: repo, security: security}
}

func (s *inspectionTaskService) List(ctx context.Context, query dto.PageQuery) (repository.Page[model.InspectionTask], error) {
	return s.repository.List(ctx, query)
}

func (s *inspectionTaskService) Get(ctx context.Context, id uint) (model.InspectionTask, error) {
	return s.repository.Get(ctx, id)
}

func (s *inspectionTaskService) Create(ctx context.Context, input dto.CreateInspectionTask, actor, requestID string) (model.InspectionTask, error) {
	if err := validateInspectionTaskBusinessFields(input.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.InspectionTask{}, err
	}
	item := model.InspectionTask{
		BaseModel: model.BaseModel{
			Code: strings.ToUpper(strings.TrimSpace(input.Code)), Name: strings.TrimSpace(input.Name),
			Status: model.InspectionTaskInitialStatus, Version: 1, Description: strings.TrimSpace(input.Description),
		},
		Facility: strings.TrimSpace(input.Facility), Owner: strings.TrimSpace(input.Owner),
		Category: strings.TrimSpace(input.Category), RiskLevel: input.RiskLevel,
		MetricValue: input.MetricValue, MetricUnit: strings.TrimSpace(input.MetricUnit),
		EffectiveAt: input.EffectiveAt.UTC(), Evidence: strings.TrimSpace(input.Evidence),
		RelatedCode: strings.ToUpper(strings.TrimSpace(input.RelatedCode)),
	}
	if err := s.repository.Create(ctx, &item); err != nil {
		return model.InspectionTask{}, fmt.Errorf("create 检查任务: %w", err)
	}
	_ = s.security.Audit(ctx, actor, requestID, "create", "InspectionTask", item.ID, "", item.Status, "created 检查任务")
	return item, nil
}

func (s *inspectionTaskService) Update(ctx context.Context, id uint, input dto.UpdateInspectionTask, actor, requestID string) (model.InspectionTask, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.InspectionTask{}, err
	}
	if err := validateInspectionTaskBusinessFields(current.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.InspectionTask{}, err
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
	if err := s.repository.Update(ctx, id, input.ExpectedVersion, &current); err != nil {
		return model.InspectionTask{}, fmt.Errorf("update 检查任务: %w", err)
	}
	_ = s.security.Audit(ctx, actor, requestID, "update", "InspectionTask", id, current.Status, current.Status, "updated business fields")
	return s.repository.Get(ctx, id)
}

func (s *inspectionTaskService) Transition(ctx context.Context, id uint, input dto.TransitionRequest, actor, requestID string) (model.InspectionTask, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.InspectionTask{}, err
	}
	target := strings.TrimSpace(input.Status)
	if !constants.CanTransition(constants.InspectionTaskTransitions, current.Status, target) {
		return model.InspectionTask{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, current.Status, target)
	}
	before := current.Status
	current.Status = target
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	if err := s.repository.Update(ctx, id, input.ExpectedVersion, &current); err != nil {
		return model.InspectionTask{}, fmt.Errorf("transition 检查任务: %w", err)
	}
	if err := s.security.Audit(ctx, actor, requestID, "transition", "InspectionTask", id, before, target, input.Reason); err != nil {
		return model.InspectionTask{}, fmt.Errorf("persist transition audit: %w", err)
	}
	return s.repository.Get(ctx, id)
}

func (s *inspectionTaskService) Delete(ctx context.Context, id uint, actor, requestID string) error {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repository.Delete(ctx, id); err != nil {
		return err
	}
	return s.security.Audit(ctx, actor, requestID, "delete", "InspectionTask", id, current.Status, "deleted", "soft deleted 检查任务")
}

func (s *inspectionTaskService) StatusCounts(ctx context.Context) (map[string]int64, error) {
	return s.repository.CountByStatus(ctx)
}

func validateInspectionTaskBusinessFields(code, name, facility, owner string) error {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(name) == "" || strings.TrimSpace(facility) == "" || strings.TrimSpace(owner) == "" {
		return ErrInvalidInput
	}
	return nil
}
