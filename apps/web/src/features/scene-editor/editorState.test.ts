import { describe, expect, it } from 'vitest'

import type { SceneEditorScene, SceneEditorView } from './api'
import {
  cloneEditorView,
  editorContentSignature,
  normalizeTransitionForKind,
  semanticSceneSummary,
  validateEditableScene,
} from './editorState'

function scene(overrides: Partial<SceneEditorScene> = {}): SceneEditorScene {
  return {
    id: 'scene-local-1',
    scene_key: 'intro',
    duration_ms: 2_000,
    visual_treatment: {
      fit: 'contain',
      position_x: 0,
      position_y: 0,
      scale: 1,
      mute_video: true,
    },
    transition_out: { kind: 'cut', duration_ms: 0 },
    ...overrides,
  }
}

function view(): SceneEditorView {
  return {
    id: 'composition-1',
    project_id: 'project-1',
    revision: 3,
    scene_plan_version: 2,
    scenes: [scene()],
    created_at: '2026-09-07T00:00:00Z',
    updated_at: '2026-09-07T00:00:00Z',
    state: 'CURRENT',
  }
}

describe('Scene Editor draft semantics', () => {
  it('validates frozen bounds and authoritative narration/caption timing', () => {
    const editable = scene({
      duration_ms: 900,
      narration: { asset_id: 'a', binding_id: 'b', lineage_id: 'l', duration_ms: 1_000 },
      caption: { document_id: 'c', revision: 1, lineage_id: 'l', last_end_ms: 1_100 },
      visual_treatment: {
        fit: 'cover',
        position_x: 1.1,
        position_y: -1.1,
        scale: 4.1,
        mute_video: false,
      },
      transition_out: { kind: 'fade', duration_ms: 2_001 },
    })

    expect(validateEditableScene(editable)).toMatchObject({
      duration_ms: expect.any(String),
      position_x: expect.any(String),
      position_y: expect.any(String),
      scale: expect.any(String),
      transition_duration_ms: expect.any(String),
    })
  })

  it('enforces transition fit against both adjacent scenes', () => {
    const first = scene({ duration_ms: 1_000, transition_out: { kind: 'crossfade', duration_ms: 700 } })
    const second = scene({ id: 'scene-local-2', scene_key: 'outro', duration_ms: 600 })
    expect(validateEditableScene(first, second).transition_duration_ms).toContain('both adjacent scenes')
  })

  it('normalizes transition defaults without mutating unrelated fields', () => {
    const editable = scene({ transition_out: { kind: 'fade', duration_ms: 0 } })
    normalizeTransitionForKind(editable)
    expect(editable.transition_out.duration_ms).toBe(300)
    editable.transition_out.kind = 'cut'
    normalizeTransitionForKind(editable)
    expect(editable.transition_out.duration_ms).toBe(0)
  })

  it('tracks content dirtiness independently from server revision metadata', () => {
    const saved = view()
    const draft = cloneEditorView(saved)
    expect(editorContentSignature(draft)).toBe(editorContentSignature(saved))
    draft.scenes[0].visual_treatment.scale = 1.25
    expect(editorContentSignature(draft)).not.toBe(editorContentSignature(saved))
    expect(saved.scenes[0].visual_treatment.scale).toBe(1)
  })

  it('describes the same persisted semantics used by the render snapshot', () => {
    const editable = scene({
      visual: { asset_id: 'asset-1', binding_id: 'binding-1' },
      narration: { asset_id: 'audio-1', binding_id: 'narration-1', lineage_id: 'lineage-1', duration_ms: 1_200 },
      caption: { document_id: 'caption-1', revision: 2, lineage_id: 'lineage-1', last_end_ms: 1_500 },
      visual_treatment: { fit: 'cover', position_x: 0.25, position_y: -0.1, scale: 1.2, mute_video: true },
      transition_out: { kind: 'fade', duration_ms: 300 },
    })
    expect(semanticSceneSummary(editable)).toContain('intro: 2000ms')
    expect(semanticSceneSummary(editable)).toContain('cover, x 0.25, y -0.1, scale 1.2')
    expect(semanticSceneSummary(editable)).toContain('narration 1200ms')
    expect(semanticSceneSummary(editable)).toContain('captions through 1500ms')
    expect(semanticSceneSummary(editable)).toContain('fade 300ms')
  })
})
