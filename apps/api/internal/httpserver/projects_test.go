package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/config"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/project"
)

func TestCreateProjectEndpoint(t *testing.T) {
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	server := newProjectTestServer(ownerID, newMemoryProjectRepository())

	response := performRequest(server, http.MethodPost, "/api/v1/projects", `{
		"title": "Du an dau tien",
		"description": "Mo ta ngan",
		"content_format": "short",
		"aspect_ratio": "9:16",
		"target_duration_seconds": 60,
		"locale": "vi"
	}`)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, response.Code, response.Body.String())
	}

	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["title"] != "Du an dau tien" {
		t.Fatalf("expected project title in response, got %#v", body)
	}
	if _, exists := body["owner_id"]; exists {
		t.Fatal("owner_id must not be exposed in project response")
	}
}

func TestCreateProjectEndpointValidation(t *testing.T) {
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	server := newProjectTestServer(ownerID, newMemoryProjectRepository())

	response := performRequest(server, http.MethodPost, "/api/v1/projects", `{
		"title": "",
		"content_format": "bad",
		"aspect_ratio": "9:16"
	}`)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, response.Code, response.Body.String())
	}
	if !bytes.Contains(response.Body.Bytes(), []byte("validation_failed")) {
		t.Fatalf("expected validation error envelope, got %s", response.Body.String())
	}
}

func TestListProjectEndpointPagination(t *testing.T) {
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	repository := newMemoryProjectRepository()
	service := project.NewService(repository)
	for _, title := range []string{"Mot", "Hai", "Ba"} {
		if _, err := service.Create(context.Background(), project.Principal{OwnerID: ownerID}, validCreateInput(title)); err != nil {
			t.Fatalf("seed project: %v", err)
		}
	}
	server := newProjectTestServer(ownerID, repository)

	firstPage := performRequest(server, http.MethodGet, "/api/v1/projects?limit=2", "")
	if firstPage.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, firstPage.Code, firstPage.Body.String())
	}

	var first listProjectsResponse
	if err := json.NewDecoder(firstPage.Body).Decode(&first); err != nil {
		t.Fatalf("decode first page: %v", err)
	}
	if len(first.Projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(first.Projects))
	}
	if first.NextCursor == "" {
		t.Fatal("expected next cursor")
	}

	secondPage := performRequest(server, http.MethodGet, "/api/v1/projects?limit=2&cursor="+first.NextCursor, "")
	if secondPage.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, secondPage.Code, secondPage.Body.String())
	}

	var second listProjectsResponse
	if err := json.NewDecoder(secondPage.Body).Decode(&second); err != nil {
		t.Fatalf("decode second page: %v", err)
	}
	if len(second.Projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(second.Projects))
	}
}

func TestGetProjectEndpointNotFoundForDifferentOwner(t *testing.T) {
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	otherOwnerID := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	repository := newMemoryProjectRepository()
	service := project.NewService(repository)
	created, err := service.Create(context.Background(), project.Principal{OwnerID: otherOwnerID}, validCreateInput("Hidden"))
	if err != nil {
		t.Fatalf("seed project: %v", err)
	}
	server := newProjectTestServer(ownerID, repository)

	response := performRequest(server, http.MethodGet, "/api/v1/projects/"+created.ID.String(), "")

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, response.Code, response.Body.String())
	}
}

func TestUpdateProjectEndpoint(t *testing.T) {
	ownerID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	repository := newMemoryProjectRepository()
	service := project.NewService(repository)
	created, err := service.Create(context.Background(), project.Principal{OwnerID: ownerID}, validCreateInput("Draft"))
	if err != nil {
		t.Fatalf("seed project: %v", err)
	}
	server := newProjectTestServer(ownerID, repository)

	response := performRequest(server, http.MethodPatch, "/api/v1/projects/"+created.ID.String(), `{
		"title": "Da sua",
		"status": "archived",
		"target_duration_seconds": null
	}`)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, response.Code, response.Body.String())
	}
	var body projectResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Title != "Da sua" || body.Status != project.StatusArchived || body.TargetDurationSeconds != nil {
		t.Fatalf("unexpected update response: %#v", body)
	}
}

func newProjectTestServer(ownerID uuid.UUID, repository project.Repository) *http.Server {
	cfg := config.Config{Addr: ":0", Environment: config.EnvironmentTest}
	return New(cfg, slog.Default(), project.NewService(repository), fixedResolver{ownerID: ownerID})
}

func performRequest(server *http.Server, method string, path string, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	server.Handler.ServeHTTP(recorder, request)
	return recorder
}

type fixedResolver struct {
	ownerID uuid.UUID
}

func (r fixedResolver) Resolve(_ *http.Request) (project.Principal, error) {
	return project.Principal{OwnerID: r.ownerID}, nil
}

type memoryProjectRepository struct {
	projects map[uuid.UUID]project.Project
}

func newMemoryProjectRepository() *memoryProjectRepository {
	return &memoryProjectRepository{projects: map[uuid.UUID]project.Project{}}
}

func (r *memoryProjectRepository) Create(_ context.Context, ownerID uuid.UUID, input project.CreateInput) (project.Project, error) {
	now := time.Now().UTC()
	item := project.Project{
		ID:                    uuid.New(),
		OwnerID:               ownerID,
		Title:                 input.Title,
		Description:           input.Description,
		ContentFormat:         input.ContentFormat,
		AspectRatio:           input.AspectRatio,
		TargetDurationSeconds: input.TargetDurationSeconds,
		Locale:                input.Locale,
		Status:                project.StatusActive,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	r.projects[item.ID] = item
	return item, nil
}

func (r *memoryProjectRepository) List(_ context.Context, ownerID uuid.UUID, options project.ListOptions) (project.ListResult, error) {
	items := make([]project.Project, 0)
	for _, item := range r.projects {
		if item.OwnerID == ownerID {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].ID.String() > items[j].ID.String()
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	if options.Cursor != nil {
		filtered := make([]project.Project, 0, len(items))
		for _, item := range items {
			beforeCursor := item.UpdatedAt.Before(options.Cursor.UpdatedAt) ||
				(item.UpdatedAt.Equal(options.Cursor.UpdatedAt) && item.ID.String() < options.Cursor.ID.String())
			if beforeCursor {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}

	var nextCursor *project.ListCursor
	if len(items) > options.Limit {
		cursorItem := items[options.Limit-1]
		nextCursor = &project.ListCursor{UpdatedAt: cursorItem.UpdatedAt, ID: cursorItem.ID}
		items = items[:options.Limit]
	}
	return project.ListResult{Projects: items, NextCursor: nextCursor}, nil
}

func (r *memoryProjectRepository) Get(_ context.Context, ownerID uuid.UUID, id uuid.UUID) (project.Project, error) {
	item, ok := r.projects[id]
	if !ok || item.OwnerID != ownerID {
		return project.Project{}, project.ErrNotFound
	}
	return item, nil
}

func (r *memoryProjectRepository) Update(_ context.Context, ownerID uuid.UUID, id uuid.UUID, input project.UpdateInput) (project.Project, error) {
	item, ok := r.projects[id]
	if !ok || item.OwnerID != ownerID {
		return project.Project{}, project.ErrNotFound
	}
	if input.Title != nil {
		item.Title = *input.Title
	}
	if input.Description != nil {
		item.Description = *input.Description
	}
	if input.ContentFormat != nil {
		item.ContentFormat = *input.ContentFormat
	}
	if input.AspectRatio != nil {
		item.AspectRatio = *input.AspectRatio
	}
	if input.TargetDurationSeconds != nil {
		item.TargetDurationSeconds = *input.TargetDurationSeconds
	}
	if input.Locale != nil {
		item.Locale = *input.Locale
	}
	if input.Status != nil {
		item.Status = *input.Status
	}
	item.UpdatedAt = time.Now().UTC()
	r.projects[id] = item
	return item, nil
}

func validCreateInput(title string) project.CreateInput {
	duration := 60
	return project.CreateInput{
		Title:                 title,
		ContentFormat:         project.ContentFormatShort,
		AspectRatio:           project.AspectRatio9x16,
		TargetDurationSeconds: &duration,
		Locale:                project.LocaleVI,
	}
}
