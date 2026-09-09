package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/actor"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/config"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/creativeproposal"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/httpserver"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneeditor"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneplan"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/script"
)

func TestUpstreamBridgeHTTPForkDoesNotMutateComposition(t *testing.T) {
	pool := integrationPool(t)
	ctx := context.Background()

	ownerID := uuid.New()
	principal := project.Principal{OwnerID: ownerID}
	projectRepo := NewProjectRepository(pool)
	proposalRepo := NewCreativeProposalRepository(pool)
	scriptRepo := NewScriptRepository(pool)
	scenePlanRepo := NewScenePlanRepository(pool)
	sceneEditorRepo := NewSceneEditorRepository(pool)
	sceneEditorResolver := NewSceneEditorDependencyResolver(pool)
	sceneEditorSvc := sceneeditor.NewService(sceneEditorRepo, sceneEditorResolver, uuid.New, func() time.Time {
		return time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	})
	scriptSvc := script.NewService(scriptRepo)

	projectItem, err := projectRepo.Create(ctx, ownerID, validIntegrationCreateInput("TASK-051 Upstream Bridge"))
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	proposal, err := proposalRepo.CreateDraft(ctx, ownerID, projectItem.ID, creativeproposal.CreateDraftInput{
		SourceBriefRevision: 1,
		Content:             validProposalContent("Upstream Bridge Proposal"),
	})
	if err != nil {
		t.Fatalf("create proposal: %v", err)
	}
	proposal, err = proposalRepo.Approve(ctx, ownerID, projectItem.ID, proposal.Version, proposal.Revision)
	if err != nil {
		t.Fatalf("approve proposal: %v", err)
	}

	scriptDraft, err := scriptRepo.CreateDraft(ctx, ownerID, projectItem.ID, script.CreateDraftInput{
		SourceProposalVersion: proposal.Version,
		Content:               validScriptContent("Upstream Bridge Script"),
	})
	if err != nil {
		t.Fatalf("create script: %v", err)
	}
	approvedScript, err := scriptRepo.Approve(ctx, ownerID, projectItem.ID, scriptDraft.Version, scriptDraft.Revision)
	if err != nil {
		t.Fatalf("approve script: %v", err)
	}

	planDraft, err := scenePlanRepo.CreateDraft(ctx, ownerID, projectItem.ID, sceneplan.CreateDraftInput{
		SourceScriptVersion: approvedScript.Version,
		Content:             validScenePlanContent("Upstream Bridge Plan"),
	})
	if err != nil {
		t.Fatalf("create scene plan: %v", err)
	}
	approvedPlan, err := scenePlanRepo.Approve(ctx, ownerID, projectItem.ID, planDraft.Version, planDraft.Revision)
	if err != nil {
		t.Fatalf("approve scene plan: %v", err)
	}

	sceneID := uuid.New()
	_, err = sceneEditorSvc.Create(ctx, ownerID, projectItem.ID, approvedPlan.Version, []sceneeditor.Scene{{
		ID: sceneID, SceneKey: "intro", DurationMS: 2_000, Notes: "creator presentation note",
		VisualTreatment: sceneeditor.VisualTreatment{Fit: sceneeditor.FitContain, Scale: 1.25, PositionX: 0.1},
		TransitionOut:   sceneeditor.Transition{Kind: sceneeditor.TransitionFade, DurationMS: 300},
	}}, nil)
	if err != nil {
		t.Fatalf("create composition: %v", err)
	}

	server := newTask051UpstreamBridgeTestServer(scriptSvc, sceneEditorSvc, ownerID)
	defer server.Close()

	base := "/api/v1/projects/" + projectItem.ID.String()

	before := task051GetSceneEditor(t, server.Client(), server.URL+base+"/scene-editor")
	if before.Revision != 1 || before.State != "CURRENT" || before.ScenePlanVersion != approvedPlan.Version {
		t.Fatalf("initial composition: revision=%d state=%s plan=%d", before.Revision, before.State, before.ScenePlanVersion)
	}

	forked := task051ForkScript(t, server.Client(), server.URL+base+"/scripts/"+fmt.Sprint(approvedScript.Version)+"/fork")
	if forked.Status != "draft" || forked.Version <= approvedScript.Version {
		t.Fatalf("fork response: %#v", forked)
	}

	afterFork := task051GetSceneEditor(t, server.Client(), server.URL+base+"/scene-editor")
	if afterFork.Revision != before.Revision || afterFork.State != before.State || afterFork.ScenePlanVersion != before.ScenePlanVersion {
		t.Fatalf("composition mutated by fork: before=%+v after=%+v", before, afterFork)
	}
	if afterFork.Scenes[0].Notes != "creator presentation note" {
		t.Fatalf("presentation notes changed after fork: %#v", afterFork.Scenes[0])
	}

	approvedScriptV2, err := scriptRepo.Approve(ctx, ownerID, projectItem.ID, forked.Version, forked.Revision)
	if err != nil {
		t.Fatalf("approve forked script: %v", err)
	}

	planDraftV2, err := scenePlanRepo.CreateDraft(ctx, ownerID, projectItem.ID, sceneplan.CreateDraftInput{
		SourceScriptVersion: approvedScriptV2.Version,
		Content:             validScenePlanContent("Upstream Bridge Plan V2"),
	})
	if err != nil {
		t.Fatalf("create scene plan v2: %v", err)
	}
	approvedPlanV2, err := scenePlanRepo.Approve(ctx, ownerID, projectItem.ID, planDraftV2.Version, planDraftV2.Revision)
	if err != nil {
		t.Fatalf("approve scene plan v2: %v", err)
	}

	staleView := task051GetSceneEditor(t, server.Client(), server.URL+base+"/scene-editor")
	if staleView.State != "STALE" {
		t.Fatalf("expected stale composition after upstream plan advanced, got %s", staleView.State)
	}
	if staleView.Revision != afterFork.Revision {
		t.Fatalf("stale transition should not silently rebind revision: %d want %d", staleView.Revision, afterFork.Revision)
	}

	updated := task051PutSceneEditor(t, server.Client(), server.URL+base+"/scene-editor", map[string]any{
		"expected_revision": staleView.Revision,
		"scenes":            staleView.Scenes,
		"audio_mix":         staleView.AudioMix,
	})
	if updated.Revision != staleView.Revision+1 {
		t.Fatalf("expected saved revision increment, got %d", updated.Revision)
	}
	if updated.Scenes[0].Notes != "creator presentation note" {
		t.Fatalf("presentation notes lost on save while stale: %#v", updated.Scenes[0])
	}

	preview := task051PreviewReconcile(t, server.Client(), server.URL+base+"/scene-editor/reconcile/preview", map[string]any{
		"candidate": map[string]any{
			"scene_plan_version": approvedPlanV2.Version,
			"scenes": []map[string]any{
				{"scene_key": "intro"},
			},
		},
	})
	if preview.Ambiguous || len(preview.Changes) == 0 || !preview.Changes[0].PreservesEdits {
		t.Fatalf("unexpected reconcile preview: %+v", preview)
	}

	reconciled := task051Reconcile(t, server.Client(), server.URL+base+"/scene-editor/reconcile", map[string]any{
		"expected_revision": updated.Revision,
		"candidate": map[string]any{
			"scene_plan_version": approvedPlanV2.Version,
			"scenes": []map[string]any{
				{"scene_key": "intro"},
			},
		},
	})
	if reconciled.State != "CURRENT" || reconciled.ScenePlanVersion != approvedPlanV2.Version {
		t.Fatalf("reconciled composition: state=%s plan=%d", reconciled.State, reconciled.ScenePlanVersion)
	}
	if reconciled.Scenes[0].Notes != "creator presentation note" || reconciled.Scenes[0].VisualTreatment.Scale != 1.25 {
		t.Fatalf("presentation edits not preserved after reconcile: %#v", reconciled.Scenes[0])
	}

	_, err = scriptSvc.ForkApprovedDraft(ctx, principal, projectItem.ID, forked.Version)
	if err == nil {
		t.Fatalf("expected fork failure for mutable draft source")
	}
}

func newTask051UpstreamBridgeTestServer(scriptSvc *script.Service, sceneEditorSvc *sceneeditor.Service, ownerID uuid.UUID) *httptest.Server {
	cfg := config.Config{Environment: config.EnvironmentDevelopment, LocalActorID: &ownerID}
	resolver := actor.NewLocalResolver(cfg)
	server := httpserver.New(cfg, nil, nil, nil, nil, scriptSvc, nil, nil, nil, nil, nil, resolver)
	server.Handler = httpserver.WithSceneEditorRoutes(nil, server.Handler, sceneEditorSvc, resolver)
	return httptest.NewServer(server.Handler)
}

type task051SceneEditorView struct {
	Revision         int            `json:"revision"`
	State            string         `json:"state"`
	ScenePlanVersion int            `json:"scene_plan_version"`
	Scenes           []task051Scene `json:"scenes"`
	AudioMix         map[string]any `json:"audio_mix"`
}

type task051Scene struct {
	ID              string                 `json:"id"`
	SceneKey        string                 `json:"scene_key"`
	Notes           string                 `json:"notes"`
	VisualTreatment task051VisualTreatment `json:"visual_treatment"`
}

type task051VisualTreatment struct {
	Scale float64 `json:"scale"`
}

type task051Script struct {
	Version  int    `json:"version"`
	Revision int    `json:"revision"`
	Status   string `json:"status"`
}

type task051ReconcilePreview struct {
	Ambiguous bool `json:"ambiguous"`
	Changes   []struct {
		PreservesEdits bool `json:"preserves_edits"`
	} `json:"changes"`
}

func task051GetSceneEditor(t *testing.T, client *http.Client, url string) task051SceneEditorView {
	resp := task051Do(t, client, http.MethodGet, url, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET scene editor status=%d body=%s", resp.StatusCode, task051ReadBody(resp))
	}
	var view task051SceneEditorView
	task051DecodeJSON(t, resp, &view)
	return view
}

func task051ForkScript(t *testing.T, client *http.Client, url string) task051Script {
	resp := task051Do(t, client, http.MethodPost, url, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST fork status=%d body=%s", resp.StatusCode, task051ReadBody(resp))
	}
	var scriptResp task051Script
	task051DecodeJSON(t, resp, &scriptResp)
	return scriptResp
}

func task051PutSceneEditor(t *testing.T, client *http.Client, url string, body map[string]any) task051SceneEditorView {
	resp := task051Do(t, client, http.MethodPut, url, body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT scene editor status=%d body=%s", resp.StatusCode, task051ReadBody(resp))
	}
	var view task051SceneEditorView
	task051DecodeJSON(t, resp, &view)
	return view
}

func task051PreviewReconcile(t *testing.T, client *http.Client, url string, body map[string]any) task051ReconcilePreview {
	resp := task051Do(t, client, http.MethodPost, url, body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST reconcile preview status=%d body=%s", resp.StatusCode, task051ReadBody(resp))
	}
	var preview task051ReconcilePreview
	task051DecodeJSON(t, resp, &preview)
	return preview
}

func task051Reconcile(t *testing.T, client *http.Client, url string, body map[string]any) task051SceneEditorView {
	resp := task051Do(t, client, http.MethodPost, url, body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST reconcile status=%d body=%s", resp.StatusCode, task051ReadBody(resp))
	}
	var view task051SceneEditorView
	task051DecodeJSON(t, resp, &view)
	return view
}

func task051Do(t *testing.T, client *http.Client, method, url string, body map[string]any) *http.Response {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	return resp
}

func task051DecodeJSON(t *testing.T, resp *http.Response, target any) {
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		t.Fatalf("decode json: %v", err)
	}
}

func task051ReadBody(resp *http.Response) string {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	return string(body)
}
