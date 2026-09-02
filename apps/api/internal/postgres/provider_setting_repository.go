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

type ProviderSettingRepository struct {
	pool *pgxpool.Pool
}

func NewProviderSettingRepository(pool *pgxpool.Pool) *ProviderSettingRepository {
	return &ProviderSettingRepository{pool: pool}
}

// Legacy alias for backward compatibility during migration
func NewTextProviderSettingRepository(pool *pgxpool.Pool) *ProviderSettingRepository {
	return NewProviderSettingRepository(pool)
}

func (r *ProviderSettingRepository) ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]providersettings.Setting, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT owner_id, provider_id, revision, enabled,
		       enabled_text_model_ids, enabled_image_model_ids, enabled_tts_model_ids, enabled_voice_ids,
		       api_key_ciphertext, api_key_nonce, key_version, created_at, updated_at
		FROM provider_settings
		WHERE owner_id = $1
		ORDER BY provider_id ASC
	`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list provider settings: %w", err)
	}
	defer rows.Close()

	var settings []providersettings.Setting
	for rows.Next() {
		s, err := scanSetting(rows)
		if err != nil {
			return nil, fmt.Errorf("scan provider setting: %w", err)
		}
		settings = append(settings, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate provider settings: %w", err)
	}
	return settings, nil
}

func (r *ProviderSettingRepository) GetByOwnerAndProvider(ctx context.Context, ownerID uuid.UUID, providerID providers.ProviderID) (providersettings.Setting, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT owner_id, provider_id, revision, enabled,
		       enabled_text_model_ids, enabled_image_model_ids, enabled_tts_model_ids, enabled_voice_ids,
		       api_key_ciphertext, api_key_nonce, key_version, created_at, updated_at
		FROM provider_settings
		WHERE owner_id = $1 AND provider_id = $2
	`, ownerID, string(providerID))

	s, err := scanSetting(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return providersettings.Setting{}, providersettings.ErrSettingNotFound
		}
		return providersettings.Setting{}, fmt.Errorf("get provider setting: %w", err)
	}
	return s, nil
}

func (r *ProviderSettingRepository) Save(ctx context.Context, setting providersettings.Setting, expectedRevision *int) (providersettings.Setting, error) {
	enabledTextModelIDsJSON, err := json.Marshal(setting.EnabledTextModelIDs)
	if err != nil {
		return providersettings.Setting{}, fmt.Errorf("marshal enabled text model ids: %w", err)
	}
	enabledImageModelIDsJSON, err := json.Marshal(setting.EnabledImageModelIDs)
	if err != nil {
		return providersettings.Setting{}, fmt.Errorf("marshal enabled image model ids: %w", err)
	}
	enabledTTSModelIDsJSON, err := json.Marshal(setting.EnabledTTSModelIDs)
	if err != nil {
		return providersettings.Setting{}, fmt.Errorf("marshal enabled tts model ids: %w", err)
	}
	enabledVoiceIDsJSON, err := json.Marshal(setting.EnabledVoiceIDs)
	if err != nil {
		return providersettings.Setting{}, fmt.Errorf("marshal enabled voice ids: %w", err)
	}

	if expectedRevision == nil {
		var created providersettings.Setting
		created = setting
		created.Revision = 1

		row := r.pool.QueryRow(ctx, `
			INSERT INTO provider_settings (
				owner_id, provider_id, revision, enabled,
				enabled_text_model_ids, enabled_image_model_ids, enabled_tts_model_ids, enabled_voice_ids,
				api_key_ciphertext, api_key_nonce, key_version, created_at, updated_at
			) VALUES (
				$1, $2, 1, $3, $4, $5, $6, $7, $8, $9, $10, now(), now()
			)
			RETURNING created_at, updated_at
		`, setting.OwnerID, string(setting.ProviderID), setting.Enabled,
			enabledTextModelIDsJSON, enabledImageModelIDsJSON, enabledTTSModelIDsJSON, enabledVoiceIDsJSON,
			setting.APIKeyCiphertext, setting.APIKeyNonce, setting.KeyVersion)

		if err := row.Scan(&created.CreatedAt, &created.UpdatedAt); err != nil {
			if isUniqueViolation(err) {
				return providersettings.Setting{}, providersettings.ErrStaleRevision
			}
			return providersettings.Setting{}, fmt.Errorf("insert provider setting: %w", err)
		}
		return created, nil
	}

	var updated providersettings.Setting
	updated = setting
	updated.Revision = *expectedRevision + 1

	row := r.pool.QueryRow(ctx, `
		UPDATE provider_settings
		SET revision = revision + 1,
		    enabled = $3,
		    enabled_text_model_ids = $4,
		    enabled_image_model_ids = $5,
		    enabled_tts_model_ids = $6,
		    enabled_voice_ids = $7,
		    api_key_ciphertext = $8,
		    api_key_nonce = $9,
		    key_version = $10,
		    updated_at = now()
		WHERE owner_id = $1 AND provider_id = $2 AND revision = $11
		RETURNING created_at, updated_at
	`, setting.OwnerID, string(setting.ProviderID), setting.Enabled,
		enabledTextModelIDsJSON, enabledImageModelIDsJSON, enabledTTSModelIDsJSON, enabledVoiceIDsJSON,
		setting.APIKeyCiphertext, setting.APIKeyNonce, setting.KeyVersion, *expectedRevision)

	if err := row.Scan(&updated.CreatedAt, &updated.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			var exists bool
			if chkErr := r.pool.QueryRow(ctx, `
				SELECT EXISTS(SELECT 1 FROM provider_settings WHERE owner_id = $1 AND provider_id = $2)
			`, setting.OwnerID, string(setting.ProviderID)).Scan(&exists); chkErr == nil && exists {
				return providersettings.Setting{}, providersettings.ErrStaleRevision
			}
			return providersettings.Setting{}, providersettings.ErrSettingNotFound
		}
		return providersettings.Setting{}, fmt.Errorf("update provider setting: %w", err)
	}

	return updated, nil
}

func (r *ProviderSettingRepository) Delete(ctx context.Context, ownerID uuid.UUID, providerID providers.ProviderID, expectedRevision int) error {
	cmdTag, err := r.pool.Exec(ctx, `
		DELETE FROM provider_settings
		WHERE owner_id = $1 AND provider_id = $2 AND revision = $3
	`, ownerID, string(providerID), expectedRevision)
	if err != nil {
		return fmt.Errorf("delete provider setting: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		var exists bool
		if chkErr := r.pool.QueryRow(ctx, `
			SELECT EXISTS(SELECT 1 FROM provider_settings WHERE owner_id = $1 AND provider_id = $2)
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
	var enabledTextModelIDsRaw, enabledImageModelIDsRaw, enabledTTSModelIDsRaw, enabledVoiceIDsRaw []byte

	if err := scanner.Scan(
		&s.OwnerID,
		&providerIDStr,
		&s.Revision,
		&s.Enabled,
		&enabledTextModelIDsRaw,
		&enabledImageModelIDsRaw,
		&enabledTTSModelIDsRaw,
		&enabledVoiceIDsRaw,
		&s.APIKeyCiphertext,
		&s.APIKeyNonce,
		&s.KeyVersion,
		&s.CreatedAt,
		&s.UpdatedAt,
	); err != nil {
		return providersettings.Setting{}, err
	}

	s.ProviderID = providers.ProviderID(providerIDStr)

	if len(enabledTextModelIDsRaw) > 0 {
		if err := json.Unmarshal(enabledTextModelIDsRaw, &s.EnabledTextModelIDs); err != nil {
			return providersettings.Setting{}, fmt.Errorf("unmarshal enabled_text_model_ids: %w", err)
		}
	} else {
		s.EnabledTextModelIDs = []providers.ModelID{}
	}

	if len(enabledImageModelIDsRaw) > 0 {
		if err := json.Unmarshal(enabledImageModelIDsRaw, &s.EnabledImageModelIDs); err != nil {
			return providersettings.Setting{}, fmt.Errorf("unmarshal enabled_image_model_ids: %w", err)
		}
	} else {
		s.EnabledImageModelIDs = []providers.ModelID{}
	}

	if len(enabledTTSModelIDsRaw) > 0 {
		if err := json.Unmarshal(enabledTTSModelIDsRaw, &s.EnabledTTSModelIDs); err != nil {
			return providersettings.Setting{}, fmt.Errorf("unmarshal enabled_tts_model_ids: %w", err)
		}
	} else {
		s.EnabledTTSModelIDs = []providers.ModelID{}
	}

	if len(enabledVoiceIDsRaw) > 0 {
		if err := json.Unmarshal(enabledVoiceIDsRaw, &s.EnabledVoiceIDs); err != nil {
			return providersettings.Setting{}, fmt.Errorf("unmarshal enabled_voice_ids: %w", err)
		}
	} else {
		s.EnabledVoiceIDs = []providers.VoiceID{}
	}

	return s, nil
}
