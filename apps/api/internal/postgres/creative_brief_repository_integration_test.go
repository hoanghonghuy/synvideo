package postgres

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/creativebrief"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
)

func TestCreativeBriefRepositoryIntegrationCreateUpdateAndGet(t *testing.T) {
	pool := integrationPool(t)
	projectRepository := NewProjectRepository(pool)
	repository := NewCreativeBriefRepository(pool)
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	projectItem, err := projectRepository.Create(context.Background(), ownerID, validIntegrationCreateInput("Brief project"))
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	created, wasCreated, err := repository.Put(context.Background(), ownerID, projectItem.ID, validBriefInput("First intent"))
	if err != nil {
		t.Fatalf("create brief: %v", err)
	}
	if !wasCreated {
		t.Fatal("expected first put to create")
	}
	if created.Revision != 1 {
		t.Fatalf("expected revision 1, got %d", created.Revision)
	}
	if created.ProjectID != projectItem.ID || created.SourceText != "First intent" {
		t.Fatalf("unexpected created brief: %#v", created)
	}

	currentRevision := created.Revision
	updateInput := validBriefInput("Updated intent")
	updateInput.Revision = &currentRevision
	updated, wasCreated, err := repository.Put(context.Background(), ownerID, projectItem.ID, updateInput)
	if err != nil {
		t.Fatalf("update brief: %v", err)
	}
	if wasCreated {
		t.Fatal("expected existing put to update")
	}
	if updated.Revision != 2 {
		t.Fatalf("expected revision 2, got %d", updated.Revision)
	}
	if updated.SourceText != "Updated intent" {
		t.Fatalf("expected updated source_text, got %q", updated.SourceText)
	}

	got, err := repository.Get(context.Background(), ownerID, projectItem.ID)
	if err != nil {
		t.Fatalf("get brief: %v", err)
	}
	if got.Revision != updated.Revision || got.SourceText != updated.SourceText {
		t.Fatalf("unexpected fetched brief: %#v", got)
	}

	staleInput := validBriefInput("Stale overwrite")
	staleInput.Revision = &currentRevision
	if _, _, err := repository.Put(context.Background(), ownerID, projectItem.ID, staleInput); !errors.Is(err, creativebrief.ErrStaleRevision) {
		t.Fatalf("expected stale revision error, got %v", err)
	}

	if _, _, err := repository.Put(context.Background(), ownerID, projectItem.ID, validBriefInput("Missing revision")); !errors.Is(err, creativebrief.ErrRevisionRequired) {
		t.Fatalf("expected revision required error, got %v", err)
	}
}

func TestCreativeBriefRepositoryIntegrationCreateWithOnlyRequiredField(t *testing.T) {
	pool := integrationPool(t)
	projectRepository := NewProjectRepository(pool)
	repository := NewCreativeBriefRepository(pool)
	ownerID := uuid.MustParse("55555555-5555-4555-8555-555555555555")
	projectItem, err := projectRepository.Create(context.Background(), ownerID, validIntegrationCreateInput("Minimal brief project"))
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	created, wasCreated, err := repository.Put(context.Background(), ownerID, projectItem.ID, creativebrief.PutInput{
		SourceText: "Only required creator intent",
	})
	if err != nil {
		t.Fatalf("create minimal brief: %v", err)
	}
	if !wasCreated || created.Revision != 1 {
		t.Fatalf("expected created revision 1, got created=%t brief=%#v", wasCreated, created)
	}
	if len(created.DistributionTargets) != 0 || len(created.MustInclude) != 0 || len(created.MustAvoid) != 0 {
		t.Fatalf("expected omitted arrays to persist empty, got %#v", created)
	}
}

func TestCreativeBriefRepositoryIntegrationRejectsRevisionWhenCreatingFirstBrief(t *testing.T) {
	pool := integrationPool(t)
	projectRepository := NewProjectRepository(pool)
	repository := NewCreativeBriefRepository(pool)
	ownerID := uuid.MustParse("66666666-6666-4666-8666-666666666666")
	projectItem, err := projectRepository.Create(context.Background(), ownerID, validIntegrationCreateInput("Revision create project"))
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	revision := 1
	input := validBriefInput("First intent with revision")
	input.Revision = &revision
	if _, _, err := repository.Put(context.Background(), ownerID, projectItem.ID, input); !errors.Is(err, creativebrief.ErrRevisionUnexpected) {
		t.Fatalf("expected revision unexpected error, got %v", err)
	}
}

func TestCreativeBriefRepositoryIntegrationOwnerIsolation(t *testing.T) {
	pool := integrationPool(t)
	projectRepository := NewProjectRepository(pool)
	repository := NewCreativeBriefRepository(pool)
	ownerA := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	ownerB := uuid.MustParse("33333333-3333-4333-8333-333333333333")

	projectA, err := projectRepository.Create(context.Background(), ownerA, validIntegrationCreateInput("Owner A brief project"))
	if err != nil {
		t.Fatalf("create owner A project: %v", err)
	}
	if _, _, err := repository.Put(context.Background(), ownerA, projectA.ID, validBriefInput("Owner A intent")); err != nil {
		t.Fatalf("create owner A brief: %v", err)
	}

	if _, err := repository.Get(context.Background(), ownerB, projectA.ID); !errors.Is(err, creativebrief.ErrNotFound) {
		t.Fatalf("expected owner B get owner A brief to return not found, got %v", err)
	}

	revision := 1
	input := validBriefInput("Owner B overwrite")
	input.Revision = &revision
	if _, _, err := repository.Put(context.Background(), ownerB, projectA.ID, input); !errors.Is(err, creativebrief.ErrNotFound) {
		t.Fatalf("expected owner B put owner A brief to return not found, got %v", err)
	}
}

func TestCreativeBriefRepositoryIntegrationConcurrentStaleRevision(t *testing.T) {
	pool := integrationPool(t)
	projectRepository := NewProjectRepository(pool)
	repository := NewCreativeBriefRepository(pool)
	ownerID := uuid.MustParse("44444444-4444-4444-8444-444444444444")
	projectItem, err := projectRepository.Create(context.Background(), ownerID, validIntegrationCreateInput("Concurrent brief project"))
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	created, _, err := repository.Put(context.Background(), ownerID, projectItem.ID, validBriefInput("Original intent"))
	if err != nil {
		t.Fatalf("create brief: %v", err)
	}

	type result struct {
		brief creativebrief.CreativeBrief
		err   error
	}
	results := make(chan result, 2)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for _, sourceText := range []string{"Concurrent edit A", "Concurrent edit B"} {
		wg.Add(1)
		go func(sourceText string) {
			defer wg.Done()
			<-start
			revision := created.Revision
			input := validBriefInput(sourceText)
			input.Revision = &revision
			brief, _, err := repository.Put(context.Background(), ownerID, projectItem.ID, input)
			results <- result{brief: brief, err: err}
		}(sourceText)
	}

	close(start)
	wg.Wait()
	close(results)

	successes := 0
	staleErrors := 0
	var winningSource string
	for result := range results {
		switch {
		case result.err == nil:
			successes++
			winningSource = result.brief.SourceText
			if result.brief.Revision != 2 {
				t.Fatalf("expected winning revision 2, got %d", result.brief.Revision)
			}
		case errors.Is(result.err, creativebrief.ErrStaleRevision):
			staleErrors++
		default:
			t.Fatalf("unexpected concurrent update error: %v", result.err)
		}
	}

	if successes != 1 || staleErrors != 1 {
		t.Fatalf("expected 1 success and 1 stale error, got %d successes and %d stale errors", successes, staleErrors)
	}

	finalBrief, err := repository.Get(context.Background(), ownerID, projectItem.ID)
	if err != nil {
		t.Fatalf("get final brief: %v", err)
	}
	if finalBrief.Revision != 2 || finalBrief.SourceText != winningSource {
		t.Fatalf("unexpected final brief: %#v, winning source %q", finalBrief, winningSource)
	}
}

func validBriefInput(sourceText string) creativebrief.PutInput {
	return creativebrief.PutInput{
		SourceText:          sourceText,
		TargetAudience:      "Creators",
		Objective:           "Explain the product",
		DesiredStyle:        "Clean documentary",
		Tone:                "Confident",
		DistributionTargets: []creativebrief.DistributionTarget{creativebrief.DistributionTargetYouTube},
		CallToAction:        "Start trial",
		MustInclude:         []string{"Product demo"},
		MustAvoid:           []string{"Unsupported claims"},
	}
}

var _ project.Repository = (*ProjectRepository)(nil)
