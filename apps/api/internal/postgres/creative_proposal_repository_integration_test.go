package postgres

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/creativeproposal"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
)

func validProposalContent(title string) creativeproposal.Content {
	duration := 60
	return creativeproposal.Content{
		TitleOptions:             []string{title, title + " Alt"},
		HookOptions:              []string{"Hook 1", "Hook 2"},
		AudienceSummary:          "Audience for " + title,
		ObjectiveSummary:         "Objective for " + title,
		NarrativeAngle:           "Narrative for " + title,
		EstimatedDurationSeconds: &duration,
		FormatRationale:          "Rationale for " + title,
		Structure: []creativeproposal.StructureItem{
			{Key: "intro", Title: "Introduction", Purpose: "Hook attention"},
			{Key: "body", Title: "Main Body", Purpose: "Demonstrate value"},
		},
		VisualDirection:  "Visual direction for " + title,
		VoiceDirection:   "Voice direction for " + title,
		MusicDirection:   "Music direction for " + title,
		CaptionDirection: "Caption direction for " + title,
		CallToAction:     "CTA for " + title,
		ResearchGaps:     []string{"Gap 1"},
		Warnings:         []string{"Warning 1"},
	}
}

func TestCreativeProposalRepositoryIntegrationCreateDraftsAndSupersede(t *testing.T) {
	pool := integrationPool(t)
	projectRepository := NewProjectRepository(pool)
	repository := NewCreativeProposalRepository(pool)
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")

	projectItem, err := projectRepository.Create(context.Background(), ownerID, validIntegrationCreateInput("Proposal Project 1"))
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	// 1. Create first draft -> version 1, revision 1, status draft
	v1, err := repository.CreateDraft(context.Background(), ownerID, projectItem.ID, creativeproposal.CreateDraftInput{
		SourceBriefRevision: 1,
		Content:             validProposalContent("Version 1"),
	})
	if err != nil {
		t.Fatalf("create first draft: %v", err)
	}
	if v1.Version != 1 || v1.Revision != 1 || v1.Status != creativeproposal.StatusDraft {
		t.Fatalf("unexpected first draft: %#v", v1)
	}

	// 2. Create second draft -> version 2, revision 1, status draft; v1 becomes superseded
	v2, err := repository.CreateDraft(context.Background(), ownerID, projectItem.ID, creativeproposal.CreateDraftInput{
		SourceBriefRevision: 1,
		Content:             validProposalContent("Version 2"),
	})
	if err != nil {
		t.Fatalf("create second draft: %v", err)
	}
	if v2.Version != 2 || v2.Revision != 1 || v2.Status != creativeproposal.StatusDraft {
		t.Fatalf("unexpected second draft: %#v", v2)
	}

	// Verify v1 is superseded
	fetchedV1, err := repository.Get(context.Background(), ownerID, projectItem.ID, 1)
	if err != nil {
		t.Fatalf("get v1: %v", err)
	}
	if fetchedV1.Status != creativeproposal.StatusSuperseded {
		t.Fatalf("expected v1 to be superseded, got %s", fetchedV1.Status)
	}

	// Listing should return newest first: v2, then v1
	list, err := repository.List(context.Background(), ownerID, projectItem.ID)
	if err != nil {
		t.Fatalf("list proposals: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 proposals, got %d", len(list))
	}
	if list[0].Version != 2 || list[1].Version != 1 {
		t.Fatalf("expected versions [2, 1], got [%d, %d]", list[0].Version, list[1].Version)
	}
}

func TestCreativeProposalRepositoryIntegrationApprovedRemainsImmutable(t *testing.T) {
	pool := integrationPool(t)
	projectRepository := NewProjectRepository(pool)
	repository := NewCreativeProposalRepository(pool)
	ownerID := uuid.MustParse("22222222-2222-4222-8222-222222222222")

	projectItem, err := projectRepository.Create(context.Background(), ownerID, validIntegrationCreateInput("Approved Project"))
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	v1, err := repository.CreateDraft(context.Background(), ownerID, projectItem.ID, creativeproposal.CreateDraftInput{
		SourceBriefRevision: 1,
		Content:             validProposalContent("Version 1"),
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}

	// Approve v1
	approvedV1, err := repository.Approve(context.Background(), ownerID, projectItem.ID, v1.Version, v1.Revision)
	if err != nil {
		t.Fatalf("approve v1: %v", err)
	}
	if approvedV1.Status != creativeproposal.StatusApproved || approvedV1.ApprovedAt == nil {
		t.Fatalf("unexpected approved v1: %#v", approvedV1)
	}

	// Create v2 -> v1 must NOT be superseded (must remain approved!)
	v2, err := repository.CreateDraft(context.Background(), ownerID, projectItem.ID, creativeproposal.CreateDraftInput{
		SourceBriefRevision: 2,
		Content:             validProposalContent("Version 2"),
	})
	if err != nil {
		t.Fatalf("create v2: %v", err)
	}
	if v2.Version != 2 || v2.Status != creativeproposal.StatusDraft {
		t.Fatalf("unexpected v2: %#v", v2)
	}

	fetchedV1, err := repository.Get(context.Background(), ownerID, projectItem.ID, 1)
	if err != nil {
		t.Fatalf("get v1: %v", err)
	}
	if fetchedV1.Status != creativeproposal.StatusApproved {
		t.Fatalf("expected v1 to remain approved, got %s", fetchedV1.Status)
	}

	// Updating approved v1 returns ErrProposalImmutable
	rev := fetchedV1.Revision
	_, err = repository.UpdateDraft(context.Background(), ownerID, projectItem.ID, 1, creativeproposal.PutInput{
		Revision: &rev,
		Content:  validProposalContent("Mutate approved"),
	})
	if !errors.Is(err, creativeproposal.ErrProposalImmutable) {
		t.Fatalf("expected ErrProposalImmutable on update, got %v", err)
	}

	// Re-approving approved v1 returns ErrProposalImmutable
	_, err = repository.Approve(context.Background(), ownerID, projectItem.ID, 1, rev)
	if !errors.Is(err, creativeproposal.ErrProposalImmutable) {
		t.Fatalf("expected ErrProposalImmutable on approve, got %v", err)
	}
}

func TestCreativeProposalRepositoryIntegrationUpdateDraftAndStaleRevision(t *testing.T) {
	pool := integrationPool(t)
	projectRepository := NewProjectRepository(pool)
	repository := NewCreativeProposalRepository(pool)
	ownerID := uuid.MustParse("33333333-3333-4333-8333-333333333333")

	projectItem, err := projectRepository.Create(context.Background(), ownerID, validIntegrationCreateInput("Update Project"))
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	draft, err := repository.CreateDraft(context.Background(), ownerID, projectItem.ID, creativeproposal.CreateDraftInput{
		SourceBriefRevision: 1,
		Content:             validProposalContent("Original Content"),
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}

	// Successful update
	currentRev := draft.Revision
	updated, err := repository.UpdateDraft(context.Background(), ownerID, projectItem.ID, draft.Version, creativeproposal.PutInput{
		Revision: &currentRev,
		Content:  validProposalContent("Updated Content"),
	})
	if err != nil {
		t.Fatalf("update draft: %v", err)
	}
	if updated.Revision != 2 || updated.TitleOptions[0] != "Updated Content" {
		t.Fatalf("unexpected updated draft: %#v", updated)
	}

	// Stale update with old revision (1) -> ErrStaleRevision
	_, err = repository.UpdateDraft(context.Background(), ownerID, projectItem.ID, draft.Version, creativeproposal.PutInput{
		Revision: &currentRev,
		Content:  validProposalContent("Stale Content"),
	})
	if !errors.Is(err, creativeproposal.ErrStaleRevision) {
		t.Fatalf("expected ErrStaleRevision, got %v", err)
	}

	// Stale approve with old revision (1) -> ErrStaleRevision
	_, err = repository.Approve(context.Background(), ownerID, projectItem.ID, draft.Version, currentRev)
	if !errors.Is(err, creativeproposal.ErrStaleRevision) {
		t.Fatalf("expected ErrStaleRevision on approve, got %v", err)
	}
}

func TestCreativeProposalRepositoryIntegrationOwnerIsolation(t *testing.T) {
	pool := integrationPool(t)
	projectRepository := NewProjectRepository(pool)
	repository := NewCreativeProposalRepository(pool)
	ownerA := uuid.MustParse("44444444-4444-4444-8444-444444444444")
	ownerB := uuid.MustParse("55555555-5555-4555-8555-555555555555")

	projectA, err := projectRepository.Create(context.Background(), ownerA, validIntegrationCreateInput("Owner A Project"))
	if err != nil {
		t.Fatalf("create owner A project: %v", err)
	}

	draftA, err := repository.CreateDraft(context.Background(), ownerA, projectA.ID, creativeproposal.CreateDraftInput{
		SourceBriefRevision: 1,
		Content:             validProposalContent("Owner A Draft"),
	})
	if err != nil {
		t.Fatalf("create draft A: %v", err)
	}

	// Owner B cannot list Owner A proposals
	_, err = repository.List(context.Background(), ownerB, projectA.ID)
	if !errors.Is(err, creativeproposal.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for owner B list, got %v", err)
	}

	// Owner B cannot get Owner A proposal
	_, err = repository.Get(context.Background(), ownerB, projectA.ID, draftA.Version)
	if !errors.Is(err, creativeproposal.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for owner B get, got %v", err)
	}

	// Owner B cannot update Owner A proposal
	rev := draftA.Revision
	_, err = repository.UpdateDraft(context.Background(), ownerB, projectA.ID, draftA.Version, creativeproposal.PutInput{
		Revision: &rev,
		Content:  validProposalContent("Owner B Attack"),
	})
	if !errors.Is(err, creativeproposal.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for owner B update, got %v", err)
	}

	// Owner B cannot approve Owner A proposal
	_, err = repository.Approve(context.Background(), ownerB, projectA.ID, draftA.Version, rev)
	if !errors.Is(err, creativeproposal.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for owner B approve, got %v", err)
	}
}

func TestCreativeProposalRepositoryIntegrationConcurrentDraftCreation(t *testing.T) {
	pool := integrationPool(t)
	projectRepository := NewProjectRepository(pool)
	repository := NewCreativeProposalRepository(pool)
	ownerID := uuid.MustParse("66666666-6666-4666-8666-666666666666")

	projectItem, err := projectRepository.Create(context.Background(), ownerID, validIntegrationCreateInput("Concurrent Creation Project"))
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	const n = 5
	var wg sync.WaitGroup
	errCh := make(chan error, n)
	proposals := make(chan creativeproposal.CreativeProposal, n)

	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			p, err := repository.CreateDraft(context.Background(), ownerID, projectItem.ID, creativeproposal.CreateDraftInput{
				SourceBriefRevision: 1,
				Content:             validProposalContent("Concurrent Draft"),
			})
			if err != nil {
				errCh <- err
				return
			}
			proposals <- p
		}(i)
	}

	close(start)
	wg.Wait()
	close(errCh)
	close(proposals)

	for err := range errCh {
		t.Fatalf("concurrent draft creation failed: %v", err)
	}

	createdVersions := make(map[int]struct{})
	for p := range proposals {
		if _, exists := createdVersions[p.Version]; exists {
			t.Fatalf("duplicate version allocated: %d", p.Version)
		}
		createdVersions[p.Version] = struct{}{}
	}

	if len(createdVersions) != n {
		t.Fatalf("expected %d distinct versions, got %d", n, len(createdVersions))
	}

	// Verify all versions exist and exactly 1 is draft, n-1 are superseded
	list, err := repository.List(context.Background(), ownerID, projectItem.ID)
	if err != nil {
		t.Fatalf("list proposals: %v", err)
	}
	if len(list) != n {
		t.Fatalf("expected %d proposals in list, got %d", n, len(list))
	}

	draftCount := 0
	supersededCount := 0
	for _, item := range list {
		if item.Status == creativeproposal.StatusDraft {
			draftCount++
			if item.Version != n {
				t.Fatalf("expected highest version %d to be draft, got version %d", n, item.Version)
			}
		} else if item.Status == creativeproposal.StatusSuperseded {
			supersededCount++
		}
	}

	if draftCount != 1 || supersededCount != n-1 {
		t.Fatalf("expected 1 draft and %d superseded, got %d drafts and %d superseded", n-1, draftCount, supersededCount)
	}
}

func TestCreativeProposalRepositoryIntegrationConcurrentUpdates(t *testing.T) {
	pool := integrationPool(t)
	projectRepository := NewProjectRepository(pool)
	repository := NewCreativeProposalRepository(pool)
	ownerID := uuid.MustParse("77777777-7777-4777-8777-777777777777")

	projectItem, err := projectRepository.Create(context.Background(), ownerID, validIntegrationCreateInput("Concurrent Update Project"))
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	draft, err := repository.CreateDraft(context.Background(), ownerID, projectItem.ID, creativeproposal.CreateDraftInput{
		SourceBriefRevision: 1,
		Content:             validProposalContent("Original Draft"),
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}

	type result struct {
		p   creativeproposal.CreativeProposal
		err error
	}
	results := make(chan result, 2)
	start := make(chan struct{})
	var wg sync.WaitGroup

	for _, title := range []string{"Concurrent A", "Concurrent B"} {
		wg.Add(1)
		go func(title string) {
			defer wg.Done()
			<-start
			rev := draft.Revision
			p, err := repository.UpdateDraft(context.Background(), ownerID, projectItem.ID, draft.Version, creativeproposal.PutInput{
				Revision: &rev,
				Content:  validProposalContent(title),
			})
			results <- result{p: p, err: err}
		}(title)
	}

	close(start)
	wg.Wait()
	close(results)

	successes := 0
	staleErrors := 0
	for res := range results {
		switch {
		case res.err == nil:
			successes++
			if res.p.Revision != 2 {
				t.Fatalf("expected revision 2, got %d", res.p.Revision)
			}
		case errors.Is(res.err, creativeproposal.ErrStaleRevision):
			staleErrors++
		default:
			t.Fatalf("unexpected concurrent update error: %v", res.err)
		}
	}

	if successes != 1 || staleErrors != 1 {
		t.Fatalf("expected 1 success and 1 stale error, got %d successes and %d stale errors", successes, staleErrors)
	}
}

func TestCreativeProposalRepositoryIntegration_IdempotentCreateDraftFromJob(t *testing.T) {
	pool := integrationPool(t)
	projectRepository := NewProjectRepository(pool)
	jobsRepository := NewJobRepository(pool)
	proposalRepository := NewCreativeProposalRepository(pool)
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")

	projectItem, err := projectRepository.Create(context.Background(), ownerID, validIntegrationCreateInput("Job Idempotency Project"))
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	job1, err := jobsRepository.Enqueue(context.Background(), jobs.EnqueueInput{
		ID:          uuid.New(),
		OwnerID:     ownerID,
		ProjectID:   &projectItem.ID,
		Kind:        "creative_proposal_generation_v1",
		MaxAttempts: 3,
		Payload:     []byte(`{"schema_version":"ai_proposal_generation_job_v1"}`),
	})
	if err != nil {
		t.Fatalf("enqueue job 1: %v", err)
	}

	// First call with job1
	draft1, err := proposalRepository.CreateDraft(context.Background(), ownerID, projectItem.ID, creativeproposal.CreateDraftInput{
		SourceBriefRevision:   1,
		SourceGenerationJobID: &job1.ID,
		Content:               validProposalContent("Generated Draft 1"),
	})
	if err != nil {
		t.Fatalf("create draft with job 1: %v", err)
	}
	if draft1.Version != 1 || draft1.Status != creativeproposal.StatusDraft {
		t.Fatalf("unexpected draft 1: %+v", draft1)
	}

	// Retry simulation with the exact same job1 ID
	draft1Retry, err := proposalRepository.CreateDraft(context.Background(), ownerID, projectItem.ID, creativeproposal.CreateDraftInput{
		SourceBriefRevision:   1,
		SourceGenerationJobID: &job1.ID,
		Content:               validProposalContent("Generated Draft 1 Retry Attempt"),
	})
	if err != nil {
		t.Fatalf("retry create draft with job 1: %v", err)
	}
	if draft1Retry.Version != draft1.Version {
		t.Fatalf("expected version %d on retry, got %d", draft1.Version, draft1Retry.Version)
	}
	if draft1Retry.TitleOptions[0] != "Generated Draft 1" {
		t.Fatalf("expected original draft content, got %v", draft1Retry.TitleOptions)
	}

	// Check proposals count is still 1
	list, err := proposalRepository.List(context.Background(), ownerID, projectItem.ID)
	if err != nil {
		t.Fatalf("list proposals: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 proposal, got %d", len(list))
	}

	// Distinct job2 creates next version
	job2, err := jobsRepository.Enqueue(context.Background(), jobs.EnqueueInput{
		ID:          uuid.New(),
		OwnerID:     ownerID,
		ProjectID:   &projectItem.ID,
		Kind:        "creative_proposal_generation_v1",
		MaxAttempts: 3,
		Payload:     []byte(`{"schema_version":"ai_proposal_generation_job_v1"}`),
	})
	if err != nil {
		t.Fatalf("enqueue job 2: %v", err)
	}

	draft2, err := proposalRepository.CreateDraft(context.Background(), ownerID, projectItem.ID, creativeproposal.CreateDraftInput{
		SourceBriefRevision:   1,
		SourceGenerationJobID: &job2.ID,
		Content:               validProposalContent("Generated Draft 2"),
	})
	if err != nil {
		t.Fatalf("create draft with job 2: %v", err)
	}
	if draft2.Version != 2 || draft2.Status != creativeproposal.StatusDraft {
		t.Fatalf("unexpected draft 2: %+v", draft2)
	}

	// Check draft 1 is now superseded
	oldDraft1, err := proposalRepository.Get(context.Background(), ownerID, projectItem.ID, 1)
	if err != nil {
		t.Fatalf("get draft 1: %v", err)
	}
	if oldDraft1.Status != creativeproposal.StatusSuperseded {
		t.Fatalf("expected draft 1 to be superseded, got %s", oldDraft1.Status)
	}
}
