package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providersettings"
)

type TextProviderSettingRepository struct {
	pool *pgxpool.Pool
}

func NewTextProviderSettingRepository(pool *pgxpool.Pool) *TextProviderSettingRepository {
	return &TextProviderSettingRepository{pool: pool}
}

func (r *TextProviderSettingRepository) ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]providersettings.Setting, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT owner_id, provider_id, revision, enabled, enabled_model_ids,
		       api_key_ciphertext, api_key_nonce, key_version, created_at, updated_at
		FROM text_provider_settings
		WHERE owner_id = $1
		ORDER BY provider_id ASC
	`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list text provider settings: %w", err)
	}
	defer rows.Close()

	var settings []providersettings.Setting
	for rows.Next() {
		s, err := scanSetting(rows)
		if err != nil {
			return nil, fmt.Errorf("scan text provider setting: %w", err)
		}
		settings = append(settings, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate text provider settings: %w", err)
	}
	return settings, nil
}

func (r *TextProviderSettingRepository) GetByOwnerAndProvider(ctx context.Context, ownerID uuid.UUID, providerID providers.ProviderID) (providersettings.Setting, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT owner_id, provider_id, revision, enabled, enabled_model_ids,
		       api_key_ciphertext, api_key_nonce, key_version, created_at, updated_at
		FROM text_provider_settings
		WHERE owner_id = $1 AND provider_id = $2
	`, ownerID, string(providerID))

	s, err := scanSetting(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return providersettings.Setting{}, providersettings.ErrSettingNotFound
		}
		return providersettings.Setting{}, fmt.Errorf("get text provider setting: %w", err)
	}
	return s, nil
}

func (r *TextProviderSettingRepository) Save(ctx context.Context, setting providersettings.Setting, expectedRevision *int) (providersettings.Setting, error) {
	enabledModelIDsJSON, err := json.Marshal(setting.EnabledModelIDs)
	if err != nil {
		return providersettings.Setting{}, fmt.Errorf("marshal enabled model ids: %w", err)
	}

	if expectedRevision == nil {
		var created providersettings.Setting
		created = setting
		created.Revision = 1

		row := r.pool.QueryRow(ctx, `
			INSERT INTO text_provider_settings (
				owner_id, provider_id, revision, enabled, enabled_model_ids,
				api_key_ciphertext, api_key_nonce, key_version, created_at, updated_at
			) VALUES (
				$1, $2, 1, $3, $4, $5, $6, $7, now(), now()
			)
			RETURNING created_at, updated_at
		`, setting.OwnerID, string(setting.ProviderID), setting.Enabled, enabledModelIDsJSON,
			setting.APIKeyCiphertext, setting.APIKeyNonce, setting.KeyVersion)

		if err := row.Scan(&created.CreatedAt, &created.UpdatedAt); err != nil {
			if isUniqueViolation(err) {
				return providersettings.Setting{}, providersettings.ErrStaleRevision
			}
			return providersettings.Setting{}, fmt.Errorf("insert text provider setting: %w", err)
		}
		return created, nil
	}

	var updated providersettings.Setting
	updated = setting
	updated.Revision = *expectedRevision + 1

	row := r.pool.QueryRow(ctx, `
		UPDATE text_provider_settings
		SET revision = revision + 1,
		    enabled = $3,
		    enabled_model_ids = $4,
		    api_key_ciphertext = $5,
		    api_key_nonce = $6,
		    key_version = $7,
		    updated_at = now()
		WHERE owner_id = $1 AND provider_id = $2 AND revision = $8
		RETURNING created_at, updated_at
	`, setting.OwnerID, string(setting.ProviderID), setting.Enabled, enabledModelIDsJSON,
		setting.APIKeyCiphertext, setting.APIKeyNonce, setting.KeyVersion, *expectedRevision)

	if err := row.Scan(&updated.CreatedAt, &updated.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			var exists bool
			if chkErr := r.pool.QueryRow(ctx, `
				SELECT EXISTS(SELECT 1 FROM text_provider_settings WHERE owner_id = $1 AND provider_id = $2)
			`, setting.OwnerID, string(setting.ProviderID)).Scan(&exists); chkErr == nil && exists {
				return providersettings.Setting{}, providersettings.ErrStaleRevision
			}
			return providersettings.Setting{}, providersettings.ErrSettingNotFound
		}
		return providersettings.Setting{}, fmt.Errorf("update text provider setting: %w", err)
	}

	return updated, nil
}

func (r *TextProviderSettingRepository) Delete(ctx context.Context, ownerID uuid.UUID, providerID providers.ProviderID, expectedRevision int) error {
	cmdTag, err := r.pool.Exec(ctx, `
		DELETE FROM text_provider_settings
		WHERE owner_id = $1 AND provider_id = $2 AND revision = $3
	`, ownerID, string(providerID), expectedRevision)
	if err != nil {
		return fmt.Errorf("delete text provider setting: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		var exists bool
		if chkErr := r.pool.QueryRow(ctx, `
			SELECT EXISTS(SELECT 1 FROM text_provider_settings WHERE owner_id = $1 AND provider_id = $2)
		`, ownerID, string(providerID)).Scan(&exists); chkErr == nil && exists {
			return providersettings.ErrStaleRevision
		}
		return providersettings.ErrSettingNotFound
	}

	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSetting(scanner rowScanner) (providersettings.Setting, error) {
	var s providersettings.Setting
	var providerIDStr string
	var enabledModelIDsRaw []byte

	if err := scanner.Scan(
		&s.OwnerID,
		&providerIDStr,
		&s.Revision,
		&s.Enabled,
		&enabledModelIDsRaw,
		&s.APIKeyCiphertext,
		&s.APIKeyNonce,
		&s.KeyVersion,
		&s.CreatedAt,
		&s.UpdatedAt,
	); err != nil {
		return providersettings.Setting{}, err
	}

	s.ProviderID = providers.ProviderID(providerIDStr)
	if len(enabledModelIDsRaw) > 0 {
		if err := json.Unmarshal(enabledModelIDsRaw, &s.EnabledModelIDs); err != nil {
			return providersettings.Setting{}, fmt.Errorf("unmarshal enabled_model_ids: %w", err)
		}
	} else {
		s.EnabledModelIDs = []providers.ModelID{}
	}

	return s, nil
}
