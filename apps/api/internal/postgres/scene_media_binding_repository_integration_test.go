package postgres

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/creativeproposal"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/mediaasset"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/scenemedia"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneplan"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/script"
)

func TestSceneMediaBindingRepositoryIntegration(t *testing.T) {
	pool := integrationPool(t)
	projectRepository := NewProjectRepository(pool)
	proposalRepository := NewCreativeProposalRepository(pool)
	scriptRepository := NewScriptRepository(pool)
	planRepository := NewScenePlanRepository(pool)
	assetRepository := NewMediaAssetRepository(pool)
	bindingRepository := NewSceneMediaBindingRepository(pool)
	service := scenemedia.NewService(planRepository, assetRepository, bindingRepository)

	ownerA := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	ownerB := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	projectA, err := projectRepository.Create(context.Background(), ownerA, validIntegrationCreateInput("Scene media bindings"))
	if err != nil {
		t.Fatalf("create project A: %v", err)
	}
	projectB, err := projectRepository.Create(context.Background(), ownerB, validIntegrationCreateInput("Other project"))
	if err != nil {
		t.Fatalf("create project B: %v", err)
	}

	proposal, err := proposalRepository.CreateDraft(context.Background(), ownerA, projectA.ID, creativeproposalCreateDraftInput("Scene media proposal"))
	if err != nil {
		t.Fatalf("create proposal: %v", err)
	}
	proposal, err = proposalRepository.Approve(context.Background(), ownerA, projectA.ID, proposal.Version, proposal.Revision)
	if err != nil {
		t.Fatalf("approve proposal: %v", err)
	}
	scriptVersion, err := scriptRepository.CreateDraft(context.Background(), ownerA, projectA.ID, script.CreateDraftInput{
		SourceProposalVersion: proposal.Version,
		Content:               validScriptContent("Scene media source"),
	})
	if err != nil {
		t.Fatalf("create script: %v", err)
	}
	scriptVersion, err = scriptRepository.Approve(context.Background(), ownerA, projectA.ID, scriptVersion.Version, scriptVersion.Revision)
	if err != nil {
		t.Fatalf("approve script: %v", err)
	}
	plan, err := planRepository.CreateDraft(context.Background(), ownerA, projectA.ID, sceneplan.CreateDraftInput{
		SourceScriptVersion: scriptVersion.Version,
		Content:             validScenePlanContent("Scene media source"),
	})
	if err != nil {
		t.Fatalf("create scene plan: %v", err)
	}
	plan, err = planRepository.Approve(context.Background(), ownerA, projectA.ID, plan.Version, plan.Revision)
	if err != nil {
		t.Fatalf("approve scene plan: %v", err)
	}

	image := integrationAsset(projectA.ID, ownerA, 10)
	image, err = assetRepository.Create(context.Background(), image)
	if err != nil {
		t.Fatalf("create image asset: %v", err)
	}
	video := integrationAsset(projectA.ID, ownerA, 11)
	video.Kind = mediaasset.KindVideo
	video.MimeType = "video/mp4"
	video, err = assetRepository.Create(context.Background(), video)
	if err != nil {
		t.Fatalf("create video asset: %v", err)
	}
	audio := integrationAsset(projectA.ID, ownerA, 12)
	audio.Kind = mediaasset.KindAudio
	audio.MimeType = "audio/mpeg"
	audio, err = assetRepository.Create(context.Background(), audio)
	if err != nil {
		t.Fatalf("create audio asset: %v", err)
	}
	foreign := integrationAsset(projectB.ID, ownerB, 13)
	foreign, err = assetRepository.Create(context.Background(), foreign)
	if err != nil {
		t.Fatalf("create foreign asset: %v", err)
	}

	principal := project.Principal{OwnerID: ownerA}
	first, err := service.AssignPrimaryVisual(context.Background(), principal, projectA.ID, plan.Version, "intro", image.ID)
	if err != nil {
		t.Fatalf("first assignment: %v", err)
	}
	if first.BindingVersion != 1 || first.Status != scenemedia.StatusActive {
		t.Fatalf("unexpected first assignment: %+v", first)
	}
	replayed, err := service.AssignPrimaryVisual(context.Background(), principal, projectA.ID, plan.Version, "intro", image.ID)
	if err != nil {
		t.Fatalf("same asset assignment: %v", err)
	}
	if replayed.ID != first.ID || replayed.BindingVersion != first.BindingVersion {
		t.Fatalf("same asset assignment was not idempotent: first=%+v replay=%+v", first, replayed)
	}

	replacement, err := service.AssignPrimaryVisual(context.Background(), principal, projectA.ID, plan.Version, "intro", video.ID)
	if err != nil {
		t.Fatalf("replacement assignment: %v", err)
	}
	if replacement.BindingVersion != 2 || replacement.AssetID != video.ID {
		t.Fatalf("unexpected replacement: %+v", replacement)
	}
	history, err := service.ListHistory(context.Background(), principal, projectA.ID, plan.Version, "intro")
	if err != nil {
		t.Fatalf("list history: %v", err)
	}
	if len(history) != 2 || history[0].BindingVersion != 2 || history[1].BindingVersion != 1 || history[1].Status != scenemedia.StatusSuperseded {
		t.Fatalf("unexpected history: %+v", history)
	}

	current, err := service.ListCurrent(context.Background(), principal, projectA.ID, plan.Version)
	if err != nil {
		t.Fatalf("list current: %v", err)
	}
	if len(current) != 3 || current[0].Scene.Key != "intro" || current[1].Scene.Key != "main" || current[2].Scene.Key != "outro" {
		t.Fatalf("current list lost scene plan order: %+v", current)
	}
	if current[0].Binding == nil || current[0].Binding.AssetID != video.ID || current[1].Binding != nil || current[2].Binding != nil {
		t.Fatalf("current list did not represent bound/unbound scenes: %+v", current)
	}

	if _, err := service.AssignPrimaryVisual(context.Background(), principal, projectA.ID, plan.Version, "intro", audio.ID); !errors.Is(err, scenemedia.ErrMediaAssetNotVisual) {
		t.Fatalf("audio assignment error=%v, want nonvisual error", err)
	}
	if _, err := service.AssignPrimaryVisual(context.Background(), principal, projectA.ID, plan.Version, "intro", foreign.ID); !errors.Is(err, scenemedia.ErrMediaAssetNotFound) {
		t.Fatalf("cross-owner/project assignment error=%v, want not found", err)
	}
	if _, err := bindingRepository.AssignPrimaryVisual(context.Background(), ownerA, projectA.ID, plan.Version, "missing", image.ID); !errors.Is(err, scenemedia.ErrSceneKeyNotFound) {
		t.Fatalf("unknown scene error=%v, want scene key not found", err)
	}

	draftPlan, err := planRepository.CreateDraft(context.Background(), ownerA, projectA.ID, sceneplan.CreateDraftInput{
		SourceScriptVersion: scriptVersion.Version,
		Content:             scenePlanContent("Scene media source", "draft scene media"),
	})
	if err != nil {
		t.Fatalf("create draft plan: %v", err)
	}
	if _, err := bindingRepository.AssignPrimaryVisual(context.Background(), ownerA, projectA.ID, draftPlan.Version, "intro", image.ID); !errors.Is(err, scenemedia.ErrScenePlanNotApproved) {
		t.Fatalf("draft plan assignment error=%v, want not approved", err)
	}

	if err := assetRepository.Delete(context.Background(), ownerA, projectA.ID, video.ID); err == nil {
		t.Fatal("expected referenced asset deletion to be restricted")
	}
	if _, err := assetRepository.Get(context.Background(), ownerA, projectA.ID, video.ID); err != nil {
		t.Fatalf("referenced asset disappeared after rejected delete: %v", err)
	}

	// The database constraints remain authoritative even when callers bypass the
	// service/repository validation boundary.
	_, err = pool.Exec(context.Background(), `
		INSERT INTO scene_media_bindings (
			owner_id, project_id, scene_plan_version, scene_key, role,
			binding_version, asset_id, status
		) VALUES ($1, $2, $3, 'intro', 'primary_visual', 99, $4, 'active')
	`, ownerA, projectA.ID, plan.Version, image.ID)
	if err == nil {
		t.Fatal("expected database one-active constraint failure")
	}
	_, err = pool.Exec(context.Background(), `
		INSERT INTO scene_media_bindings (
			owner_id, project_id, scene_plan_version, scene_key, role,
			binding_version, asset_id, status
		) VALUES ($1, $2, $3, 'main', 'primary_visual', 1, $4, 'active')
	`, ownerA, projectA.ID, plan.Version, foreign.ID)
	if err == nil {
		t.Fatal("expected database cross-project asset constraint failure")
	}
}

func TestSceneMediaBindingRepositoryIntegrationConcurrentReplacement(t *testing.T) {
	pool := integrationPool(t)
	projectRepository := NewProjectRepository(pool)
	proposalRepository := NewCreativeProposalRepository(pool)
	scriptRepository := NewScriptRepository(pool)
	planRepository := NewScenePlanRepository(pool)
	assetRepository := NewMediaAssetRepository(pool)
	bindingRepository := NewSceneMediaBindingRepository(pool)
	ownerID := uuid.MustParse("33333333-3333-4333-8333-333333333333")
	projectItem, err := projectRepository.Create(context.Background(), ownerID, validIntegrationCreateInput("Concurrent scene media"))
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	proposal, err := proposalRepository.CreateDraft(context.Background(), ownerID, projectItem.ID, creativeproposalCreateDraftInput("Concurrent proposal"))
	if err != nil {
		t.Fatalf("create proposal: %v", err)
	}
	proposal, err = proposalRepository.Approve(context.Background(), ownerID, projectItem.ID, proposal.Version, proposal.Revision)
	if err != nil {
		t.Fatalf("approve proposal: %v", err)
	}
	scriptItem, err := scriptRepository.CreateDraft(context.Background(), ownerID, projectItem.ID, script.CreateDraftInput{SourceProposalVersion: proposal.Version, Content: validScriptContent("Concurrent source")})
	if err != nil {
		t.Fatalf("create script: %v", err)
	}
	scriptItem, err = scriptRepository.Approve(context.Background(), ownerID, projectItem.ID, scriptItem.Version, scriptItem.Revision)
	if err != nil {
		t.Fatalf("approve script: %v", err)
	}
	plan, err := planRepository.CreateDraft(context.Background(), ownerID, projectItem.ID, sceneplan.CreateDraftInput{SourceScriptVersion: scriptItem.Version, Content: validScenePlanContent("Concurrent source")})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	plan, err = planRepository.Approve(context.Background(), ownerID, projectItem.ID, plan.Version, plan.Revision)
	if err != nil {
		t.Fatalf("approve plan: %v", err)
	}

	assets := make([]mediaasset.MediaAsset, 8)
	for i := range assets {
		asset, err := assetRepository.Create(context.Background(), integrationAsset(projectItem.ID, ownerID, 20+i))
		if err != nil {
			t.Fatalf("create asset %d: %v", i, err)
		}
		assets[i] = asset
	}

	start := make(chan struct{})
	errorsCh := make(chan error, len(assets))
	var wg sync.WaitGroup
	for _, asset := range assets {
		wg.Add(1)
		go func(asset mediaasset.MediaAsset) {
			defer wg.Done()
			<-start
			_, err := bindingRepository.AssignPrimaryVisual(context.Background(), ownerID, projectItem.ID, plan.Version, "outro", asset.ID)
			errorsCh <- err
		}(asset)
	}
	close(start)
	wg.Wait()
	close(errorsCh)
	for err := range errorsCh {
		if err != nil {
			t.Fatalf("concurrent replacement: %v", err)
		}
	}

	history, err := bindingRepository.ListHistory(context.Background(), ownerID, projectItem.ID, plan.Version, "outro")
	if err != nil {
		t.Fatalf("list concurrent history: %v", err)
	}
	if len(history) != len(assets) || history[0].BindingVersion != len(assets) {
		t.Fatalf("unexpected concurrent history length/first version: len=%d history=%+v", len(history), history)
	}
	seenVersions := make(map[int]bool, len(history))
	active := 0
	for _, binding := range history {
		if seenVersions[binding.BindingVersion] {
			t.Fatalf("duplicate binding version %d", binding.BindingVersion)
		}
		seenVersions[binding.BindingVersion] = true
		if binding.Status == scenemedia.StatusActive {
			active++
		}
	}
	if active != 1 || len(seenVersions) != len(assets) {
		t.Fatalf("concurrent replacement active=%d versions=%v", active, seenVersions)
	}
}

func creativeproposalCreateDraftInput(title string) creativeproposal.CreateDraftInput {
	return creativeproposal.CreateDraftInput{SourceBriefRevision: 1, Content: validProposalContent(title)}
}
