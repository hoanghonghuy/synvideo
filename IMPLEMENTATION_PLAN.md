# TASK-028 Implementation Plan

## Current State Analysis

### Text-Only BYOK (TASK-017)
- **Table**: `text_provider_settings` with `enabled_model_ids jsonb`
- **Catalog**: text-only `ProviderDefinition` + `ModelDefinition`
- **Setting**: single `[]ModelID` for text models
- **Service**: `ResolveTextGenerator` per owner
- **HTTP**: `/api/v1/ai/provider-settings` (list/put/delete)
- **Frontend**: `ProviderSettingsView.vue` with model checkboxes

### New Capabilities (TASK-025/026/027)
- **Providers**: `ImageGenerator`, `SpeechSynthesizer` ports
- **Registry**: multi-capability `Registration`, `ResolveImageGenerator`, `ResolveSpeechSynthesizer`
- **Adapters**: `openaiimage`, `openaitts` with isolated configs
- **Voice**: `VoiceID`, `VoiceMetadata` types exist

## Frozen Contract Requirements

1. **Migration**: backward-compatible, existing text settings remain usable
2. **Catalog**: capability-aware provider definitions (text/image/TTS models + voices)
3. **Settings**: persist enabled capabilities + model/voice selections per capability
4. **Runtime**: owner-scoped resolvers for text/image/TTS
5. **Options**: safe endpoints for image/TTS generation options
6. **UI**: extend settings workspace for capability/model/voice selection
7. **Security**: no secret/external-ID leakage, fail-closed encryption unchanged

## Design Decisions

### Schema Strategy
Keep table name `text_provider_settings` → rename to `provider_settings` in migration.

**Columns**:
- Existing: `owner_id`, `provider_id`, `revision`, `enabled`, `api_key_ciphertext`, `api_key_nonce`, `key_version`, timestamps
- Add: `enabled_text_model_ids jsonb`, `enabled_image_model_ids jsonb`, `enabled_voice_ids jsonb`
- Migrate: `enabled_model_ids` → `enabled_text_model_ids` (preserve data)
- Remove: `enabled_model_ids` (after data copy)

Why separate columns per capability? Simpler than polymorphic `[(capability, id)]` array, explicit validation, backward-compatible query.

### Catalog Evolution
Extend `ProviderDefinition`:
```go
type ModelDefinition struct {
    ModelID         ModelID
    DisplayName     string
    ExternalModelID string
    Capabilities    []Capability  // NEW: text/image/both
}

type VoiceDefinition struct {  // NEW
    VoiceID       VoiceID
    DisplayName   string
    ExternalVoice string
    Locale        string
    Language      string
    Style         string
}

type ProviderDefinition struct {
    ProviderID   ProviderID
    DisplayName  string
    BaseURL      string
    Models       []ModelDefinition   // now multi-capability
    Voices       []VoiceDefinition   // NEW
    Timeout      time.Duration
    MaxResponseBytes int64
}
```

Catalog validates: one model can declare `[text_generation, image_generation]`, voices are TTS-specific.

### Setting Domain
```go
type Setting struct {
    OwnerID               uuid.UUID
    ProviderID            ProviderID
    Revision              int
    Enabled               bool
    EnabledTextModelIDs   []ModelID   // NEW
    EnabledImageModelIDs  []ModelID   // NEW
    EnabledVoiceIDs       []VoiceID   // NEW
    APIKeyCiphertext      []byte
    APIKeyNonce           []byte
    KeyVersion            string
    CreatedAt             time.Time
    UpdatedAt             time.Time
}
```

### Service API Evolution
```go
// Extend PutSettingInput
type PutSettingInput struct {
    Revision              *int
    Enabled               bool
    EnabledTextModelIDs   []ModelID  // NEW
    EnabledImageModelIDs  []ModelID  // NEW
    EnabledVoiceIDs       []VoiceID  // NEW
    APIKey                *string
}

// NEW: image generation options
type ImageGenerationOptionsResponse struct {
    Providers []ImageGenerationOptionProvider
}

// NEW: TTS options
type TTSOptionsResponse struct {
    Providers []TTSOptionProvider
}

// NEW runtime resolvers
func (s *Service) ResolveImageGenerator(ctx, ownerID, providerID, modelID) (ImageGenerator, error)
func (s *Service) ResolveSpeechSynthesizer(ctx, ownerID, providerID, modelID) (SpeechSynthesizer, error)
```

### HTTP Routes
- **Existing**: `GET /api/v1/ai/provider-settings` → extend response with capabilities/voices
- **Existing**: `PUT /api/v1/ai/provider-settings/{provider_id}` → accept new fields
- **Existing**: `DELETE /api/v1/ai/provider-settings/{provider_id}`
- **Existing**: `GET /api/v1/ai/text-generation-options` → unchanged
- **NEW**: `GET /api/v1/ai/image-generation-options`
- **NEW**: `GET /api/v1/ai/tts-options`

### Frontend Evolution
Extend `ProviderSettingsView.vue`:
- Group models by capability (Text Models, Image Models)
- Add voice selection grid for TTS
- Safe metadata only, no external IDs

## Implementation Steps (TDD)

### Phase 1: Migration + Repository (RED → GREEN)
1. **Migration**: `0014_add_provider_capabilities.sql`
   - Rename table
   - Add `enabled_text_model_ids`, `enabled_image_model_ids`, `enabled_voice_ids`
   - Migrate `enabled_model_ids` → `enabled_text_model_ids`
   - Drop `enabled_model_ids`

2. **Repository**: extend `TextProviderSettingRepository` → `ProviderSettingRepository`
   - Update scanSetting for new columns
   - Test: existing text settings migrate correctly
   - Test: save/load text/image/voice selections

### Phase 2: Catalog + Service (RED → GREEN)
3. **Catalog**: extend `ProviderDefinition` with capabilities + voices
   - Add `ModelDefinition.Capabilities []Capability`
   - Add `VoiceDefinition` struct
   - Validation: duplicate voice IDs rejected
   - Test: multi-capability model catalog
   - Test: voice catalog validation

4. **Setting domain**: extend `Setting` + `PutSettingInput`
   - Add `EnabledTextModelIDs`, `EnabledImageModelIDs`, `EnabledVoiceIDs`
   - Test: validation per capability

5. **Service**: extend runtime resolvers
   - `ResolveImageGenerator(ctx, owner, provider, model) (ImageGenerator, error)`
   - `ResolveSpeechSynthesizer(ctx, owner, provider, model) (SpeechSynthesizer, error)`
   - Test: owner isolation for image/TTS
   - Test: disabled model/voice fails resolution
   - Test: live adapter construction with owner secret

### Phase 3: HTTP + Options (RED → GREEN)
6. **HTTP**: extend provider-settings endpoints
   - Extend `ProviderSettingView` with capability-grouped models + voices
   - Accept new `PutSettingInput` fields
   - Test: secret sentinel never leaks
   - Test: stale revision handling

7. **Options endpoints**: image + TTS
   - `GET /api/v1/ai/image-generation-options`
   - `GET /api/v1/ai/tts-options`
   - Test: only enabled owner models/voices returned
   - Test: no external IDs/secrets

### Phase 4: Frontend (RED → GREEN)
8. **UI**: extend `ProviderSettingsView.vue`
   - Capability-grouped model grids (Text/Image)
   - Voice selection grid with locale/language/style
   - Test: form state preserves selections
   - Test: API key cleared after save

### Phase 5: Integration + CI
9. **Main.go**: wire multi-capability catalog from config
   - Add `IMAGE_PROVIDER_DEFINITIONS`, `TTS_PROVIDER_DEFINITIONS` or unified JSON
   - Test: startup with multi-capability catalog

10. **Full verification**:
    - Real PostgreSQL migration integration test
    - Race tests
    - Backend full verify
    - Frontend full verify
    - CI green

## Acceptance Verification Checklist

Per contract TDD gates:
1. ✅ Migration/backward compatibility from TASK-017 text settings
2. ✅ Encryption, owner isolation and fail-closed regressions
3. ✅ Deployment catalog validation and duplicate-ID rejection
4. ✅ Image/TTS model and voice selection validation
5. ✅ Disabled selections fail before adapter use
6. ✅ Correct owner-specific secret reaches runtime adapter only at request time
7. ✅ Safe option/settings payloads contain no secret/external-ID sentinel
8. ✅ Existing text generation E2E/regressions remain green
9. ✅ Settings UI secret-preserve/rotate/delete and capability-selection flows
10. ✅ Race/full API/frontend verification and fresh PR CI

## Out of Scope (per contract)
- Durable generation jobs
- Media Asset ingestion/persistence
- Scene binding
- Video provider activation
- Caption/music/render/publish

## Risks + Mitigations
- **Risk**: breaking existing text Proposal/Script/ScenePlan generation
  - **Mitigation**: keep text resolver unchanged, add integration test for existing job flows
- **Risk**: migration fails on production data
  - **Mitigation**: test against real Postgres with existing TASK-017 seed data
- **Risk**: catalog config format change breaks deployment
  - **Mitigation**: support both legacy text-only + new multi-capability formats with graceful fallback

## Estimated Complexity
- Migration: 1 file, straightforward column addition + data copy
- Catalog: ~200 LOC (voice definitions, capability validation)
- Setting/Service: ~300 LOC (resolvers, options)
- HTTP: ~150 LOC (endpoints, safe views)
- Frontend: ~200 LOC (capability grids, voice selection)
- Tests: ~800 LOC (TDD coverage for all 10 gates)

**Total**: ~1650 LOC across backend + frontend + migration + tests. Expect 4–6 hours focused work following TDD RED → GREEN → REFACTOR.
