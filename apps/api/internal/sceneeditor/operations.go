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
	return PreviewReconciliation(doc, candidate)
}

func (s *Service) Reconcile(ctx context.Context, ownerID, projectID uuid.UUID, input ReconcileInput) (View, error) {
	if ownerID == uuid.Nil {
		return View{}, ErrUnauthenticated
	}
	doc, err := s.repo.GetLatest(ctx, ownerID, projectID)
	if err != nil {
		return View{}, normalizeRepoError(err)
	}
	expectedDigest, err := ReconcilePreviewDigest(doc.Revision, doc.ScenePlanVersion, input.Candidate.ScenePlanVersion, input.Candidate)
	if err != nil {
		return View{}, err
	}
	if input.PreviewDigest == "" || input.PreviewDigest != expectedDigest {
		return View{}, ErrPreviewStale
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
