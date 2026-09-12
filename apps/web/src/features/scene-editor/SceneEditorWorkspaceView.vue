<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'

import { ApiError } from '@/api/projects'
import { listScenePlans } from '@/features/scene-plan/api'
import {
  cancelRenderExport,
  createRenderExport,
  createSceneEditorSnapshot,
  duplicateScene,
  getRenderExport,
  getSceneEditor,
  listRenderExportHistory,
  mediaAssetContentURL,
  previewSceneEditorReconcile,
  reconcileSceneEditor,
  removeScene,
  reorderScene,
  retryRenderExport,
  updateSceneEditor,
  type RenderExportJob,
  type RenderSubtitleMode,
  type SceneEditorCandidate,
  type SceneEditorReconcilePreview,
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
import {
  isRenderExportCancellable,
  isRenderExportRetryable,
  isRenderExportTerminal,
  persistRenderJobID,
  RENDER_EXPORT_POLL_MS,
  restoreRenderJobID,
} from './renderExportState'
import type { UpstreamBridgeGuidance } from './upstreamBridgeGuidance'
import {
  buildReconcileCandidate,
  forkApprovedScriptForComposition,
  latestApprovedScenePlanVersion,
  loadUpstreamBridgeGuidance,
} from './upstreamBridge'

const route = useRoute()
const router = useRouter()
const projectID = computed(() => String(route.params.id ?? ''))
const composition = ref<SceneEditorView | null>(null)
const draft = ref<SceneEditorView | null>(null)
const renderJob = ref<RenderExportJob | null>(null)
const renderHistory = ref<RenderExportJob[]>([])
const renderHistoryCursor = ref<string | null>(null)
const renderSubtitleMode = ref<RenderSubtitleMode>('off')
const loading = ref(true)
const acting = ref(false)
const conflict = ref(false)
const error = ref('')
const notice = ref('')
const reconcilePreview = ref<SceneEditorReconcilePreview | null>(null)
const reconcileCandidate = ref<SceneEditorCandidate | null>(null)
const upstreamGuidance = ref<UpstreamBridgeGuidance | null>(null)
const upstreamBusy = ref(false)
let renderPollTimer: ReturnType<typeof setInterval> | null = null

const dirty = computed(() => editorContentSignature(draft.value) !== editorContentSignature(composition.value))
const invalid = computed(() => hasEditorErrors(draft.value))
const renderBusy = computed(() => renderJob.value !== null && !isRenderExportTerminal(renderJob.value))
const snapshotBlocked = computed(() => composition.value?.state !== 'CURRENT' || dirty.value || invalid.value || conflict.value || renderBusy.value)
const renderDownloadURL = computed(() => {
  const assetID = renderJob.value?.state === 'succeeded' ? renderJob.value.artifact?.media_asset_id : undefined
  return assetID ? mediaAssetContentURL(projectID.value, assetID) : ''
})
const subtitleDownloadURL = computed(() => {
  const assetID = renderJob.value?.state === 'succeeded' ? renderJob.value.artifact?.subtitle_media_asset_id : undefined
  return assetID ? mediaAssetContentURL(projectID.value, assetID) : ''
})
function subtitleURLFor(job: RenderExportJob) {
  const assetID = job.state === 'succeeded' ? job.artifact?.subtitle_media_asset_id : undefined
  return assetID ? mediaAssetContentURL(projectID.value, assetID) : ''
}
const saveStatus = computed(() => {
  if (loading.value) return 'Loading'
  if (conflict.value) return 'Conflict — authoritative state changed'
  if (acting.value && dirty.value) return 'Saving…'
  if (dirty.value) return 'Unsaved changes'
  return 'Saved'
})

onMounted(() => {
  void load(true)
  void restoreRenderExport()
  void refreshRenderHistory()
})

onUnmounted(() => {
  stopRenderPolling()
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
    await refreshUpstreamGuidance()
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
    const job = await createRenderExport(projectID.value, snapshot.digest, renderSubtitleMode.value)
    renderJob.value = job
    persistRenderJobID(window.localStorage, projectID.value, job.id)
    notice.value = `Immutable snapshot ${snapshot.digest.slice(0, 12)}… queued for MP4${renderSubtitleMode.value === 'webvtt' ? ' + WebVTT' : ''} render.`
    startRenderPolling()
    void refreshRenderHistory()
  } catch (cause) {
    error.value = messageFor(cause)
  } finally {
    acting.value = false
  }
}

async function restoreRenderExport() {
  if (!projectID.value || typeof window === 'undefined') return
  const jobID = restoreRenderJobID(window.localStorage, projectID.value)
  if (!jobID) return
  await refreshRenderExport(jobID, false)
}

async function refreshRenderExport(jobID = renderJob.value?.id, reportError = true) {
  if (!jobID) return
  try {
    const latest = await getRenderExport(projectID.value, jobID)
    renderJob.value = latest
    persistRenderJobID(window.localStorage, projectID.value, latest.id)
    if (isRenderExportTerminal(latest)) stopRenderPolling()
    else startRenderPolling()
  } catch (cause) {
    if (cause instanceof ApiError && cause.status === 404) {
      renderJob.value = null
      persistRenderJobID(window.localStorage, projectID.value, null)
      stopRenderPolling()
    } else if (reportError) {
      error.value = `Render status refresh failed: ${messageFor(cause)}`
    }
  }
}

function startRenderPolling() {
  if (!renderJob.value || isRenderExportTerminal(renderJob.value) || renderPollTimer !== null) return
  renderPollTimer = setInterval(() => {
    void refreshRenderExport(renderJob.value?.id, false)
  }, RENDER_EXPORT_POLL_MS)
}

function stopRenderPolling() {
  if (renderPollTimer === null) return
  clearInterval(renderPollTimer)
  renderPollTimer = null
}

async function refreshRenderHistory(append = false) {
  if (!projectID.value) return
  try {
    const page = await listRenderExportHistory(projectID.value, 20, append ? renderHistoryCursor.value ?? undefined : undefined)
    renderHistory.value = append ? [...renderHistory.value, ...page.items] : page.items
    renderHistoryCursor.value = page.next_cursor ?? null
  } catch (cause) {
    if (append) error.value = `Render history refresh failed: ${messageFor(cause)}`
  }
}

async function cancelActiveRender() {
  if (!renderJob.value || !isRenderExportCancellable(renderJob.value) || acting.value) return
  acting.value = true
  error.value = ''
  try {
    const cancelled = await cancelRenderExport(projectID.value, renderJob.value.id)
    renderJob.value = cancelled
    persistRenderJobID(window.localStorage, projectID.value, cancelled.id)
    if (isRenderExportTerminal(cancelled)) stopRenderPolling()
    notice.value = cancelled.state === 'cancelled' ? 'Render cancelled.' : 'Cancellation requested.'
    await refreshRenderHistory()
  } catch (cause) {
    error.value = messageFor(cause)
  } finally {
    acting.value = false
  }
}

async function retryTerminalRender(sourceJob: RenderExportJob) {
  if (!isRenderExportRetryable(sourceJob) || acting.value) return
  acting.value = true
  error.value = ''
  try {
    const requestID = crypto.randomUUID()
    const retried = await retryRenderExport(projectID.value, sourceJob.id, requestID)
    renderJob.value = retried
    persistRenderJobID(window.localStorage, projectID.value, retried.id)
    notice.value = `Retry queued from ${sourceJob.id.slice(0, 8)}…`
    startRenderPolling()
    await refreshRenderHistory()
  } catch (cause) {
    error.value = messageFor(cause)
  } finally {
    acting.value = false
  }
}

function selectRenderHistoryItem(job: RenderExportJob) {
  renderJob.value = job
  persistRenderJobID(window.localStorage, projectID.value, job.id)
  if (isRenderExportTerminal(job)) stopRenderPolling()
  else startRenderPolling()
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

async function refreshUpstreamGuidance() {
  if (!composition.value || !projectID.value) {
    upstreamGuidance.value = null
    return
  }
  try {
    upstreamGuidance.value = await loadUpstreamBridgeGuidance(projectID.value, composition.value)
  } catch {
    upstreamGuidance.value = null
  }
}

async function beginUpstreamScriptEdit() {
  if (!composition.value || dirty.value || conflict.value || acting.value || upstreamBusy.value) return
  upstreamBusy.value = true
  error.value = ''
  notice.value = ''
  try {
    const forked = await forkApprovedScriptForComposition(projectID.value, composition.value.scene_plan_version)
    notice.value = `Authoritative script draft v${forked.version} was created from approved history. Continue editing in the Script workspace, then approve and regenerate downstream Scene Plan/narration before reconciling here.`
    await router.push({ name: 'script', params: { id: projectID.value }, query: { version: String(forked.version), returnTo: 'scene-editor' } })
  } catch (cause) {
    error.value = messageFor(cause)
  } finally {
    upstreamBusy.value = false
  }
}

async function previewUpstreamReconcile() {
  if (!composition.value || dirty.value || conflict.value || acting.value) return
  acting.value = true
  error.value = ''
  notice.value = ''
  reconcilePreview.value = null
  reconcileCandidate.value = null
  try {
    const summaries = await listScenePlans(projectID.value)
    const targetVersion = latestApprovedScenePlanVersion(summaries)
    if (targetVersion === null) {
      error.value = 'No approved Scene Plan is available to reconcile against.'
      return
    }
    const candidate = await buildReconcileCandidate(projectID.value, targetVersion)
    reconcileCandidate.value = candidate
    reconcilePreview.value = await previewSceneEditorReconcile(projectID.value, candidate)
    if (reconcilePreview.value.ambiguous) {
      notice.value = 'Reconciliation preview is ambiguous. Resolve upstream scene-key mapping before applying changes.'
    } else {
      notice.value = `Reconciliation preview ready for Scene Plan v${targetVersion}.`
    }
  } catch (cause) {
    error.value = messageFor(cause)
  } finally {
    acting.value = false
  }
}

async function applyUpstreamReconcile() {
  if (!composition.value || !reconcilePreview.value || !reconcileCandidate.value || reconcilePreview.value.ambiguous || acting.value) return
  acting.value = true
  error.value = ''
  notice.value = ''
  try {
    const targetVersion = reconcilePreview.value.to_scene_plan_version
    const reconciled = await reconcileSceneEditor(
      projectID.value,
      composition.value.revision,
      reconcileCandidate.value,
      reconcilePreview.value.preview_digest,
    )
    composition.value = reconciled
    draft.value = cloneEditorView(reconciled)
    reconcilePreview.value = null
    reconcileCandidate.value = null
    conflict.value = false
    notice.value = `Composition reconciled to Scene Plan v${targetVersion} as revision ${reconciled.revision}.`
    await refreshUpstreamGuidance()
  } catch (cause) {
    if (cause instanceof ApiError && cause.status === 409) {
      if (cause.code === 'SCENE_EDITOR_RECONCILE_PREVIEW_STALE') {
        reconcilePreview.value = null
        reconcileCandidate.value = null
        error.value = 'Upstream reconciliation preview is stale. Re-preview before applying.'
      } else {
        conflict.value = true
        error.value = 'Reconciliation conflict. Reload authoritative state and retry.'
        await rereadAfterConflict()
      }
    } else {
      error.value = messageFor(cause)
    }
  } finally {
    acting.value = false
  }
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
        <div v-if="upstreamGuidance && upstreamGuidance.phase !== 'aligned'" class="upstream-guidance" :data-phase="upstreamGuidance.phase" aria-live="polite">
          <p><strong>Upstream bridge:</strong> {{ upstreamGuidance.message }}</p>
          <ul v-if="upstreamGuidance.rebuildNarration || upstreamGuidance.rebuildCaptions || upstreamGuidance.rebuildAudioMix" class="rebuild-checklist">
            <li v-if="upstreamGuidance.rebuildNarration">Rebuild scene narration against the approved Scene Plan.</li>
            <li v-if="upstreamGuidance.rebuildCaptions">Rebuild captions for the new upstream lineage.</li>
            <li v-if="upstreamGuidance.rebuildAudioMix">Rebuild the project audio mix assumptions.</li>
          </ul>
          <div class="guidance-links">
            <RouterLink
              v-if="upstreamGuidance.phase === 'script_draft_pending' || upstreamGuidance.phase === 'script_approved_needs_scene_plan'"
              :to="`/projects/${projectID}/script${upstreamGuidance.pendingScriptDraftVersion ? `?version=${upstreamGuidance.pendingScriptDraftVersion}&returnTo=scene-editor` : '?returnTo=scene-editor'}`"
            >
              Open Script workspace
            </RouterLink>
            <RouterLink
              v-if="upstreamGuidance.phase === 'script_approved_needs_scene_plan' || upstreamGuidance.phase === 'scene_plan_draft_pending_approval' || upstreamGuidance.phase === 'downstream_rebuild_required' || upstreamGuidance.phase === 'ready_to_reconcile'"
              :to="`/projects/${projectID}/scene-plan${upstreamGuidance.pendingScenePlanDraftVersion ? `?version=${upstreamGuidance.pendingScenePlanDraftVersion}&returnTo=scene-editor` : '?returnTo=scene-editor'}`"
            >
              Open Scene Plan workspace
            </RouterLink>
            <RouterLink
              v-if="upstreamGuidance.rebuildNarration"
              :to="`/projects/${projectID}/narration`"
            >
              Open narration workspace
            </RouterLink>
            <RouterLink
              v-if="upstreamGuidance.rebuildCaptions"
              :to="`/projects/${projectID}/captions`"
            >
              Open captions workspace
            </RouterLink>
            <RouterLink
              v-if="upstreamGuidance.rebuildAudioMix"
              :to="`/projects/${projectID}/audio-mix`"
            >
              Open audio mix workspace
            </RouterLink>
          </div>
        </div>
        <div class="upstream-actions">
          <button type="button" :disabled="dirty || conflict || acting || upstreamBusy" @click="beginUpstreamScriptEdit">
            Edit upstream script
          </button>
          <button
            v-if="composition.state === 'STALE'"
            type="button"
            :disabled="dirty || conflict || acting"
            @click="previewUpstreamReconcile"
          >
            Preview upstream reconcile
          </button>
          <button
            v-if="reconcilePreview && !reconcilePreview.ambiguous"
            type="button"
            :disabled="acting"
            @click="applyUpstreamReconcile"
          >
            Apply reconcile
          </button>
        </div>
        <div v-if="reconcilePreview" class="reconcile-preview" aria-live="polite">
          <p>
            Reconcile Scene Plan v{{ reconcilePreview.to_scene_plan_version }} from revision {{ reconcilePreview.from_revision }}.
          </p>
          <ul>
            <li v-for="change in reconcilePreview.changes" :key="change.composition_scene_id">
              <strong>{{ change.scene_key }}</strong>
              <span>{{ change.reasons.join(', ') }}</span>
              <span>{{ change.preserves_edits ? 'Preserves local presentation edits' : 'Requires creator action' }}</span>
            </li>
          </ul>
          <p v-if="reconcilePreview.audio_mix_changed">Project audio mix will be updated.</p>
        </div>
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
          <div class="render-config">
            <label>
              Subtitles
              <select v-model="renderSubtitleMode" :disabled="snapshotBlocked || acting || renderBusy" aria-describedby="subtitle-mode-help">
                <option value="off">Off</option>
                <option value="webvtt">WebVTT</option>
              </select>
            </label>
            <button type="button" :disabled="snapshotBlocked || acting" @click="createSnapshot">Snapshot &amp; render MP4</button>
          </div>
        </div>
        <p id="subtitle-mode-help" class="action-hint">WebVTT uses only caption revisions pinned by this immutable snapshot. The download appears only after the sidecar is durably finalized.</p>
        <ol class="preview-timeline">
          <li v-for="scene in draft.scenes" :key="`preview-${scene.id}`">
            <strong>{{ scene.scene_key }}</strong>
            <span>{{ semanticSceneSummary(scene) }}</span>
          </li>
        </ol>
        <p v-if="draft.audio_mix">Audio mix document {{ draft.audio_mix.document_id }} revision {{ draft.audio_mix.revision }}.</p>
        <p v-else>No project audio mix selected.</p>

        <div v-if="renderJob" class="render-status" aria-live="polite">
          <div>
            <strong>Render {{ renderJob.cancellation_pending ? 'cancelling' : renderJob.state }}</strong>
            <span>Attempt {{ renderJob.attempt }}/{{ renderJob.max_attempts }} · {{ renderJob.profile_id }} · subtitles {{ renderJob.subtitle_mode }}</span>
          </div>
          <p>Snapshot {{ renderJob.snapshot_digest.slice(0, 12) }}…</p>
          <p v-if="renderJob.retry_of_render_job_id">Retry of {{ renderJob.retry_of_render_job_id.slice(0, 8) }}…</p>
          <p v-if="renderJob.error_code" class="field-error" role="alert">Render {{ renderJob.state }}: {{ renderJob.error_code }}</p>
          <p v-if="renderJob.state === 'succeeded' && renderJob.artifact">
            MP4 ready · {{ renderJob.artifact.width }}×{{ renderJob.artifact.height }} · {{ seconds(renderJob.artifact.duration_ms) }} · {{ renderJob.artifact.byte_size }} bytes
          </p>
          <p v-if="renderJob.state === 'succeeded' && renderJob.subtitle_mode === 'webvtt' && !renderJob.artifact?.subtitle_media_asset_id" class="action-hint">
            WebVTT was requested, but this immutable snapshot had no enabled caption sidecar to export.
          </p>
          <div class="render-actions">
            <button v-if="isRenderExportCancellable(renderJob)" type="button" :disabled="acting" @click="cancelActiveRender">Cancel render</button>
            <button v-if="isRenderExportRetryable(renderJob)" type="button" :disabled="acting" @click="retryTerminalRender(renderJob)">Retry render</button>
            <a v-if="renderDownloadURL" :href="renderDownloadURL" download>Download rendered MP4</a>
            <a v-if="subtitleDownloadURL" :href="subtitleDownloadURL" download>Download WebVTT</a>
            <button v-else-if="!renderDownloadURL && !isRenderExportTerminal(renderJob)" type="button" @click="refreshRenderExport(renderJob.id)">Refresh render status</button>
          </div>
        </div>
        <div v-if="renderHistory.length" class="render-history" aria-label="Render history">
          <div class="render-history-header">
            <strong>Render history</strong>
            <button type="button" @click="refreshRenderHistory()">Refresh history</button>
          </div>
          <ol>
            <li v-for="item in renderHistory" :key="item.id">
              <button type="button" class="history-item" :data-state="item.state" @click="selectRenderHistoryItem(item)">
                <span>{{ item.state }} · subtitles {{ item.subtitle_mode }}</span>
                <span>{{ item.snapshot_digest.slice(0, 8) }}…</span>
                <span>{{ item.created_at }}</span>
              </button>
              <div class="history-actions">
                <a v-if="subtitleURLFor(item)" :href="subtitleURLFor(item)" download>Download WebVTT</a>
                <button v-if="isRenderExportRetryable(item)" type="button" :disabled="acting" @click="retryTerminalRender(item)">Retry</button>
              </div>
            </li>
          </ol>
          <button v-if="renderHistoryCursor" type="button" @click="refreshRenderHistory(true)">Load more history</button>
        </div>
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
.page-header, .section-heading, .scene-card header, .status-panel > div, .render-status > div { display: flex; gap: 1rem; justify-content: space-between; align-items: flex-start; }
.eyebrow, .scene-index { text-transform: uppercase; letter-spacing: .08em; font-size: .75rem; font-weight: 700; }
.notice, .status-panel, .scene-card, .preview-panel { border: 1px solid currentColor; border-radius: .75rem; padding: 1rem; }
.notice.error, .status-panel[data-state='BROKEN'] { border-width: 2px; }
.status-panel, .preview-panel, .render-status { display: grid; gap: .75rem; }
.render-status { border-top: 1px solid currentColor; padding-top: .9rem; }
.render-status p { margin: 0; overflow-wrap: anywhere; }
.render-actions, .render-history-header, .render-config, .history-actions { display: flex; gap: .75rem; flex-wrap: wrap; align-items: center; }
.render-config label { display: grid; gap: .25rem; font-weight: 600; }
.render-config select { min-height: 2.75rem; padding: .45rem .6rem; }
.render-history { display: grid; gap: .75rem; border-top: 1px solid currentColor; padding-top: .9rem; }
.render-history ol { list-style: none; margin: 0; padding: 0; display: grid; gap: .5rem; }
.render-history li { display: flex; gap: .5rem; align-items: center; justify-content: space-between; }
.history-item { display: grid; gap: .15rem; text-align: left; }
.status-panel strong { margin-right: .75rem; }
.save-status[data-dirty='true'] { text-decoration: underline; text-decoration-thickness: 2px; }
.upstream-guidance { display: grid; gap: .5rem; border-top: 1px solid currentColor; padding-top: .75rem; }
.upstream-guidance .rebuild-checklist { margin: 0; padding-left: 1.25rem; }
.guidance-links { display: flex; gap: .75rem; flex-wrap: wrap; }
.save-actions, .scene-actions, .upstream-actions { display: flex; gap: .5rem; flex-wrap: wrap; }
.reconcile-preview { display: grid; gap: .5rem; border-top: 1px solid currentColor; padding-top: .75rem; }
.reconcile-preview ul { margin: 0; padding-left: 1.25rem; display: grid; gap: .35rem; }
.reconcile-preview li { display: grid; gap: .15rem; }
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
@media (max-width: 640px) { .page-header, .section-heading, .scene-card header, .status-panel > div, .render-status > div { flex-direction: column; } }
</style>