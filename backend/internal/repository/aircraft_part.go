package repository

import (
	"context"

	"github.com/blueship581/aircraft-component-airworthiness-release/backend/internal/dto"
	"github.com/blueship581/aircraft-component-airworthiness-release/backend/internal/model"
	"gorm.io/gorm"
)

// AircraftPartRepository owns all persistence operations for 航空部件.
type AircraftPartRepository interface {
	List(context.Context, dto.PageQuery) (Page[model.AircraftPart], error)
	Get(context.Context, uint) (model.AircraftPart, error)
	Create(context.Context, *model.AircraftPart) error
	Update(context.Context, uint, uint, *model.AircraftPart) error
	Delete(context.Context, uint) error
	CountByStatus(context.Context) (map[string]int64, error)
}

type aircraftPartRepository struct {
	store *Store[model.AircraftPart]
}

func NewAircraftPartRepository(db *gorm.DB) AircraftPartRepository {
	return &aircraftPartRepository{store: NewStore[model.AircraftPart](db)}
}

func (r *aircraftPartRepository) List(ctx context.Context, q dto.PageQuery) (Page[model.AircraftPart], error) {
	return r.store.List(ctx, q)
}
func (r *aircraftPartRepository) Get(ctx context.Context, id uint) (model.AircraftPart, error) {
	return r.store.Get(ctx, id)
}
func (r *aircraftPartRepository) Create(ctx context.Context, item *model.AircraftPart) error {
	return r.store.Create(ctx, item)
}
func (r *aircraftPartRepository) Update(ctx context.Context, id, version uint, item *model.AircraftPart) error {
	return r.store.Update(ctx, id, version, item)
}
func (r *aircraftPartRepository) Delete(ctx context.Context, id uint) error {
	return r.store.Delete(ctx, id)
}
func (r *aircraftPartRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	return r.store.CountByStatus(ctx)
}
