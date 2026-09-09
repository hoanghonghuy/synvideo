package sceneeditor

import (
	"context"

	"github.com/google/uuid"
)

type UpdateInput struct {
	ExpectedRevision int          `json:"expected_revision"`
	Scenes           []Scene      `json:"scenes"`
	AudioMix         *AudioMixRef `json:"audio_mix,omitempty"`
}

type CreateInput struct {
	ScenePlanVersion int          `json:"scene_plan_version"`
	Scenes           []Scene      `json:"scenes"`
	AudioMix         *AudioMixRef `json:"audio_mix,omitempty"`
}

type ReconcileInput struct {
	ExpectedRevision int                `json:"expected_revision"`
	PreviewDigest    string             `json:"preview_digest"`
	Candidate        ReconcileCandidate `json:"candidate"`
}

func (s *Service) Update(ctx context.Context, ownerID, projectID uuid.UUID, input UpdateInput) (View, error) {
	return s.Save(ctx, ownerID, projectID, input.ExpectedRevision, func(doc Document) (Document, error) {
		doc.Scenes = cloneScenes(input.Scenes)
		doc.AudioMix = cloneAudioMix(input.AudioMix)
		return doc, nil
	})
}

func (s *Service) PreviewReconcile(ctx context.Context, ownerID, projectID uuid.UUID, candidate ReconcileCandidate) (ReconcilePreview, error) {
	if ownerID == uuid.Nil {
		return ReconcilePreview{}, ErrUnauthenticated
	}
	doc, err := s.repo.GetLatest(ctx, ownerID, projectID)
	if err != nil {
		return ReconcilePreview{}, normalizeRepoError(err)
	}
	if err := s.validateWriteDependencies(ctx, ownerID, candidateDependencyDocument(doc, candidate)); err != nil {
		return ReconcilePreview{}, err
	}
	preview, err := PreviewReconciliation(doc, candidate)
	if err != nil {
		return ReconcilePreview{}, err
	}
	return s.attachReconcilePreviewDigest(ctx, ownerID, doc, candidate, preview)
}

func (s *Service) Reconcile(ctx context.Context, ownerID, projectID uuid.UUID, input ReconcileInput) (View, error) {
	if ownerID == uuid.Nil {
		return View{}, ErrUnauthenticated
	}
	doc, err := s.repo.GetLatest(ctx, ownerID, projectID)
	if err != nil {
		return View{}, normalizeRepoError(err)
	}
	if err := s.validateReconcilePreviewDigest(ctx, ownerID, doc, input); err != nil {
		return View{}, err
	}
	updated, err := ApplyReconciliation(doc, input.Candidate, input.ExpectedRevision, s.now().UTC(), s.newID)
	if err != nil {
		return View{}, err
	}
	if err := s.validateWriteDependencies(ctx, ownerID, updated); err != nil {
		return View{}, err
	}
	saved, err := s.repo.CreateRevision(ctx, updated, input.ExpectedRevision)
	if err != nil {
		return View{}, normalizeRepoError(err)
	}
	return s.view(ctx, ownerID, saved)
}

func (s *Service) attachReconcilePreviewDigest(ctx context.Context, ownerID uuid.UUID, doc Document, candidate ReconcileCandidate, preview ReconcilePreview) (ReconcilePreview, error) {
	fingerprint, err := s.reconcileUpstreamFingerprint(ctx, ownerID, doc, candidate)
	if err != nil {
		return ReconcilePreview{}, err
	}
	digest, err := ReconcilePreviewDigest(doc.Revision, doc.ScenePlanVersion, candidate.ScenePlanVersion, candidate, fingerprint)
	if err != nil {
		return ReconcilePreview{}, err
	}
	preview.PreviewDigest = digest
	return preview, nil
}

func (s *Service) validateReconcilePreviewDigest(ctx context.Context, ownerID uuid.UUID, doc Document, input ReconcileInput) error {
	fingerprint, err := s.reconcileUpstreamFingerprint(ctx, ownerID, doc, input.Candidate)
	if err != nil {
		return err
	}
	expectedDigest, err := ReconcilePreviewDigest(doc.Revision, doc.ScenePlanVersion, input.Candidate.ScenePlanVersion, input.Candidate, fingerprint)
	if err != nil {
		return err
	}
	if input.PreviewDigest == "" || input.PreviewDigest != expectedDigest {
		return ErrPreviewStale
	}
	return nil
}

func (s *Service) reconcileUpstreamFingerprint(ctx context.Context, ownerID uuid.UUID, doc Document, candidate ReconcileCandidate) (string, error) {
	states, err := s.resolver.State(ctx, ownerID, candidateDependencyDocument(doc, candidate))
	if err != nil {
		return "", err
	}
	return DependencyFingerprint(states)
}

func candidateDependencyDocument(base Document, candidate ReconcileCandidate) Document {
	doc := base
	doc.ScenePlanVersion = candidate.ScenePlanVersion
	doc.AudioMix = cloneAudioMix(candidate.AudioMix)
	doc.Scenes = make([]Scene, 0, len(candidate.Scenes))
	for _, candidateScene := range candidate.Scenes {
		doc.Scenes = append(doc.Scenes, Scene{
			SceneKey:  candidateScene.SceneKey,
			Visual:    cloneVisual(candidateScene.Visual),
			Narration: cloneNarration(candidateScene.Narration),
			Caption:   cloneCaption(candidateScene.Caption),
		})
	}
	return doc
}
