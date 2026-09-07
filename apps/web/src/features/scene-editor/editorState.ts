import type { SceneEditorScene, SceneEditorView } from './api'

export interface SceneEditorFieldErrors {
  [field: string]: string
}

export function cloneEditorView(view: SceneEditorView): SceneEditorView {
  return {
    ...view,
    scenes: view.scenes.map((scene) => ({
      ...scene,
      visual: scene.visual ? { ...scene.visual } : undefined,
      narration: scene.narration ? { ...scene.narration } : undefined,
      caption: scene.caption ? { ...scene.caption } : undefined,
      visual_treatment: {
        ...scene.visual_treatment,
        crop: scene.visual_treatment.crop ? { ...scene.visual_treatment.crop } : undefined,
      },
      transition_out: { ...scene.transition_out },
    })),
    audio_mix: view.audio_mix ? { ...view.audio_mix } : undefined,
  }
}

export function editorContentSignature(view: SceneEditorView | null): string {
  if (!view) return ''
  return JSON.stringify({ scenes: view.scenes, audio_mix: view.audio_mix ?? null })
}

export function validateEditableScene(scene: SceneEditorScene, nextScene?: SceneEditorScene): SceneEditorFieldErrors {
  const errors: SceneEditorFieldErrors = {}
  if (!Number.isInteger(scene.duration_ms) || scene.duration_ms <= 0) {
    errors.duration_ms = 'Duration must be a positive whole number of milliseconds.'
  }
  if (scene.narration && scene.duration_ms < scene.narration.duration_ms) {
    errors.duration_ms = 'Duration cannot truncate the selected narration.'
  }
  if (scene.caption && scene.duration_ms < scene.caption.last_end_ms) {
    errors.duration_ms = 'Duration must include the final selected caption cue.'
  }

  const treatment = scene.visual_treatment
  if (treatment.position_x < -1 || treatment.position_x > 1) errors.position_x = 'Position X must be between -1 and 1.'
  if (treatment.position_y < -1 || treatment.position_y > 1) errors.position_y = 'Position Y must be between -1 and 1.'
  if (treatment.scale < 0.25 || treatment.scale > 4) errors.scale = 'Scale must be between 0.25 and 4.'

  const transition = scene.transition_out
  if (transition.kind === 'cut') {
    if (transition.duration_ms !== 0) errors.transition_duration_ms = 'Cut duration must be 0 ms.'
  } else {
    if (transition.duration_ms < 100 || transition.duration_ms > 2_000) {
      errors.transition_duration_ms = 'Fade and crossfade duration must be between 100 and 2000 ms.'
    } else if (transition.duration_ms > scene.duration_ms || (nextScene && transition.duration_ms > nextScene.duration_ms)) {
      errors.transition_duration_ms = 'Transition duration must fit both adjacent scenes.'
    }
  }
  return errors
}

export function hasEditorErrors(view: SceneEditorView | null): boolean {
  if (!view) return false
  return view.scenes.some((scene, index) => Object.keys(validateEditableScene(scene, view.scenes[index + 1])).length > 0)
}

export function normalizeTransitionForKind(scene: SceneEditorScene): void {
  if (scene.transition_out.kind === 'cut') {
    scene.transition_out.duration_ms = 0
    return
  }
  if (scene.transition_out.duration_ms < 100 || scene.transition_out.duration_ms > 2_000) {
    scene.transition_out.duration_ms = 300
  }
}

export function semanticSceneSummary(scene: SceneEditorScene): string {
  const visual = scene.visual
    ? `${scene.visual_treatment.fit}, x ${scene.visual_treatment.position_x}, y ${scene.visual_treatment.position_y}, scale ${scene.visual_treatment.scale}`
    : 'no visual'
  const caption = scene.caption ? `captions through ${scene.caption.last_end_ms}ms` : 'no captions'
  const narration = scene.narration ? `narration ${scene.narration.duration_ms}ms` : 'no narration'
  const transition = `${scene.transition_out.kind} ${scene.transition_out.duration_ms}ms`
  return `${scene.scene_key}: ${scene.duration_ms}ms; ${visual}; ${narration}; ${caption}; ${transition}`
}
