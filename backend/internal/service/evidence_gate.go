package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/blueship581/aircraft-component-airworthiness-release/backend/internal/repository"
	"gorm.io/gorm"
)

// EvidenceGate re-reads the live part, inspection and certificate records
// linked by the authorization's related part code. Every unmet condition is
// returned as a human-readable blocker so a rejected submission or approval
// can explain exactly which evidence is missing or stale.
type EvidenceGate interface {
	Blockers(ctx context.Context, relatedCode string) ([]string, error)
}

type evidenceGate struct {
	parts        repository.AircraftPartRepository
	inspections  repository.InspectionTaskRepository
	certificates repository.CertificateRecordRepository
}

func NewEvidenceGate(parts repository.AircraftPartRepository, inspections repository.InspectionTaskRepository, certificates repository.CertificateRecordRepository) EvidenceGate {
	return &evidenceGate{parts: parts, inspections: inspections, certificates: certificates}
}

func (g *evidenceGate) Blockers(ctx context.Context, relatedCode string) ([]string, error) {
	code := strings.ToUpper(strings.TrimSpace(relatedCode))
	if code == "" {
		return []string{"放行授权缺少关联部件编码，无法核对部件、检查与证书证据"}, nil
	}
	blockers := make([]string, 0, 3)

	part, err := g.parts.GetByCode(ctx, code)
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		blockers = append(blockers, fmt.Sprintf("未找到编码为 %s 的关联部件", code))
	case err != nil:
		return nil, fmt.Errorf("查询关联部件: %w", err)
	case part.Status != "hold":
		blockers = append(blockers, fmt.Sprintf("关联部件 %s 当前状态为 %s，必须处于 hold（暂停）", part.Code, part.Status))
	}

	inspection, err := g.inspections.LatestByRelatedCode(ctx, code)
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		blockers = append(blockers, fmt.Sprintf("未找到关联部件 %s 对应的检查任务", code))
	case err != nil:
		return nil, fmt.Errorf("查询检查任务: %w", err)
	case inspection.Status != "passed":
		blockers = append(blockers, fmt.Sprintf("检查任务 %s 当前状态为 %s，必须为 passed（检查已通过）", inspection.Code, inspection.Status))
	}

	certificate, err := g.certificates.LatestByRelatedCode(ctx, code)
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		blockers = append(blockers, fmt.Sprintf("未找到关联部件 %s 对应的证书记录", code))
	case err != nil:
		return nil, fmt.Errorf("查询证书记录: %w", err)
	default:
		if certificate.Status != "valid" {
			blockers = append(blockers, fmt.Sprintf("证书 %s 当前状态为 %s，必须为 valid（有效）", certificate.Code, certificate.Status))
		}
		if certificate.EffectiveAt.After(time.Now().UTC()) {
			blockers = append(blockers, fmt.Sprintf("证书 %s 至 %s 才生效，当前不在有效期内", certificate.Code, certificate.EffectiveAt.Format("2006-01-02 15:04 UTC")))
		}
	}
	return blockers, nil
}
