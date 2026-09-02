package postgres

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providersettings"
)

func TestTextProviderSettingRepository_Integration(t *testing.T) {
	pool := integrationPool(t)
	repo := NewTextProviderSettingRepository(pool)
	ctx := context.Background()

	ownerA := uuid.New()
	ownerB := uuid.New()
	providerA := providers.ProviderID("openai")
	providerB := providers.ProviderID("anthropic")

	plaintextSentinel := "sk-super-secret-creator-key-999"
	fakeCiphertext := []byte{0xde, 0xad, 0xbe, 0xef, 0x01, 0x02}
	fakeNonce := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b}

	t.Run("create initial setting with revision 1", func(t *testing.T) {
		setting := providersettings.Setting{
			OwnerID:          ownerA,
			ProviderID:       providerA,
			Enabled:          true,
			EnabledTextModelIDs:  []providers.ModelID{"gpt-5-mini", "gpt-4o"},
			APIKeyCiphertext: fakeCiphertext,
			APIKeyNonce:      fakeNonce,
			KeyVersion:       "v1",
		}

		created, err := repo.Save(ctx, setting, nil)
		if err != nil {
			t.Fatalf("unexpected save error: %v", err)
		}
		if created.Revision != 1 {
			t.Fatalf("expected revision 1, got %d", created.Revision)
		}
		if !created.Enabled || len(created.EnabledTextModelIDs) != 2 {
			t.Fatalf("unexpected created setting fields: %+v", created)
		}

		// Plaintext sentinel is absent from stored ciphertext in DB
		var dbCiphertext []byte
		err = pool.QueryRow(ctx, `SELECT api_key_ciphertext FROM provider_settings WHERE owner_id = $1 AND provider_id = $2`, ownerA, string(providerA)).Scan(&dbCiphertext)
		if err != nil {
			t.Fatalf("query db ciphertext: %v", err)
		}
		if bytes.Contains(dbCiphertext, []byte(plaintextSentinel)) {
			t.Fatalf("database row contains plaintext sentinel!")
		}
	})

	t.Run("duplicate initial insert returns ErrStaleRevision", func(t *testing.T) {
		setting := providersettings.Setting{
			OwnerID:          ownerA,
			ProviderID:       providerA,
			Enabled:          true,
			EnabledTextModelIDs:  []providers.ModelID{"gpt-5-mini"},
			APIKeyCiphertext: fakeCiphertext,
			APIKeyNonce:      fakeNonce,
			KeyVersion:       "v1",
		}
		_, err := repo.Save(ctx, setting, nil)
		if !errors.Is(err, providersettings.ErrStaleRevision) {
			t.Fatalf("expected ErrStaleRevision on duplicate initial insert, got %v", err)
		}
	})

	t.Run("get and list by owner", func(t *testing.T) {
		// Owner A has providerA
		s, err := repo.GetByOwnerAndProvider(ctx, ownerA, providerA)
		if err != nil {
			t.Fatalf("get ownerA providerA: %v", err)
		}
		if s.Revision != 1 || s.ProviderID != providerA {
			t.Fatalf("unexpected setting: %+v", s)
		}

		// Owner B gets NotFound for providerA
		_, err = repo.GetByOwnerAndProvider(ctx, ownerB, providerA)
		if !errors.Is(err, providersettings.ErrSettingNotFound) {
			t.Fatalf("expected ErrSettingNotFound for ownerB, got %v", err)
		}

		// Add providerB for ownerA
		_, err = repo.Save(ctx, providersettings.Setting{
			OwnerID:          ownerA,
			ProviderID:       providerB,
			Enabled:          false,
			EnabledTextModelIDs:  []providers.ModelID{"claude-3-5-sonnet"},
			APIKeyCiphertext: fakeCiphertext,
			APIKeyNonce:      fakeNonce,
			KeyVersion:       "v1",
		}, nil)
		if err != nil {
			t.Fatalf("save providerB: %v", err)
		}

		listA, err := repo.ListByOwner(ctx, ownerA)
		if err != nil {
			t.Fatalf("list ownerA: %v", err)
		}
		if len(listA) != 2 {
			t.Fatalf("expected 2 settings for ownerA, got %d", len(listA))
		}

		listB, err := repo.ListByOwner(ctx, ownerB)
		if err != nil {
			t.Fatalf("list ownerB: %v", err)
		}
		if len(listB) != 0 {
			t.Fatalf("expected 0 settings for ownerB, got %d", len(listB))
		}
	})

	t.Run("optimistic update with revision", func(t *testing.T) {
		expectedRev := 1
		newCiphertext := []byte{0xca, 0xfe, 0xba, 0xbe}
		updated, err := repo.Save(ctx, providersettings.Setting{
			OwnerID:          ownerA,
			ProviderID:       providerA,
			Enabled:          false,
			EnabledTextModelIDs:  []providers.ModelID{"gpt-4o"},
			APIKeyCiphertext: newCiphertext,
			APIKeyNonce:      fakeNonce,
			KeyVersion:       "v1",
		}, &expectedRev)
		if err != nil {
			t.Fatalf("update with rev 1: %v", err)
		}
		if updated.Revision != 2 {
			t.Fatalf("expected revision 2, got %d", updated.Revision)
		}
		if updated.Enabled {
			t.Fatalf("expected enabled=false after update")
		}

		// Stale update with old revision 1 returns ErrStaleRevision
		_, err = repo.Save(ctx, providersettings.Setting{
			OwnerID:          ownerA,
			ProviderID:       providerA,
			Enabled:          true,
			EnabledTextModelIDs:  []providers.ModelID{"gpt-4o"},
			APIKeyCiphertext: newCiphertext,
			APIKeyNonce:      fakeNonce,
			KeyVersion:       "v1",
		}, &expectedRev)
		if !errors.Is(err, providersettings.ErrStaleRevision) {
			t.Fatalf("expected ErrStaleRevision on stale update, got %v", err)
		}
	})

	t.Run("delete with revision", func(t *testing.T) {
		// Wrong revision returns ErrStaleRevision
		err := repo.Delete(ctx, ownerA, providerA, 1)
		if !errors.Is(err, providersettings.ErrStaleRevision) {
			t.Fatalf("expected ErrStaleRevision for delete with rev 1, got %v", err)
		}

		// Correct revision 2 succeeds
		err = repo.Delete(ctx, ownerA, providerA, 2)
		if err != nil {
			t.Fatalf("delete with rev 2: %v", err)
		}

		// Subsequent get returns ErrSettingNotFound
		_, err = repo.GetByOwnerAndProvider(ctx, ownerA, providerA)
		if !errors.Is(err, providersettings.ErrSettingNotFound) {
			t.Fatalf("expected ErrSettingNotFound after delete, got %v", err)
		}

		// Delete on non-existent setting returns ErrSettingNotFound
		err = repo.Delete(ctx, ownerA, providerA, 2)
		if !errors.Is(err, providersettings.ErrSettingNotFound) {
			t.Fatalf("expected ErrSettingNotFound on second delete, got %v", err)
		}
	})

	t.Run("concurrent same-revision updates have exactly one winner", func(t *testing.T) {
		concurrentOwner := uuid.New()
		concurrentProvider := providers.ProviderID("openai")

		initial, err := repo.Save(ctx, providersettings.Setting{
			OwnerID:          concurrentOwner,
			ProviderID:       concurrentProvider,
			Enabled:          true,
			EnabledTextModelIDs:  []providers.ModelID{"gpt-5-mini"},
			APIKeyCiphertext: fakeCiphertext,
			APIKeyNonce:      fakeNonce,
			KeyVersion:       "v1",
		}, nil)
		if err != nil {
			t.Fatalf("create initial setting for concurrency test: %v", err)
		}
		if initial.Revision != 1 {
			t.Fatalf("expected revision 1, got %d", initial.Revision)
		}

		concurrency := 10
		type result struct {
			setting providersettings.Setting
			err     error
		}
		results := make(chan result, concurrency)
		baseRev := 1

		for i := 0; i < concurrency; i++ {
			go func(idx int) {
				expectedRev := baseRev
				res, sErr := repo.Save(ctx, providersettings.Setting{
					OwnerID:          concurrentOwner,
					ProviderID:       concurrentProvider,
					Enabled:          idx%2 == 0,
					EnabledTextModelIDs:  []providers.ModelID{"gpt-5-mini"},
					APIKeyCiphertext: []byte{byte(idx)},
					APIKeyNonce:      fakeNonce,
					KeyVersion:       "v1",
				}, &expectedRev)
				results <- result{setting: res, err: sErr}
			}(i)
		}

		var successCount, staleCount, otherCount int
		for i := 0; i < concurrency; i++ {
			r := <-results
			if r.err == nil {
				successCount++
				if r.setting.Revision != 2 {
					t.Errorf("expected winner revision 2, got %d", r.setting.Revision)
				}
			} else if errors.Is(r.err, providersettings.ErrStaleRevision) {
				staleCount++
			} else {
				otherCount++
				t.Errorf("unexpected error in concurrent save: %v", r.err)
			}
		}

		if successCount != 1 {
			t.Fatalf("expected exactly 1 winner, got %d (stale: %d, other: %d)", successCount, staleCount, otherCount)
		}
		if staleCount != concurrency-1 {
			t.Fatalf("expected %d stale competitors, got %d", concurrency-1, staleCount)
		}

		// Final check from DB confirms revision is 2
		finalSetting, err := repo.GetByOwnerAndProvider(ctx, concurrentOwner, concurrentProvider)
		if err != nil {
			t.Fatalf("get final setting: %v", err)
		}
		if finalSetting.Revision != 2 {
			t.Fatalf("expected final revision 2 in DB, got %d", finalSetting.Revision)
		}
	})
}
