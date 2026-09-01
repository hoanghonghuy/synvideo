package postgres

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/creativeproposal"
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
	if _, err := projectRepo.Create(context.Background(), ownerB, validIntegrationCreateInput("Other Owner Project")); err != nil {
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
