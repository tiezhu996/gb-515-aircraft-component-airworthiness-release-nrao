package repository

import (
	"context"

	"github.com/blueship581/aircraft-component-airworthiness-release/backend/internal/dto"
	"github.com/blueship581/aircraft-component-airworthiness-release/backend/internal/model"
	"gorm.io/gorm"
)

// CertificateRecordRepository owns all persistence operations for 证书记录.
type CertificateRecordRepository interface {
	List(context.Context, dto.PageQuery) (Page[model.CertificateRecord], error)
	Get(context.Context, uint) (model.CertificateRecord, error)
	CreateVersion(context.Context, *model.CertificateRecord, string, string) error
	UpdateVersion(context.Context, uint, uint, *model.CertificateRecord, string, string, string, string, string) error
	Delete(context.Context, uint) error
	CountByStatus(context.Context) (map[string]int64, error)
}

type certificateRecordRepository struct {
	store *Store[model.CertificateRecord]
	db    *gorm.DB
}

func NewCertificateRecordRepository(db *gorm.DB) CertificateRecordRepository {
	return &certificateRecordRepository{store: NewStore[model.CertificateRecord](db), db: db}
}

func (r *certificateRecordRepository) List(ctx context.Context, q dto.PageQuery) (Page[model.CertificateRecord], error) {
	page, err := r.store.List(ctx, q)
	if err != nil || len(page.Items) == 0 {
		return page, err
	}
	ids := make([]uint, 0, len(page.Items))
	for _, item := range page.Items {
		ids = append(ids, item.ID)
	}
	var revisions []model.CertificateRecordRevision
	if err := r.db.WithContext(ctx).Where("certificate_record_id IN ?", ids).
		Order("certificate_record_id, version").Find(&revisions).Error; err != nil {
		return Page[model.CertificateRecord]{}, err
	}
	byRecord := make(map[uint][]model.CertificateRecordRevision)
	for _, revision := range revisions {
		byRecord[revision.CertificateRecordID] = append(byRecord[revision.CertificateRecordID], revision)
	}
	for index := range page.Items {
		page.Items[index].Revisions = byRecord[page.Items[index].ID]
	}
	return page, nil
}
func (r *certificateRecordRepository) Get(ctx context.Context, id uint) (model.CertificateRecord, error) {
	var item model.CertificateRecord
	err := r.db.WithContext(ctx).Preload("Revisions", func(db *gorm.DB) *gorm.DB {
		return db.Order("version")
	}).First(&item, id).Error
	return item, err
}

func (r *certificateRecordRepository) CreateVersion(ctx context.Context, item *model.CertificateRecord, actor, requestID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit("Revisions").Create(item).Error; err != nil {
			return err
		}
		revision := model.CertificateRecordRevision{
			CertificateRecordID: item.ID, Version: item.Version, Status: item.Status,
			Evidence: item.Evidence, Actor: actor, RequestID: requestID, Action: "create",
			Reason: "certificate prepared", CreatedAt: item.CreatedAt,
		}
		if err := tx.Create(&revision).Error; err != nil {
			return err
		}
		return appendAudit(tx, actor, requestID, "create", "CertificateRecord", item.ID, "", item.Status, "certificate version 1 prepared")
	})
}

func (r *certificateRecordRepository) UpdateVersion(ctx context.Context, id, expectedVersion uint, item *model.CertificateRecord, actor, requestID, action, before, reason string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		item.Revisions = nil
		if err := optimisticUpdate(tx, id, expectedVersion, item); err != nil {
			return err
		}
		revision := model.CertificateRecordRevision{
			CertificateRecordID: id, Version: item.Version, Status: item.Status,
			Evidence: item.Evidence, Actor: actor, RequestID: requestID, Action: action,
			Reason: reason, CreatedAt: item.UpdatedAt,
		}
		if err := tx.Create(&revision).Error; err != nil {
			return err
		}
		return appendAudit(tx, actor, requestID, action, "CertificateRecord", id, before, item.Status, reason)
	})
}
func (r *certificateRecordRepository) Delete(ctx context.Context, id uint) error {
	return r.store.Delete(ctx, id)
}
func (r *certificateRecordRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	return r.store.CountByStatus(ctx)
}
