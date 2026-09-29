package data

import (
	"context"
	"time"

	entCrud "github.com/tx7do/go-crud/entgo"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"github.com/go-tangra/go-tangra-executor/internal/data/ent"
	"github.com/go-tangra/go-tangra-executor/internal/data/ent/setting"
)

// SettingRepo stores platform-wide executor settings as JSON documents.
type SettingRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
}

// NewSettingRepo creates a SettingRepo.
func NewSettingRepo(_ *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *SettingRepo {
	return &SettingRepo{entClient: entClient}
}

// StoredSetting is one setting row.
type StoredSetting struct {
	Value      string
	UpdateTime time.Time
	UpdatedBy  *uint32
}

// Get returns the setting stored under key (found false when absent).
func (r *SettingRepo) Get(ctx context.Context, key string) (StoredSetting, bool, error) {
	e, err := r.entClient.Client().Setting.Query().Where(setting.ID(key)).Only(ctx)
	if ent.IsNotFound(err) {
		return StoredSetting{}, false, nil
	}
	if err != nil {
		return StoredSetting{}, false, err
	}
	return StoredSetting{Value: e.Value, UpdateTime: e.UpdateTime, UpdatedBy: e.UpdatedBy}, true, nil
}

// Put stores value under key (insert or replace).
func (r *SettingRepo) Put(ctx context.Context, key, value string, updatedBy uint32) error {
	c := r.entClient.Client().Setting.Create().SetID(key).SetValue(value).SetUpdateTime(time.Now())
	if updatedBy != 0 {
		c.SetUpdatedBy(updatedBy)
	}
	return c.OnConflictColumns(setting.FieldID).UpdateNewValues().Exec(ctx)
}
