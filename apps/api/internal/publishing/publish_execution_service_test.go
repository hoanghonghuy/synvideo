package publishing

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
)

type executionAttemptRepo struct{ attempt PublishAttempt }

func (r *executionAttemptRepo) CreateAttempt(context.Context, PublishAttempt) (PublishAttempt, error) {
	panic("not used")
}
func (r *executionAttemptRepo) GetAttempt(_ context.Context, ownerID, projectID, attemptID uuid.UUID) (PublishAttempt, error) {
	if r.attempt.OwnerID != ownerID || r.attempt.ProjectID != projectID || r.attempt.ID != attemptID {
		return PublishAttempt{}, ErrAttemptNotFound
	}
	return r.attempt, nil
}
func (r *executionAttemptRepo) GetAttemptByRequest(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) (PublishAttempt, error) {
	panic("not used")
}
func (r *executionAttemptRepo) SaveAttemptProgress(_ context.Context, attempt PublishAttempt, sessionURI string, uploadedBytes int64, lastErrorCode string) (PublishAttempt, error) {
	attempt.ResumableSessionURI = sessionURI
	attempt.UploadedBytes = uploadedBytes
	attempt.LastErrorCode = lastErrorCode
	r.attempt = attempt
	return attempt, nil
}

type executionCredentialReader struct{ token string }

func (r executionCredentialReader) RefreshToken(context.Context, uuid.UUID, uuid.UUID) (string, error) {
	return r.token, nil
}

type executionOAuthRefresher struct{ token OAuthToken }

func (r executionOAuthRefresher) Refresh(context.Context, string) (OAuthToken, error) {
	return r.token, nil
}

type executionUploader struct {
	initiated      ResumableUploadResult
	uploaded       ResumableUploadResult
	initiateCalls  int
	uploadStart    int64
	uploadLength   int64
	uploadSession  string
	uploadContents []byte
}

func (u *executionUploader) Initiate(context.Context, string, YouTubeUploadMetadata, string, int64) (ResumableUploadResult, error) {
	u.initiateCalls++
	return u.initiated, nil
}
func (u *executionUploader) UploadChunk(_ context.Context, _ string, sessionURI, _ string, body io.Reader, start, length, _ int64) (ResumableUploadResult, error) {
	u.uploadSession = sessionURI
	u.uploadStart = start
	u.uploadLength = length
	u.uploadContents, _ = io.ReadAll(body)
	return u.uploaded, nil
}

func newExecutionAttempt() PublishAttempt {
	now := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
	return PublishAttempt{
		ID:               uuid.New(),
		OwnerID:          uuid.New(),
		ProjectID:        uuid.New(),
		ConnectionID:     uuid.New(),
		RenderArtifactID: uuid.New(),
		RequestID:        uuid.New(),
		Provider:         ProviderYouTube,
		State:            PublishQueued,
		Title:            "Release video",
		Description:      "description",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func TestPublishExecutionInitiatesAndCompletesChunk(t *testing.T) {
	attempt := newExecutionAttempt()
	repo := &executionAttemptRepo{attempt: attempt}
	uploader := &executionUploader{
		initiated: ResumableUploadResult{SessionURI: "https://upload.test/session"},
		uploaded: ResumableUploadResult{
			SessionURI:    "https://upload.test/session",
			UploadedBytes: 4,
			RemoteVideoID: "video-123",
			Complete:      true,
		},
	}
	service, err := NewPublishExecutionService(repo, executionCredentialReader{token: "refresh"}, executionOAuthRefresher{token: OAuthToken{AccessToken: "access"}}, uploader)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return attempt.UpdatedAt.Add(time.Minute) }

	got, err := service.ExecuteChunk(context.Background(), attempt.OwnerID, attempt.ProjectID, attempt.ID, "video/mp4", 4, 4, bytes.NewBufferString("data"))
	if err != nil {
		t.Fatal(err)
	}
	if uploader.initiateCalls != 1 || uploader.uploadSession != "https://upload.test/session" || uploader.uploadStart != 0 || uploader.uploadLength != 4 || string(uploader.uploadContents) != "data" {
		t.Fatalf("unexpected upload call: %+v", uploader)
	}
	if got.State != PublishUploadAccepted || got.RemoteVideoID != "video-123" || got.UploadedBytes != 4 {
		t.Fatalf("unexpected attempt: %+v", got)
	}
}

func TestPublishExecutionResumesPersistedSessionWithoutReinitiation(t *testing.T) {
	attempt := newExecutionAttempt()
	attempt.State = PublishRetryableFailure
	attempt.ResumableSessionURI = "https://upload.test/existing"
	attempt.UploadedBytes = 3
	repo := &executionAttemptRepo{attempt: attempt}
	uploader := &executionUploader{uploaded: ResumableUploadResult{
		SessionURI:    "https://upload.test/existing",
		UploadedBytes: 5,
	}}
	service, err := NewPublishExecutionService(repo, executionCredentialReader{token: "refresh"}, executionOAuthRefresher{token: OAuthToken{AccessToken: "access"}}, uploader)
	if err != nil {
		t.Fatal(err)
	}

	got, err := service.ExecuteChunk(context.Background(), attempt.OwnerID, attempt.ProjectID, attempt.ID, "video/mp4", 8, 2, bytes.NewBufferString("de"))
	if err != nil {
		t.Fatal(err)
	}
	if uploader.initiateCalls != 0 || uploader.uploadStart != 3 || uploader.uploadSession != attempt.ResumableSessionURI {
		t.Fatalf("resume did not use persisted provider state: %+v", uploader)
	}
	if got.State != PublishUploading || got.UploadedBytes != 5 {
		t.Fatalf("unexpected resumed attempt: %+v", got)
	}
}

func TestPublishExecutionPersistsReconnectFailure(t *testing.T) {
	attempt := newExecutionAttempt()
	repo := &executionAttemptRepo{attempt: attempt}
	uploader := &executionUploader{initiated: ResumableUploadResult{Failure: UploadFailureReconnectRequired, ProviderCode: 401}}
	service, err := NewPublishExecutionService(repo, executionCredentialReader{token: "refresh"}, executionOAuthRefresher{token: OAuthToken{AccessToken: "access"}}, uploader)
	if err != nil {
		t.Fatal(err)
	}

	got, err := service.ExecuteChunk(context.Background(), attempt.OwnerID, attempt.ProjectID, attempt.ID, "video/mp4", 4, 4, bytes.NewBufferString("data"))
	if err != nil {
		t.Fatal(err)
	}
	if got.State != PublishReconnectRequired || got.LastErrorCode != "youtube_reconnect_required" {
		t.Fatalf("unexpected reconnect attempt: %+v", got)
	}
	if uploader.uploadSession != "" {
		t.Fatalf("upload should not run after reconnect-required initiation: %+v", uploader)
	}
}
