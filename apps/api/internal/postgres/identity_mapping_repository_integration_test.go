package postgres

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/identity"
)

func TestIdentityMappingRepositoryConcurrentFirstLoginCreatesSingleOwner(t *testing.T) {
	pool := integrationPool(t)
	repository := NewIdentityMappingRepository(pool)
	external := identity.ExternalIdentity{
		Issuer:  "https://issuer.test.example",
		Subject: "subject-concurrent",
	}

	const workers = 8
	results := make([]uuid.UUID, workers)
	errs := make([]error, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(index int) {
			defer wg.Done()
			results[index], errs[index] = repository.ResolveOrCreateOwner(context.Background(), external)
		}(i)
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			t.Fatalf("resolve or create owner: %v", err)
		}
	}
	first := results[0]
	for _, ownerID := range results[1:] {
		if ownerID != first {
			t.Fatalf("expected one owner mapping, got %s and %s", first, ownerID)
		}
	}
}

func TestIdentityMappingRepositoryDoesNotAliasDifferentIssuerOrSubject(t *testing.T) {
	pool := integrationPool(t)
	repository := NewIdentityMappingRepository(pool)

	ownerA, err := repository.ResolveOrCreateOwner(context.Background(), identity.ExternalIdentity{
		Issuer:  "https://issuer-a.test",
		Subject: "shared-subject",
	})
	if err != nil {
		t.Fatalf("resolve owner A: %v", err)
	}
	ownerB, err := repository.ResolveOrCreateOwner(context.Background(), identity.ExternalIdentity{
		Issuer:  "https://issuer-b.test",
		Subject: "shared-subject",
	})
	if err != nil {
		t.Fatalf("resolve owner B: %v", err)
	}
	ownerC, err := repository.ResolveOrCreateOwner(context.Background(), identity.ExternalIdentity{
		Issuer:  "https://issuer-a.test",
		Subject: "other-subject",
	})
	if err != nil {
		t.Fatalf("resolve owner C: %v", err)
	}

	if ownerA == ownerB || ownerA == ownerC || ownerB == ownerC {
		t.Fatalf("expected distinct owners, got %s %s %s", ownerA, ownerB, ownerC)
	}
}

func TestIdentityMappingRepositoryReturnsExistingMapping(t *testing.T) {
	pool := integrationPool(t)
	repository := NewIdentityMappingRepository(pool)
	external := identity.ExternalIdentity{
		Issuer:  "https://issuer.test.example",
		Subject: "stable-subject",
	}

	first, err := repository.ResolveOrCreateOwner(context.Background(), external)
	if err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	second, err := repository.ResolveOrCreateOwner(context.Background(), external)
	if err != nil {
		t.Fatalf("second resolve: %v", err)
	}
	if first != second {
		t.Fatalf("expected stable mapping, got %s then %s", first, second)
	}
}
