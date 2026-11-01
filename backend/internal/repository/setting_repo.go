package repository

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/setting"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type settingRepository struct {
	client *ent.Client
}

func NewSettingRepository(client *ent.Client) service.SettingRepository {
	return &settingRepository{client: client}
}

func (r *settingRepository) Get(ctx context.Context, key string) (*service.Setting, error) {
	m, err := r.client.Setting.Query().Where(setting.KeyEQ(key)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, service.ErrSettingNotFound
		}
		return nil, err
	}
	return &service.Setting{
		ID:        m.ID,
		Key:       m.Key,
		Value:     m.Value,
		UpdatedAt: m.UpdatedAt,
	}, nil
}

func (r *settingRepository) GetValue(ctx context.Context, key string) (string, error) {
	setting, err := r.Get(ctx, key)
	if err != nil {
		return "", err
	}
	return setting.Value, nil
}

// GetValueWithRevision is the narrow compare-and-set read used by settings
// whose writes must not silently overwrite another administrator. The existing
// updated_at column is the revision; no schema migration is required.
func (r *settingRepository) GetValueWithRevision(ctx context.Context, key string) (string, string, error) {
	m, err := r.client.Setting.Query().Where(setting.KeyEQ(key)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return "", "", service.ErrSettingNotFound
		}
		return "", "", err
	}
	return m.Value, m.UpdatedAt.UTC().Format(time.RFC3339Nano), nil
}

// CompareAndSet atomically updates a setting only when both its previous value
// and revision still match. The unique key rejects competing initial inserts.
// Revisions use PostgreSQL microsecond precision and always advance on CAS.
func (r *settingRepository) CompareAndSet(ctx context.Context, key, expectedRevision, expectedValue, value string) (string, error) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	if expectedRevision == "" && expectedValue == "" {
		created, err := r.client.Setting.Create().
			SetKey(key).
			SetValue(value).
			SetUpdatedAt(now).
			Save(ctx)
		if err != nil {
			if ent.IsConstraintError(err) {
				return "", service.ErrModelBillingRevisionConflict
			}
			return "", err
		}
		return created.UpdatedAt.UTC().Format(time.RFC3339Nano), nil
	}

	previous := parseSettingRevision(expectedRevision)
	if !now.After(previous) {
		now = previous.Add(time.Microsecond)
	}
	n, err := r.client.Setting.Update().
		Where(setting.KeyEQ(key), setting.ValueEQ(expectedValue), setting.UpdatedAt(previous)).
		SetValue(value).
		SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return "", service.ErrModelBillingRevisionConflict
		}
		return "", err
	}
	if n != 1 {
		return "", service.ErrModelBillingRevisionConflict
	}
	return now.UTC().Format(time.RFC3339Nano), nil
}

func parseSettingRevision(revision string) time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, revision)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func (r *settingRepository) Set(ctx context.Context, key, value string) error {
	now := time.Now()
	return r.client.Setting.
		Create().
		SetKey(key).
		SetValue(value).
		SetUpdatedAt(now).
		OnConflictColumns(setting.FieldKey).
		UpdateNewValues().
		Exec(ctx)
}

func (r *settingRepository) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	if len(keys) == 0 {
		return map[string]string{}, nil
	}
	settings, err := r.client.Setting.Query().Where(setting.KeyIn(keys...)).All(ctx)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, s := range settings {
		result[s.Key] = s.Value
	}
	return result, nil
}

func (r *settingRepository) SetMultiple(ctx context.Context, settings map[string]string) error {
	if len(settings) == 0 {
		return nil
	}

	now := time.Now()
	builders := make([]*ent.SettingCreate, 0, len(settings))
	for key, value := range settings {
		builders = append(builders, r.client.Setting.Create().SetKey(key).SetValue(value).SetUpdatedAt(now))
	}
	return r.client.Setting.
		CreateBulk(builders...).
		OnConflictColumns(setting.FieldKey).
		UpdateNewValues().
		Exec(ctx)
}

func (r *settingRepository) GetAll(ctx context.Context) (map[string]string, error) {
	settings, err := r.client.Setting.Query().All(ctx)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, s := range settings {
		result[s.Key] = s.Value
	}
	return result, nil
}

func (r *settingRepository) Delete(ctx context.Context, key string) error {
	_, err := r.client.Setting.Delete().Where(setting.KeyEQ(key)).Exec(ctx)
	return err
}
