<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink, useRoute } from 'vue-router'

import { ApiError } from '@/api/projects'
import {
  createSceneEditorSnapshot,
  duplicateScene,
  getSceneEditor,
  removeScene,
  reorderScene,
  updateSceneEditor,
  type SceneEditorScene,
  type SceneEditorView,
} from './api'
import {
  cloneEditorView,
  editorContentSignature,
  hasEditorErrors,
  normalizeTransitionForKind,
  semanticSceneSummary,
  validateEditableScene,
} from './editorState'

const route = useRoute()
const projectID = computed(() => String(route.params.id ?? ''))
const composition = ref<SceneEditorView | null>(null)
const draft = ref<SceneEditorView | null>(null)
const loading = ref(true)
const acting = ref(false)
const conflict = ref(false)
const error = ref('')
const notice = ref('')

const dirty = computed(() => editorContentSignature(draft.value) !== editorContentSignature(composition.value))
const invalid = computed(() => hasEditorErrors(draft.value))
const snapshotBlocked = computed(() => composition.value?.state !== 'CURRENT' || dirty.value || invalid.value || conflict.value)
const saveStatus = computed(() => {
  if (loading.value) return 'Loading'
  if (conflict.value) return 'Conflict — authoritative state changed'
  if (acting.value && dirty.value) return 'Saving…'
  if (dirty.value) return 'Unsaved changes'
  return 'Saved'
})

onMounted(() => {
  void load(true)
})

async function load(resetDraft: boolean) {
  if (!projectID.value) return
  loading.value = true
  error.value = ''
  try {
    const latest = await getSceneEditor(projectID.value)
    composition.value = latest
    if (resetDraft || !draft.value) draft.value = cloneEditorView(latest)
    conflict.value = false
  } catch (cause) {
    if (cause instanceof ApiError && cause.status === 404) {
      composition.value = null
      draft.value = null
      notice.value = 'Scene composition has not been initialized from authoritative project sources yet.'
    } else {
      error.value = messageFor(cause)
    }
  } finally {
    loading.value = false
  }
}

async function saveDraft() {
  if (!composition.value || !draft.value || !dirty.value || invalid.value || acting.value || conflict.value) return
  acting.value = true
  error.value = ''
  notice.value = ''
  try {
    const saved = await updateSceneEditor(projectID.value, {
      expected_revision: composition.value.revision,
      scenes: draft.value.scenes,
      audio_mix: draft.value.audio_mix,
    })
    composition.value = saved
    draft.value = cloneEditorView(saved)
    notice.value = `Scene composition saved as revision ${saved.revision}.`
  } catch (cause) {
    if (cause instanceof ApiError && cause.status === 409) {
      conflict.value = true
      error.value = 'Revision conflict. The authoritative composition changed; local draft is preserved until you explicitly reload.'
      await rereadAfterConflict()
    } else {
      error.value = messageFor(cause)
    }
  } finally {
    acting.value = false
  }
}

async function rereadAfterConflict() {
  try {
    composition.value = await getSceneEditor(projectID.value)
    conflict.value = true
  } catch (cause) {
    error.value = `Conflict recovery could not reload authoritative state: ${messageFor(cause)}`
  }
}

function resetToSaved() {
  if (!composition.value || acting.value) return
  draft.value = cloneEditorView(composition.value)
  conflict.value = false
  error.value = ''
  notice.value = 'Local draft reset to the authoritative saved revision.'
}

function transitionKindChanged(scene: SceneEditorScene) {
  normalizeTransitionForKind(scene)
}

function sceneErrors(scene: SceneEditorScene, index: number) {
  return validateEditableScene(scene, draft.value?.scenes[index + 1])
}

async function move(scene: SceneEditorScene, delta: -1 | 1) {
  if (!composition.value || dirty.value || conflict.value || acting.value) return
  const from = composition.value.scenes.findIndex((item) => item.id === scene.id)
  const to = from + delta
  if (from < 0 || to < 0 || to >= composition.value.scenes.length) return
  await act(() => reorderScene(projectID.value, scene.id, composition.value!.revision, to), 'Scene order saved.')
}

async function duplicate(scene: SceneEditorScene) {
  if (!composition.value || dirty.value || conflict.value || acting.value) return
  await act(() => duplicateScene(projectID.value, scene.id, composition.value!.revision), 'Scene duplicated.')
}

async function remove(scene: SceneEditorScene) {
  if (!composition.value || dirty.value || conflict.value || acting.value || composition.value.scenes.length <= 1) return
  await act(() => removeScene(projectID.value, scene.id, composition.value!.revision), 'Scene removed.')
}

async function createSnapshot() {
  if (!composition.value || snapshotBlocked.value || acting.value) return
  acting.value = true
  error.value = ''
  notice.value = ''
  try {
    const snapshot = await createSceneEditorSnapshot(projectID.value, composition.value.revision)
    notice.value = `Immutable render snapshot ready: ${snapshot.digest.slice(0, 12)}… (revision ${snapshot.revision}).`
  } catch (cause) {
    error.value = messageFor(cause)
  } finally {
    acting.value = false
  }
}

async function act(operation: () => Promise<SceneEditorView>, success: string) {
  acting.value = true
  error.value = ''
  notice.value = ''
  try {
    const saved = await operation()
    composition.value = saved
    draft.value = cloneEditorView(saved)
    conflict.value = false
    notice.value = success
  } catch (cause) {
    error.value = messageFor(cause)
    if (cause instanceof ApiError && cause.status === 409) {
      conflict.value = true
      await rereadAfterConflict()
    }
  } finally {
    acting.value = false
  }
}

function messageFor(cause: unknown): string {
  if (cause instanceof ApiError) return `${cause.code}: ${cause.message}`
  return cause instanceof Error ? cause.message : 'Scene editor request failed.'
}

function seconds(ms: number): string {
  return `${(ms / 1000).toFixed(ms % 1000 === 0 ? 0 : 1)}s`
}
</script>

<template>
  <main class="scene-editor-page">
    <header class="page-header">
      <div>
        <p class="eyebrow">Creator composition</p>
        <h1>Scene Editor</h1>
        <p>Edit bounded composition semantics and preview exactly what the immutable render snapshot will consume.</p>
      </div>
      <RouterLink :to="`/projects/${projectID}`">Back to project</RouterLink>
    </header>

    <p v-if="error" class="notice error" role="alert">{{ error }}</p>
    <p v-if="notice" class="notice" role="status">{{ notice }}</p>
    <p v-if="loading" role="status">Loading scene composition…</p>

    <template v-else-if="composition && draft">
      <section class="status-panel" :data-state="composition.state" aria-live="polite">
        <div>
          <div>
            <strong>{{ composition.state }}</strong>
            <span>Revision {{ composition.revision }} · Scene plan v{{ composition.scene_plan_version }}</span>
          </div>
          <strong class="save-status" :data-dirty="dirty || conflict">{{ saveStatus }}</strong>
        </div>
        <p v-if="composition.state === 'STALE'">Upstream source lineage changed. Saved creator edits are preserved; reconcile before rendering.</p>
        <p v-else-if="composition.state === 'BROKEN'">One or more exact upstream dependencies are unavailable. Rendering is blocked.</p>
        <p v-else-if="dirty">The preview below reflects local edits that have not been persisted yet.</p>
        <p v-else>All tracked dependencies and creator edits match this saved composition revision.</p>
        <div class="save-actions">
          <button type="button" :disabled="!dirty || invalid || acting || conflict" @click="saveDraft">Save composition</button>
          <button type="button" :disabled="(!dirty && !conflict) || acting" @click="resetToSaved">Reload saved revision</button>
        </div>
        <p v-if="invalid" class="validation-summary" role="alert">Fix the highlighted composition fields before saving or snapshotting.</p>
      </section>

      <section class="preview-panel" aria-labelledby="preview-heading">
        <div class="section-heading">
          <div>
            <p class="eyebrow">Snapshot-equivalent semantics</p>
            <h2 id="preview-heading">Composition preview</h2>
          </div>
          <button type="button" :disabled="snapshotBlocked || acting" @click="createSnapshot">Create render snapshot</button>
        </div>
        <ol class="preview-timeline">
          <li v-for="scene in draft.scenes" :key="`preview-${scene.id}`">
            <strong>{{ scene.scene_key }}</strong>
            <span>{{ semanticSceneSummary(scene) }}</span>
          </li>
        </ol>
        <p v-if="draft.audio_mix">Audio mix document {{ draft.audio_mix.document_id }} revision {{ draft.audio_mix.revision }}.</p>
        <p v-else>No project audio mix selected.</p>
      </section>

      <section aria-labelledby="scene-list-heading">
        <div class="section-heading">
          <div>
            <h2 id="scene-list-heading">Scenes</h2>
            <p>Every editable control uses the same frozen bounds validated by the API contract.</p>
          </div>
        </div>

        <ol class="scene-list">
          <li v-for="(scene, index) in draft.scenes" :key="scene.id" class="scene-card">
            <header>
              <div>
                <span class="scene-index">Scene {{ index + 1 }}</span>
                <h3>{{ scene.scene_key }}</h3>
              </div>
              <span>{{ seconds(scene.duration_ms) }}</span>
            </header>

            <fieldset class="editor-grid" :disabled="acting || conflict">
              <legend>Timing and visual treatment</legend>
              <label>
                Duration (ms)
                <input v-model.number="scene.duration_ms" type="number" min="1" step="1" :aria-describedby="sceneErrors(scene, index).duration_ms ? `duration-error-${scene.id}` : undefined">
                <span v-if="sceneErrors(scene, index).duration_ms" :id="`duration-error-${scene.id}`" class="field-error" role="alert">{{ sceneErrors(scene, index).duration_ms }}</span>
              </label>
              <label>
                Fit
                <select v-model="scene.visual_treatment.fit">
                  <option value="contain">Contain</option>
                  <option value="cover">Cover</option>
                </select>
              </label>
              <label>
                Position X
                <input v-model.number="scene.visual_treatment.position_x" type="number" min="-1" max="1" step="0.05">
                <span v-if="sceneErrors(scene, index).position_x" class="field-error" role="alert">{{ sceneErrors(scene, index).position_x }}</span>
              </label>
              <label>
                Position Y
                <input v-model.number="scene.visual_treatment.position_y" type="number" min="-1" max="1" step="0.05">
                <span v-if="sceneErrors(scene, index).position_y" class="field-error" role="alert">{{ sceneErrors(scene, index).position_y }}</span>
              </label>
              <label>
                Scale
                <input v-model.number="scene.visual_treatment.scale" type="number" min="0.25" max="4" step="0.05">
                <span v-if="sceneErrors(scene, index).scale" class="field-error" role="alert">{{ sceneErrors(scene, index).scale }}</span>
              </label>
              <label class="checkbox-label">
                <input v-model="scene.visual_treatment.mute_video" type="checkbox">
                Mute source-video audio
              </label>
              <label>
                Transition
                <select v-model="scene.transition_out.kind" @change="transitionKindChanged(scene)">
                  <option value="cut">Cut</option>
                  <option value="fade">Fade</option>
                  <option value="crossfade">Crossfade</option>
                </select>
              </label>
              <label>
                Transition duration (ms)
                <input v-model.number="scene.transition_out.duration_ms" type="number" :disabled="scene.transition_out.kind === 'cut'" :min="scene.transition_out.kind === 'cut' ? 0 : 100" :max="scene.transition_out.kind === 'cut' ? 0 : 2000" step="1">
                <span v-if="sceneErrors(scene, index).transition_duration_ms" class="field-error" role="alert">{{ sceneErrors(scene, index).transition_duration_ms }}</span>
              </label>
            </fieldset>

            <dl class="lineage-grid">
              <div>
                <dt>Visual</dt>
                <dd>{{ scene.visual ? scene.visual.asset_id : 'No visual bound' }}</dd>
              </div>
              <div>
                <dt>Narration</dt>
                <dd>{{ scene.narration ? `${scene.narration.lineage_id} · ${scene.narration.duration_ms}ms` : 'No narration bound' }}</dd>
              </div>
              <div>
                <dt>Captions</dt>
                <dd>{{ scene.caption ? `${scene.caption.document_id} r${scene.caption.revision} · through ${scene.caption.last_end_ms}ms` : 'No captions bound' }}</dd>
              </div>
              <div>
                <dt>Transition</dt>
                <dd>{{ scene.transition_out.kind }} · {{ scene.transition_out.duration_ms }}ms</dd>
              </div>
            </dl>

            <div class="scene-actions" :aria-label="`Actions for ${scene.scene_key}`">
              <button type="button" :disabled="acting || dirty || conflict || index === 0" @click="move(scene, -1)">Move up</button>
              <button type="button" :disabled="acting || dirty || conflict || index === draft.scenes.length - 1" @click="move(scene, 1)">Move down</button>
              <button type="button" :disabled="acting || dirty || conflict" @click="duplicate(scene)">Duplicate</button>
              <button type="button" :disabled="acting || dirty || conflict || draft.scenes.length <= 1" @click="remove(scene)">Remove</button>
              <span v-if="dirty" class="action-hint">Save or reload local edits before structural actions.</span>
            </div>
          </li>
        </ol>
      </section>
    </template>
  </main>
</template>

<style scoped>
.scene-editor-page { display: grid; gap: 1.5rem; max-width: 1100px; margin: 0 auto; padding: 2rem 1rem 4rem; }
.page-header, .section-heading, .scene-card header, .status-panel > div { display: flex; gap: 1rem; justify-content: space-between; align-items: flex-start; }
.eyebrow, .scene-index { text-transform: uppercase; letter-spacing: .08em; font-size: .75rem; font-weight: 700; }
.notice, .status-panel, .scene-card, .preview-panel { border: 1px solid currentColor; border-radius: .75rem; padding: 1rem; }
.notice.error, .status-panel[data-state='BROKEN'] { border-width: 2px; }
.status-panel, .preview-panel { display: grid; gap: .75rem; }
.status-panel strong { margin-right: .75rem; }
.save-status[data-dirty='true'] { text-decoration: underline; text-decoration-thickness: 2px; }
.save-actions, .scene-actions { display: flex; gap: .5rem; flex-wrap: wrap; }
.validation-summary, .field-error { font-weight: 700; }
.preview-timeline { display: grid; gap: .5rem; margin: 0; padding-left: 1.25rem; }
.preview-timeline li { display: grid; gap: .2rem; }
.preview-timeline span { overflow-wrap: anywhere; }
.scene-list { display: grid; gap: 1rem; padding: 0; list-style: none; }
.scene-card { display: grid; gap: 1rem; }
.scene-card h3 { margin: .2rem 0 0; }
.editor-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: .9rem; border: 0; padding: 0; margin: 0; }
.editor-grid legend { grid-column: 1 / -1; font-weight: 700; margin-bottom: .25rem; }
.editor-grid label { display: grid; align-content: start; gap: .35rem; font-weight: 600; }
.editor-grid input, .editor-grid select { min-height: 2.5rem; width: 100%; padding: .4rem .5rem; }
.editor-grid .checkbox-label { grid-template-columns: auto 1fr; align-items: center; align-self: end; min-height: 2.5rem; }
.editor-grid .checkbox-label input { width: auto; min-height: 1.25rem; }
.field-error { font-size: .8rem; }
.lineage-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(190px, 1fr)); gap: .75rem; margin: 0; }
.lineage-grid div { min-width: 0; }
.lineage-grid dt { font-weight: 700; }
.lineage-grid dd { margin: .25rem 0 0; overflow-wrap: anywhere; }
.action-hint { align-self: center; font-size: .85rem; }
button { min-height: 2.75rem; padding: .55rem .8rem; }
button:focus-visible, a:focus-visible, input:focus-visible, select:focus-visible { outline: 3px solid currentColor; outline-offset: 3px; }
@media (max-width: 640px) { .page-header, .section-heading, .scene-card header, .status-panel > div { flex-direction: column; } }
</style>
