package publishing

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

type recoveryAttemptRepo struct {
	attempt PublishAttempt
	saved   PublishAttempt
}

func (r *recoveryAttemptRepo) CreateAttempt(context.Context, PublishAttempt) (PublishAttempt, error) {
	panic("not used")
}
func (r *recoveryAttemptRepo) GetAttempt(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (PublishAttempt, error) {
	return r.attempt, nil
}
func (r *recoveryAttemptRepo) GetAttemptByRequest(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) (PublishAttempt, error) {
	panic("not used")
}
func (r *recoveryAttemptRepo) SaveAttemptProgress(_ context.Context, attempt PublishAttempt, sessionURI string, uploadedBytes int64, lastErrorCode string) (PublishAttempt, error) {
	attempt.ResumableSessionURI = sessionURI
	attempt.UploadedBytes = uploadedBytes
	attempt.LastErrorCode = lastErrorCode
	r.saved = attempt
	return attempt, nil
}

type recoveryCredentials struct{ token string }

func (r recoveryCredentials) RefreshToken(context.Context, uuid.UUID, uuid.UUID) (string, error) {
	return r.token, nil
}

type recoveryOAuth struct {
	refreshSeen string
	accessToken string
}

func (r *recoveryOAuth) Refresh(_ context.Context, refreshToken string) (OAuthToken, error) {
	r.refreshSeen = refreshToken
	return OAuthToken{AccessToken: r.accessToken}, nil
}

type recoveryUploader struct {
	accessSeen  string
	sessionSeen string
	result      ResumableUploadResult
}

func (r *recoveryUploader) Query(_ context.Context, accessToken, sessionURI string, _ int64) (ResumableUploadResult, error) {
	r.accessSeen = accessToken
	r.sessionSeen = sessionURI
	return r.result, nil
}

func TestPublishRecoveryUsesDurableSessionAndProviderProgress(t *testing.T) {
	now := time.Now().UTC()
	attempt := validAttempt(now)
	attempt.State = PublishRetryableFailure
	attempt.ResumableSessionURI = "https://upload.example/session-old"
	attempt.UploadedBytes = 128
	attempt.LastErrorCode = "youtube_retryable"

	repo := &recoveryAttemptRepo{attempt: attempt}
	oauth := &recoveryOAuth{accessToken: "access-new"}
	uploader := &recoveryUploader{result: ResumableUploadResult{
		SessionURI:    "https://upload.example/session-rotated",
		UploadedBytes: 512,
	}}
	service, err := NewPublishRecoveryService(repo, recoveryCredentials{token: "refresh-secret"}, oauth, uploader)
	if err != nil {
		t.Fatalf("new recovery service: %v", err)
	}
	service.now = func() time.Time { return now.Add(time.Minute) }

	got, err := service.Recover(context.Background(), attempt.OwnerID, attempt.ProjectID, attempt.ID, 1024)
	if err != nil {
		t.Fatalf("recover attempt: %v", err)
	}
	if oauth.refreshSeen != "refresh-secret" || uploader.accessSeen != "access-new" {
		t.Fatalf("expected server-side token refresh to feed provider query")
	}
	if uploader.sessionSeen != attempt.ResumableSessionURI {
		t.Fatalf("expected durable session URI to be queried, got %q", uploader.sessionSeen)
	}
	if got.State != PublishUploading || got.UploadedBytes != 512 || got.ResumableSessionURI != uploader.result.SessionURI || got.LastErrorCode != "" {
		t.Fatalf("unexpected recovered attempt: %+v", got)
	}
}

func TestPublishRecoveryMapsReconnectWithoutLosingProgress(t *testing.T) {
	now := time.Now().UTC()
	attempt := validAttempt(now)
	attempt.State = PublishUploading
	attempt.ResumableSessionURI = "https://upload.example/session"
	attempt.UploadedBytes = 256

	repo := &recoveryAttemptRepo{attempt: attempt}
	uploader := &recoveryUploader{result: ResumableUploadResult{Failure: UploadFailureReconnectRequired}}
	service, err := NewPublishRecoveryService(repo, recoveryCredentials{token: "refresh"}, &recoveryOAuth{accessToken: "access"}, uploader)
	if err != nil {
		t.Fatalf("new recovery service: %v", err)
	}
	service.now = func() time.Time { return now.Add(time.Minute) }

	got, err := service.Recover(context.Background(), attempt.OwnerID, attempt.ProjectID, attempt.ID, 1024)
	if err != nil {
		t.Fatalf("recover attempt: %v", err)
	}
	if got.State != PublishReconnectRequired || got.UploadedBytes != 256 || got.ResumableSessionURI != attempt.ResumableSessionURI {
		t.Fatalf("expected reconnect state to retain durable progress: %+v", got)
	}
	if got.LastErrorCode != "youtube_reconnect_required" {
		t.Fatalf("unexpected error code %q", got.LastErrorCode)
	}
}
