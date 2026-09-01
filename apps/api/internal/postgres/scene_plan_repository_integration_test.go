package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/creativeproposal"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/jobs"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneplan"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/script"
)

func TestScenePlanRepositoryIntegration(t *testing.T) {
	pool := integrationPool(t)
	projectRepo := NewProjectRepository(pool)
	proposalRepo := NewCreativeProposalRepository(pool)
	scriptRepo := NewScriptRepository(pool)
	repo := NewScenePlanRepository(pool)

	ownerA := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	ownerB := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	projectA, err := projectRepo.Create(context.Background(), ownerA, validIntegrationCreateInput("Scene Plan Project"))
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	projectB, err := projectRepo.Create(context.Background(), ownerB, validIntegrationCreateInput("Other Owner Project"))
	if err != nil {
		t.Fatalf("create other owner project: %v", err)
	}

	proposal, err := proposalRepo.CreateDraft(context.Background(), ownerA, projectA.ID, creativeproposal.CreateDraftInput{
		SourceBriefRevision: 1,
		Content:             validProposalContent("Scene Plan Proposal"),
	})
	if err != nil {
		t.Fatalf("create proposal: %v", err)
	}
	proposal, err = proposalRepo.Approve(context.Background(), ownerA, projectA.ID, proposal.Version, proposal.Revision)
	if err != nil {
		t.Fatalf("approve proposal: %v", err)
	}

	scriptV1, err := scriptRepo.CreateDraft(context.Background(), ownerA, projectA.ID, script.CreateDraftInput{
		SourceProposalVersion: proposal.Version,
		Content:               validScriptContent("Scene Plan Source"),
	})
	if err != nil {
		t.Fatalf("create source script: %v", err)
	}
	scriptV1, err = scriptRepo.Approve(context.Background(), ownerA, projectA.ID, scriptV1.Version, scriptV1.Revision)
	if err != nil {
		t.Fatalf("approve source script: %v", err)
	}

	first, err := repo.CreateDraft(context.Background(), ownerA, projectA.ID, sceneplan.CreateDraftInput{
		SourceScriptVersion: scriptV1.Version,
		Content:             validScenePlanContent("Scene Plan Source"),
	})
	if err != nil {
		t.Fatalf("create first scene plan: %v", err)
	}
	if first.Version != 1 || first.Revision != 1 || first.Status != sceneplan.StatusDraft {
		t.Fatalf("unexpected first scene plan: %#v", first)
	}
	if first.SourceScriptVersion != scriptV1.Version || first.SourceProposalVersion != proposal.Version || first.ContentLocale != string(projectA.Locale) {
		t.Fatalf("unexpected source metadata: %#v", first)
	}

	updated, err := repo.UpdateDraft(context.Background(), ownerA, projectA.ID, first.Version, sceneplan.PutInput{
		Revision: &first.Revision,
		Content:  scenePlanContent("Scene Plan Source", "Scene Plan Source Updated"),
	})
	if err != nil {
		t.Fatalf("update scene plan: %v", err)
	}
	if updated.Revision != 2 || updated.Scenes[0].VisualInstruction != "Scene Plan Source Updated intro visual" {
		t.Fatalf("unexpected updated scene plan: %#v", updated)
	}

	stale := first.Revision
	if _, err := repo.UpdateDraft(context.Background(), ownerA, projectA.ID, first.Version, sceneplan.PutInput{
		Revision: &stale,
		Content:  scenePlanContent("Scene Plan Source", "stale"),
	}); !errors.Is(err, sceneplan.ErrStaleRevision) {
		t.Fatalf("stale update error = %v", err)
	}
	if _, err := repo.Approve(context.Background(), ownerA, projectA.ID, first.Version, stale); !errors.Is(err, sceneplan.ErrStaleRevision) {
		t.Fatalf("stale approval error = %v", err)
	}

	approved, err := repo.Approve(context.Background(), ownerA, projectA.ID, first.Version, updated.Revision)
	if err != nil {
		t.Fatalf("approve scene plan: %v", err)
	}
	if approved.Status != sceneplan.StatusApproved || approved.ApprovedAt == nil {
		t.Fatalf("expected immutable approved plan: %#v", approved)
	}
	if _, err := repo.UpdateDraft(context.Background(), ownerA, projectA.ID, first.Version, sceneplan.PutInput{
		Revision: &approved.Revision,
		Content:  validScenePlanContent("mutate approved"),
	}); !errors.Is(err, sceneplan.ErrScenePlanImmutable) {
		t.Fatalf("approved update error = %v", err)
	}

	// Project locale changes do not rewrite metadata copied into an existing plan.
	en := project.LocaleEN
	if _, err := projectRepo.Update(context.Background(), ownerA, projectA.ID, project.UpdateInput{Locale: &en}); err != nil {
		t.Fatalf("update project locale: %v", err)
	}
	fetched, err := repo.GetByVersion(context.Background(), ownerA, projectA.ID, first.Version)
	if err != nil {
		t.Fatalf("get approved scene plan: %v", err)
	}
	if fetched.ContentLocale != string(project.LocaleVI) {
		t.Fatalf("scene plan locale changed with project: %s", fetched.ContentLocale)
	}

	second, err := repo.CreateDraft(context.Background(), ownerA, projectA.ID, sceneplan.CreateDraftInput{
		SourceScriptVersion: scriptV1.Version,
		Content:             scenePlanContent("Scene Plan Source", "second"),
	})
	if err != nil {
		t.Fatalf("create second scene plan: %v", err)
	}
	if second.Version != 2 || second.ContentLocale != string(project.LocaleEN) {
		t.Fatalf("unexpected second scene plan: %#v", second)
	}
	old, err := repo.GetByVersion(context.Background(), ownerA, projectA.ID, first.Version)
	if err != nil {
		t.Fatalf("get superseded candidate: %v", err)
	}
	if old.Status != sceneplan.StatusApproved {
		t.Fatalf("approved history was superseded: %s", old.Status)
	}

	if _, err := repo.UpdateDraft(context.Background(), ownerB, projectA.ID, second.Version, sceneplan.PutInput{
		Revision: &second.Revision,
		Content:  scenePlanContent("Scene Plan Source", "foreign owner update"),
	}); !errors.Is(err, sceneplan.ErrNotFound) {
		t.Fatalf("owner-isolated update error = %v", err)
	}
	if _, err := repo.Approve(context.Background(), ownerB, projectA.ID, second.Version, second.Revision); !errors.Is(err, sceneplan.ErrNotFound) {
		t.Fatalf("owner-isolated approve error = %v", err)
	}

	// An unapproved and a missing source are both rejected without creating a plan.
	draftScript, err := scriptRepo.CreateDraft(context.Background(), ownerA, projectA.ID, script.CreateDraftInput{
		SourceProposalVersion: proposal.Version,
		Content:               validScriptContent("unapproved source"),
	})
	if err != nil {
		t.Fatalf("create unapproved source script: %v", err)
	}
	if _, err := repo.CreateDraft(context.Background(), ownerA, projectA.ID, sceneplan.CreateDraftInput{
		SourceScriptVersion: draftScript.Version,
		Content:             validScenePlanContent("bad source"),
	}); !errors.Is(err, sceneplan.ErrScriptNotApproved) {
		t.Fatalf("unapproved source error = %v", err)
	}
	if _, err := repo.CreateDraft(context.Background(), ownerA, projectA.ID, sceneplan.CreateDraftInput{
		SourceScriptVersion: 999,
		Content:             validScenePlanContent("missing source"),
	}); !errors.Is(err, sceneplan.ErrScriptSourceInvalid) {
		t.Fatalf("missing source error = %v", err)
	}

	invalid := validScenePlanContent("coverage")
	invalid.Scenes[0].Narration = "changed narration"
	if _, err := repo.CreateDraft(context.Background(), ownerA, projectA.ID, sceneplan.CreateDraftInput{
		SourceScriptVersion: scriptV1.Version,
		Content:             invalid,
	}); err == nil {
		t.Fatal("expected narration coverage rejection")
	}

	if _, err := repo.GetByVersion(context.Background(), ownerB, projectA.ID, first.Version); !errors.Is(err, sceneplan.ErrNotFound) {
		t.Fatalf("owner-isolated get error = %v", err)
	}
	if _, err := repo.ListVersions(context.Background(), ownerB, projectA.ID); !errors.Is(err, sceneplan.ErrNotFound) {
		t.Fatalf("owner-isolated list error = %v", err)
	}
	if _, err := repo.CreateDraft(context.Background(), ownerB, projectA.ID, sceneplan.CreateDraftInput{
		SourceScriptVersion: scriptV1.Version,
		Content:             validScenePlanContent("foreign owner"),
	}); !errors.Is(err, sceneplan.ErrNotFound) {
		t.Fatalf("owner-isolated create error = %v", err)
	}

	proposalB, err := proposalRepo.CreateDraft(context.Background(), ownerB, projectB.ID, creativeproposal.CreateDraftInput{
		SourceBriefRevision: 1,
		Content:             validProposalContent("Foreign Source Proposal"),
	})
	if err != nil {
		t.Fatalf("create foreign source proposal: %v", err)
	}
	proposalB, err = proposalRepo.Approve(context.Background(), ownerB, projectB.ID, proposalB.Version, proposalB.Revision)
	if err != nil {
		t.Fatalf("approve foreign source proposal: %v", err)
	}
	scriptB, err := scriptRepo.CreateDraft(context.Background(), ownerB, projectB.ID, script.CreateDraftInput{
		SourceProposalVersion: proposalB.Version,
		Content:               validScriptContent("Foreign Source Script"),
	})
	if err != nil {
		t.Fatalf("create foreign source script: %v", err)
	}
	scriptB, err = scriptRepo.Approve(context.Background(), ownerB, projectB.ID, scriptB.Version, scriptB.Revision)
	if err != nil {
		t.Fatalf("approve foreign source script: %v", err)
	}
	projectC, err := projectRepo.Create(context.Background(), ownerA, validIntegrationCreateInput("Foreign Source Target"))
	if err != nil {
		t.Fatalf("create foreign source target project: %v", err)
	}
	if _, err := repo.CreateDraft(context.Background(), ownerA, projectC.ID, sceneplan.CreateDraftInput{
		SourceScriptVersion: scriptB.Version,
		Content:             validScenePlanContent("Foreign Source Script"),
	}); !errors.Is(err, sceneplan.ErrScriptSourceInvalid) {
		t.Fatalf("foreign source error = %v", err)
	}

	concurrentDrafts := 6
	created := make(chan sceneplan.Plan, concurrentDrafts)
	errorsCh := make(chan error, concurrentDrafts)
	var createWG sync.WaitGroup
	for i := 0; i < concurrentDrafts; i++ {
		createWG.Add(1)
		go func() {
			defer createWG.Done()
			plan, err := repo.CreateDraft(context.Background(), ownerA, projectA.ID, sceneplan.CreateDraftInput{
				SourceScriptVersion: scriptV1.Version,
				Content:             scenePlanContent("Scene Plan Source", "concurrent"),
			})
			if err != nil {
				errorsCh <- err
				return
			}
			created <- plan
		}()
	}
	createWG.Wait()
	close(created)
	close(errorsCh)
	for err := range errorsCh {
		t.Fatalf("concurrent create: %v", err)
	}
	versions := map[int]bool{}
	for plan := range created {
		if versions[plan.Version] {
			t.Fatalf("duplicate concurrent version %d", plan.Version)
		}
		versions[plan.Version] = true
	}
	if len(versions) != concurrentDrafts {
		t.Fatalf("created %d concurrent plans, want %d", len(versions), concurrentDrafts)
	}
	plans, err := repo.ListVersions(context.Background(), ownerA, projectA.ID)
	if err != nil {
		t.Fatalf("list scene plans: %v", err)
	}
	activeDrafts := 0
	for _, plan := range plans {
		if plan.Status == sceneplan.StatusDraft {
			activeDrafts++
		}
	}
	if activeDrafts != 1 {
		t.Fatalf("active drafts = %d, want 1", activeDrafts)
	}

	current := plans[0]
	concurrentUpdates := 6
	updateErrors := make(chan error, concurrentUpdates)
	var updateWG sync.WaitGroup
	for i := 0; i < concurrentUpdates; i++ {
		updateWG.Add(1)
		go func() {
			defer updateWG.Done()
			_, err := repo.UpdateDraft(context.Background(), ownerA, projectA.ID, current.Version, sceneplan.PutInput{
				Revision: &current.Revision,
				Content:  scenePlanContent("Scene Plan Source", "concurrent update"),
			})
			updateErrors <- err
		}()
	}
	updateWG.Wait()
	close(updateErrors)
	successes := 0
	staleErrors := 0
	for err := range updateErrors {
		if err == nil {
			successes++
		} else if errors.Is(err, sceneplan.ErrStaleRevision) {
			staleErrors++
		} else {
			t.Fatalf("concurrent update error = %v", err)
		}
	}
	if successes != 1 || staleErrors != concurrentUpdates-1 {
		t.Fatalf("concurrent updates: successes=%d stale=%d", successes, staleErrors)
	}
}

func validScenePlanContent(prefix string) sceneplan.Content {
	return scenePlanContent(prefix, prefix)
}

func scenePlanContent(narrationPrefix, visualPrefix string) sceneplan.Content {
	return sceneplan.Content{Scenes: []sceneplan.Scene{
		{Key: "intro", ScriptSectionKey: "intro", Narration: narrationPrefix + " intro body", VisualInstruction: visualPrefix + " intro visual", PlannedSourceType: sceneplan.SourceTypeStock, ExpectedDurationSeconds: 5},
		{Key: "main", ScriptSectionKey: "section-1", Narration: narrationPrefix + " main body", VisualInstruction: visualPrefix + " main visual", PlannedSourceType: sceneplan.SourceTypeCreatorMedia, ExpectedDurationSeconds: 8},
		{Key: "outro", ScriptSectionKey: "outro", Narration: narrationPrefix + " outro body", VisualInstruction: visualPrefix + " outro visual", PlannedSourceType: sceneplan.SourceTypeGeneratedImage, ExpectedDurationSeconds: 6},
	}}
}

func TestScenePlanRepositoryIntegration_IdempotentCreateDraftFromJob(t *testing.T) {
	pool := integrationPool(t)
	projectRepo := NewProjectRepository(pool)
	jobsRepo := NewJobRepository(pool)
	proposalRepo := NewCreativeProposalRepository(pool)
	scriptRepo := NewScriptRepository(pool)
	repo := NewScenePlanRepository(pool)

	ownerID := uuid.MustParse("33333333-3333-4333-8333-333333333333")
	projectItem, err := projectRepo.Create(context.Background(), ownerID, validIntegrationCreateInput("Scene Plan Idempotent Project"))
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	prop, err := proposalRepo.CreateDraft(context.Background(), ownerID, projectItem.ID, creativeproposal.CreateDraftInput{
		SourceBriefRevision: 1,
		Content:             validProposalContent("Approved Proposal"),
	})
	if err != nil {
		t.Fatalf("create proposal: %v", err)
	}
	propApproved, err := proposalRepo.Approve(context.Background(), ownerID, projectItem.ID, prop.Version, prop.Revision)
	if err != nil {
		t.Fatalf("approve proposal: %v", err)
	}

	scr, err := scriptRepo.CreateDraft(context.Background(), ownerID, projectItem.ID, script.CreateDraftInput{
		SourceProposalVersion: propApproved.Version,
		Content:               validScriptContent("Approved Script"),
	})
	if err != nil {
		t.Fatalf("create script: %v", err)
	}
	scrApproved, err := scriptRepo.Approve(context.Background(), ownerID, projectItem.ID, scr.Version, scr.Revision)
	if err != nil {
		t.Fatalf("approve script: %v", err)
	}

	jobID := uuid.New()
	_, err = jobsRepo.Enqueue(context.Background(), jobs.EnqueueInput{
		ID:          jobID,
		OwnerID:     ownerID,
		ProjectID:   &projectItem.ID,
		Kind:        "scene_plan_generation_v1",
		MaxAttempts: 3,
		Payload:     []byte(`{"schema_version":"scene_plan_generation_job_v1"}`),
	})
	if err != nil {
		t.Fatalf("enqueue job: %v", err)
	}

	// First CreateDraft call creates draft v1
	draft1, err := repo.CreateDraft(context.Background(), ownerID, projectItem.ID, sceneplan.CreateDraftInput{
		SourceScriptVersion:   scrApproved.Version,
		SourceGenerationJobID: &jobID,
		ContentLocale:         "vi",
		Content:               validScenePlanContent("Approved Script"),
	})
	if err != nil {
		t.Fatalf("first CreateDraft: %v", err)
	}
	if draft1.Version != 1 {
		t.Fatalf("expected version 1, got %d", draft1.Version)
	}

	// Second CreateDraft call with same job ID returns existing draft1 without creating a new version
	draft2, err := repo.CreateDraft(context.Background(), ownerID, projectItem.ID, sceneplan.CreateDraftInput{
		SourceScriptVersion:   scrApproved.Version,
		SourceGenerationJobID: &jobID,
		ContentLocale:         "vi",
		Content:               validScenePlanContent("Approved Script"),
	})
	if err != nil {
		t.Fatalf("second CreateDraft (replay): %v", err)
	}
	if draft2.Version != draft1.Version {
		t.Fatalf("expected returned draft version %d, got %d", draft1.Version, draft2.Version)
	}
	if draft2.Revision != draft1.Revision {
		t.Fatalf("expected returned draft revision %d, got %d", draft1.Revision, draft2.Revision)
	}

	// List versions must return exactly 1 version
	list, err := repo.ListVersions(context.Background(), ownerID, projectItem.ID)
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected exactly 1 version in list, got %d", len(list))
	}
}

func TestScenePlanRepositoryIntegration_SnapshottedLocalePreservedOnProjectLocaleChange(t *testing.T) {
	pool := integrationPool(t)
	projectRepo := NewProjectRepository(pool)
	jobsRepo := NewJobRepository(pool)
	proposalRepo := NewCreativeProposalRepository(pool)
	scriptRepo := NewScriptRepository(pool)
	repo := NewScenePlanRepository(pool)

	ownerID := uuid.MustParse("44444444-4444-4444-8444-444444444444")
	projectItem, err := projectRepo.Create(context.Background(), ownerID, validIntegrationCreateInput("Scene Plan Locale Preservation Project"))
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	prop, err := proposalRepo.CreateDraft(context.Background(), ownerID, projectItem.ID, creativeproposal.CreateDraftInput{
		SourceBriefRevision: 1,
		Content:             validProposalContent("Approved Proposal"),
	})
	if err != nil {
		t.Fatalf("create proposal: %v", err)
	}
	propApproved, err := proposalRepo.Approve(context.Background(), ownerID, projectItem.ID, prop.Version, prop.Revision)
	if err != nil {
		t.Fatalf("approve proposal: %v", err)
	}

	scr, err := scriptRepo.CreateDraft(context.Background(), ownerID, projectItem.ID, script.CreateDraftInput{
		SourceProposalVersion: propApproved.Version,
		Content:               validScriptContent("Approved Script"),
	})
	if err != nil {
		t.Fatalf("create script: %v", err)
	}
	scrApproved, err := scriptRepo.Approve(context.Background(), ownerID, projectItem.ID, scr.Version, scr.Revision)
	if err != nil {
		t.Fatalf("approve script: %v", err)
	}

	jobID := uuid.New()
	_, err = jobsRepo.Enqueue(context.Background(), jobs.EnqueueInput{
		ID:          jobID,
		OwnerID:     ownerID,
		ProjectID:   &projectItem.ID,
		Kind:        "scene_plan_generation_v1",
		MaxAttempts: 3,
		Payload:     []byte(`{"schema_version":"scene_plan_generation_job_v1"}`),
	})
	if err != nil {
		t.Fatalf("enqueue job: %v", err)
	}

	// Change project's locale in DB to 'en' after job enqueue
	if _, err := pool.Exec(context.Background(), `UPDATE projects SET locale = 'en' WHERE id = $1`, projectItem.ID.String()); err != nil {
		t.Fatalf("update project locale: %v", err)
	}

	// Worker creates scene plan draft passing the snapshotted locale 'vi'
	created, err := repo.CreateDraft(context.Background(), ownerID, projectItem.ID, sceneplan.CreateDraftInput{
		SourceScriptVersion:   scrApproved.Version,
		SourceGenerationJobID: &jobID,
		ContentLocale:         "vi",
		Content:               validScenePlanContent("Approved Script"),
	})
	if err != nil {
		t.Fatalf("create draft with snapshotted locale: %v", err)
	}

	if created.ContentLocale != "vi" {
		t.Fatalf("expected created content locale 'vi', got %q", created.ContentLocale)
	}

	fetched, err := repo.GetByVersion(context.Background(), ownerID, projectItem.ID, created.Version)
	if err != nil {
		t.Fatalf("get scene plan version: %v", err)
	}
	if fetched.ContentLocale != "vi" {
		t.Fatalf("expected fetched content locale 'vi', got %q", fetched.ContentLocale)
	}
}

func TestScenePlanRepositoryIntegration_ConcurrentCreateDraftFromJob(t *testing.T) {
	pool := integrationPool(t)
	projectRepo := NewProjectRepository(pool)
	jobsRepo := NewJobRepository(pool)
	proposalRepo := NewCreativeProposalRepository(pool)
	scriptRepo := NewScriptRepository(pool)
	repo := NewScenePlanRepository(pool)

	ownerID := uuid.MustParse("55555555-5555-4555-8555-555555555555")
	projectItem, err := projectRepo.Create(context.Background(), ownerID, validIntegrationCreateInput("Scene Plan Concurrency Project"))
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	prop, err := proposalRepo.CreateDraft(context.Background(), ownerID, projectItem.ID, creativeproposal.CreateDraftInput{
		SourceBriefRevision: 1,
		Content:             validProposalContent("Approved Proposal"),
	})
	if err != nil {
		t.Fatalf("create proposal: %v", err)
	}
	propApproved, err := proposalRepo.Approve(context.Background(), ownerID, projectItem.ID, prop.Version, prop.Revision)
	if err != nil {
		t.Fatalf("approve proposal: %v", err)
	}

	scr, err := scriptRepo.CreateDraft(context.Background(), ownerID, projectItem.ID, script.CreateDraftInput{
		SourceProposalVersion: propApproved.Version,
		Content:               validScriptContent("Approved Script"),
	})
	if err != nil {
		t.Fatalf("create script: %v", err)
	}
	scrApproved, err := scriptRepo.Approve(context.Background(), ownerID, projectItem.ID, scr.Version, scr.Revision)
	if err != nil {
		t.Fatalf("approve script: %v", err)
	}

	jobID := uuid.New()
	_, err = jobsRepo.Enqueue(context.Background(), jobs.EnqueueInput{
		ID:          jobID,
		OwnerID:     ownerID,
		ProjectID:   &projectItem.ID,
		Kind:        "scene_plan_generation_v1",
		MaxAttempts: 3,
		Payload:     []byte(`{"schema_version":"scene_plan_generation_job_v1"}`),
	})
	if err != nil {
		t.Fatalf("enqueue job: %v", err)
	}

	const concurrency = 10
	type result struct {
		plan sceneplan.Plan
		err  error
	}

	results := make(chan result, concurrency)
	startBarrier := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-startBarrier
			res, err := repo.CreateDraft(context.Background(), ownerID, projectItem.ID, sceneplan.CreateDraftInput{
				SourceScriptVersion:   scrApproved.Version,
				SourceGenerationJobID: &jobID,
				ContentLocale:         "vi",
				Content:               validScenePlanContent("Approved Script"),
			})
			results <- result{plan: res, err: err}
		}(i)
	}

	close(startBarrier)
	wg.Wait()
	close(results)

	var expectedVersion int
	for res := range results {
		if res.err != nil {
			t.Fatalf("concurrent CreateDraft failed: %v", res.err)
		}
		if expectedVersion == 0 {
			expectedVersion = res.plan.Version
		} else if res.plan.Version != expectedVersion {
			t.Fatalf("mismatched version returned: expected %d, got %d", expectedVersion, res.plan.Version)
		}
		if res.plan.Status != sceneplan.StatusDraft {
			t.Fatalf("expected draft status, got %s", res.plan.Status)
		}
	}

	// Verify exactly 1 scene plan version in DB
	list, err := repo.ListVersions(context.Background(), ownerID, projectItem.ID)
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected exactly 1 scene plan version, got %d", len(list))
	}
	if list[0].Version != expectedVersion {
		t.Fatalf("expected version %d, got %d", expectedVersion, list[0].Version)
	}
	if list[0].Status != sceneplan.StatusDraft {
		t.Fatalf("expected draft status, got %s", list[0].Status)
	}
}

func TestScenePlanRepositoryIntegration_JSONDoesNotExposeJobID(t *testing.T) {
	jobID := uuid.New()
	p := sceneplan.Plan{
		ProjectID:             uuid.New(),
		Version:               1,
		Revision:              1,
		Status:                sceneplan.StatusDraft,
		SourceScriptVersion:   1,
		SourceProposalVersion: 1,
		ContentLocale:         "vi",
		Scenes: []sceneplan.Scene{
			{Key: "intro", ScriptSectionKey: "intro", Narration: "N", VisualInstruction: "V", PlannedSourceType: sceneplan.SourceTypeStock, ExpectedDurationSeconds: 5},
		},
		SourceGenerationJobID: &jobID,
	}

	b, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal scene plan: %v", err)
	}

	jsonStr := string(b)
	if strings.Contains(jsonStr, jobID.String()) || strings.Contains(jsonStr, "source_generation_job_id") {
		t.Fatalf("public JSON exposes source_generation_job_id: %s", jsonStr)
	}
}
