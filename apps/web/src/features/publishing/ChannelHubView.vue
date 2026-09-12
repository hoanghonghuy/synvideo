<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'

import { ApiError } from '@/api/projects'
import {
  createPublishAttempt,
  getPublishAttempt,
  listPublishArtifacts,
  listPublishAttempts,
  listPublishingConnections,
  reconcilePublishAttempt,
  retryPublishAttempt,
  startYouTubeOAuth,
  type ChannelConnection,
  type PublishArtifactSummary,
  type PublishAttempt,
  type PublishState,
} from '@/api/publishing'

const LIVE_PROGRESS_REFRESH_MS = 4000

const route = useRoute()
const projectID = computed(() => String(route.params.id ?? ''))

const connections = ref<ChannelConnection[]>([])
const artifacts = ref<PublishArtifactSummary[]>([])
const attempts = ref<PublishAttempt[]>([])
const selectedConnectionID = ref('')
const renderArtifactID = ref('')
const title = ref('')
const description = ref('')
const loading = ref(true)
const submitting = ref(false)
const refreshing = ref(false)
const liveRefreshing = ref(false)
const retrying = ref(false)
const reconciling = ref(false)
const connecting = ref(false)
const errorMessage = ref('')
const successMessage = ref('')
const liveRefreshMessage = ref('')
const attempt = ref<PublishAttempt | null>(null)
let liveRefreshTimer: ReturnType<typeof setTimeout> | null = null
let liveRefreshRequest = 0
let disposed = false

const selectedConnection = computed(() => connections.value.find((item) => item.id === selectedConnectionID.value) ?? null)
const selectedAttemptArtifact = computed(() => {
  if (!attempt.value) return null
  return artifacts.value.find((item) => item.id === attempt.value?.render_artifact_id) ?? null
})
const uploadPercent = computed(() => {
  const current = attempt.value
  const artifact = selectedAttemptArtifact.value
  if (!current || !artifact || artifact.byte_size <= 0) return null
  return Math.max(0, Math.min(100, Math.round((current.uploaded_bytes / artifact.byte_size) * 100)))
})
const canReconcile = computed(() => {
  const current = attempt.value
  if (!current?.remote_video_id) return false
  return ['upload_accepted', 'processing', 'private', 'scheduled', 'public'].includes(current.state)
})
const canSubmit = computed(() => {
  const connection = selectedConnection.value
  return Boolean(
    connection?.state === 'connected'
      && connection.capabilities.can_upload
      && renderArtifactID.value
      && title.value.trim()
      && !submitting.value,
  )
})

watch(
  () => [attempt.value?.id, attempt.value?.state],
  () => {
    liveRefreshMessage.value = ''
    scheduleLiveProgressRefresh()
  },
)

onMounted(() => {
  if (route.query.youtube === 'connected') {
    successMessage.value = 'YouTube authorization completed. Channel capabilities have been refreshed.'
  }
  void loadWorkspace()
})

onUnmounted(() => {
  disposed = true
  liveRefreshRequest += 1
  clearLiveProgressRefresh()
})

async function loadWorkspace() {
  loading.value = true
  errorMessage.value = ''
  try {
    const [connectionItems, artifactItems, historyItems] = await Promise.all([
      listPublishingConnections(),
      listPublishArtifacts(projectID.value),
      listPublishAttempts(projectID.value),
    ])
    connections.value = connectionItems
    artifacts.value = artifactItems
    attempts.value = historyItems
    const firstUsable = connections.value.find((item) => item.state === 'connected' && item.capabilities.can_upload)
    if (!connections.value.some((item) => item.id === selectedConnectionID.value)) {
      selectedConnectionID.value = firstUsable?.id ?? connections.value[0]?.id ?? ''
    }
    if (!artifacts.value.some((item) => item.id === renderArtifactID.value)) {
      renderArtifactID.value = artifacts.value[0]?.id ?? ''
    }
    if (attempt.value) {
      attempt.value = attempts.value.find((item) => item.id === attempt.value?.id) ?? attempt.value
    }
  } catch (error) {
    errorMessage.value = error instanceof ApiError ? error.message : 'Could not load the Channel Hub workspace.'
  } finally {
    loading.value = false
  }
}

async function connectYouTube() {
  if (connecting.value) return
  connecting.value = true
  errorMessage.value = ''
  successMessage.value = ''
  try {
    const authorizationURL = await startYouTubeOAuth(projectID.value)
    window.location.assign(authorizationURL)
  } catch (error) {
    connecting.value = false
    errorMessage.value = error instanceof ApiError ? error.message : 'Could not start YouTube authorization.'
  }
}

async function submit() {
  if (!canSubmit.value) return
  submitting.value = true
  errorMessage.value = ''
  try {
    const created = await createPublishAttempt(projectID.value, {
      connection_id: selectedConnectionID.value,
      render_artifact_id: renderArtifactID.value,
      request_id: crypto.randomUUID(),
      title: title.value.trim(),
      description: description.value.trim(),
    })
    attempt.value = created
    attempts.value = [created, ...attempts.value.filter((item) => item.id !== created.id)]
  } catch (error) {
    errorMessage.value = error instanceof ApiError ? error.message : 'Could not create the publish attempt.'
  } finally {
    submitting.value = false
  }
}

async function refreshAttempt(item: PublishAttempt | null = attempt.value) {
  if (!item || liveRefreshing.value) return
  refreshing.value = true
  errorMessage.value = ''
  try {
    const refreshed = await getPublishAttempt(projectID.value, item.id)
    if (attempt.value?.id === item.id) {
      liveRefreshMessage.value = ''
      updateAttempt(refreshed)
      scheduleLiveProgressRefresh()
    }
  } catch (error) {
    errorMessage.value = error instanceof ApiError ? error.message : 'Could not refresh the persisted publish attempt.'
  } finally {
    refreshing.value = false
  }
}

async function refreshLiveProgress(attemptID: string) {
  if (
    disposed
    || liveRefreshing.value
    || refreshing.value
    || attempt.value?.id !== attemptID
    || !isLiveProgressState(attempt.value.state)
  ) {
    scheduleLiveProgressRefresh()
    return
  }

  liveRefreshing.value = true
  const request = ++liveRefreshRequest
  try {
    const refreshed = await getPublishAttempt(projectID.value, attemptID)
    if (!disposed && request === liveRefreshRequest && attempt.value?.id === attemptID) {
      liveRefreshMessage.value = ''
      updateAttempt(refreshed)
    }
  } catch {
    if (!disposed && request === liveRefreshRequest && attempt.value?.id === attemptID) {
      liveRefreshMessage.value = 'Live progress refresh paused. The last saved upload state is still shown; use Refresh saved state to retry.'
    }
  } finally {
    if (request === liveRefreshRequest) {
      liveRefreshing.value = false
    }
    if (!liveRefreshMessage.value) {
      scheduleLiveProgressRefresh()
    }
  }
}

function scheduleLiveProgressRefresh() {
  clearLiveProgressRefresh()
  const current = attempt.value
  if (disposed || !current || !isLiveProgressState(current.state) || liveRefreshMessage.value) return
  const attemptID = current.id
  liveRefreshTimer = setTimeout(() => {
    liveRefreshTimer = null
    void refreshLiveProgress(attemptID)
  }, LIVE_PROGRESS_REFRESH_MS)
}

function clearLiveProgressRefresh() {
  if (liveRefreshTimer !== null) {
    clearTimeout(liveRefreshTimer)
    liveRefreshTimer = null
  }
}

function isLiveProgressState(state: PublishState): boolean {
  return state === 'queued' || state === 'uploading'
}

async function reconcileAttempt() {
  if (!attempt.value || !canReconcile.value || reconciling.value) return
  reconciling.value = true
  errorMessage.value = ''
  successMessage.value = ''
  try {
    const reconciled = await reconcilePublishAttempt(projectID.value, attempt.value.id)
    updateAttempt(reconciled)
    if (reconciled.last_error_code === 'youtube_status_retryable') {
      errorMessage.value = 'YouTube status is temporarily unavailable. The remote video is preserved; retry this status check later.'
    } else {
      successMessage.value = 'YouTube processing and publication status refreshed.'
    }
  } catch (error) {
    errorMessage.value = error instanceof ApiError ? error.message : 'Could not check the current YouTube status.'
  } finally {
    reconciling.value = false
  }
}

async function retryAttempt() {
  if (!attempt.value || attempt.value.state !== 'retryable_failure' || retrying.value) return
  retrying.value = true
  errorMessage.value = ''
  try {
    const retried = await retryPublishAttempt(projectID.value, attempt.value.id)
    updateAttempt(retried)
  } catch (error) {
    errorMessage.value = error instanceof ApiError ? error.message : 'Could not queue the publish retry.'
  } finally {
    retrying.value = false
  }
}

function updateAttempt(updated: PublishAttempt) {
  attempt.value = updated
  attempts.value = attempts.value.map((entry) => entry.id === updated.id ? updated : entry)
}

function inspectAttempt(item: PublishAttempt) {
  attempt.value = item
}

function connectionTone(connection: ChannelConnection): string {
  return connection.state === 'connected' ? 'good' : connection.state === 'reconnect_required' ? 'warn' : 'bad'
}

function attemptTone(item: PublishAttempt): string {
  return item.state === 'rejected' ? 'bad' : item.state.includes('failure') || item.state === 'reconnect_required' ? 'warn' : 'good'
}

function stateLabel(state: string): string {
  return state.split('_').join(' ')
}

function formatBytes(bytes: number): string {
  if (bytes < 1024 * 1024) return `${Math.max(1, Math.round(bytes / 1024)).toLocaleString()} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

function formatDuration(durationMS: number): string {
  const totalSeconds = Math.round(durationMS / 1000)
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60
  return `${minutes}:${seconds.toString().padStart(2, '0')}`
}
</script>

<template>
  <section class="channel-hub page">
    <div class="hub-heading">
      <div>
        <RouterLink class="text-link" :to="`/projects/${projectID}`">Back to project</RouterLink>
        <p class="eyebrow">Publishing</p>
        <h1>Channel Hub</h1>
        <p class="body-copy">Publish an immutable rendered video to YouTube with durable progress, retry, reconnect and post-upload status recovery.</p>
      </div>
      <button class="secondary-button" type="button" :disabled="loading" @click="loadWorkspace">
        {{ loading ? 'Refreshing…' : 'Refresh workspace' }}
      </button>
    </div>

    <div v-if="successMessage" class="notice success" role="status">{{ successMessage }}</div>
    <div v-if="errorMessage" class="notice error" role="alert">{{ errorMessage }}</div>

    <div class="hub-grid">
      <section class="panel" aria-labelledby="connections-title">
        <div class="panel-heading">
          <div>
            <p class="eyebrow">Destination</p>
            <h2 id="connections-title">YouTube connection</h2>
          </div>
          <span class="count-badge">{{ connections.length }}</span>
        </div>

        <p v-if="loading" class="state-text">Loading connected channels…</p>
        <div v-else-if="connections.length === 0" class="empty-state">
          <strong>No channel connected</strong>
          <p>Authorize YouTube to create the first server-managed channel connection. OAuth credentials and refresh tokens never enter the browser.</p>
          <button class="primary-button" type="button" :disabled="connecting" @click="connectYouTube">
            {{ connecting ? 'Opening YouTube…' : 'Connect YouTube' }}
          </button>
        </div>
        <div v-else class="connection-list">
          <label v-for="connection in connections" :key="connection.id" class="connection-card" :class="{ selected: selectedConnectionID === connection.id }">
            <input v-model="selectedConnectionID" type="radio" name="publishing-connection" :value="connection.id">
            <span class="connection-copy">
              <span class="connection-title">{{ connection.display_name }}</span>
              <span class="connection-meta">YouTube · {{ connection.remote_channel_id }}</span>
              <span class="status-pill" :class="connectionTone(connection)">{{ stateLabel(connection.state) }}</span>
            </span>
          </label>
        </div>

        <div v-if="selectedConnection" class="capability-box">
          <strong>Channel capabilities</strong>
          <ul>
            <li>Upload: {{ selectedConnection.capabilities.can_upload ? 'available' : 'unavailable' }}</li>
            <li>Public publish: {{ selectedConnection.capabilities.can_publish ? 'available' : 'unavailable' }}</li>
            <li>Schedule: {{ selectedConnection.capabilities.can_schedule ? 'available' : 'unavailable' }}</li>
          </ul>
          <div v-if="selectedConnection.state === 'reconnect_required' || selectedConnection.state === 'revoked'" class="recovery-box">
            <p class="warning-copy">Authorization is no longer usable. Reconnect the same remote channel before upload can continue.</p>
            <button class="primary-button" type="button" :disabled="connecting" @click="connectYouTube">
              {{ connecting ? 'Opening YouTube…' : 'Reconnect YouTube' }}
            </button>
          </div>
          <p v-else-if="!selectedConnection.capabilities.can_publish" class="warning-copy">This connection cannot promise public publication. Controls remain limited to server-reported capability.</p>
        </div>
      </section>

      <section class="panel" aria-labelledby="publish-title">
        <p class="eyebrow">New upload</p>
        <h2 id="publish-title">Publish rendered video</h2>
        <form class="publish-form" @submit.prevent="submit">
          <label>
            <span>Rendered video</span>
            <select v-model="renderArtifactID" required :disabled="loading || artifacts.length === 0">
              <option value="" disabled>{{ artifacts.length === 0 ? 'No successful renders available' : 'Select a render' }}</option>
              <option v-for="artifact in artifacts" :key="artifact.id" :value="artifact.id">
                {{ artifact.width }}×{{ artifact.height }} · {{ formatDuration(artifact.duration_ms) }} · {{ formatBytes(artifact.byte_size) }} · {{ new Date(artifact.created_at).toLocaleString() }}
              </option>
            </select>
            <small v-if="artifacts.length">Only owned immutable MP4 render artifacts for this project are listed.</small>
            <small v-else>Create a successful render before publishing.</small>
          </label>
          <label>
            <span>Video title</span>
            <input v-model="title" required maxlength="100" autocomplete="off" placeholder="Title shown on YouTube">
          </label>
          <label>
            <span>Description</span>
            <textarea v-model="description" rows="5" maxlength="5000" placeholder="Optional description" />
          </label>
          <button class="primary-button" type="submit" :disabled="!canSubmit">
            {{ submitting ? 'Creating publish attempt…' : 'Create publish attempt' }}
          </button>
        </form>
      </section>
    </div>

    <section v-if="attempt" class="panel attempt-panel" aria-live="polite">
      <div class="panel-heading">
        <div>
          <p class="eyebrow">Selected attempt</p>
          <h2>{{ attempt.title }}</h2>
        </div>
        <span class="status-pill" :class="attemptTone(attempt)">{{ stateLabel(attempt.state) }}</span>
      </div>

      <div class="attempt-grid">
        <div><span>Attempt</span><strong>{{ attempt.id }}</strong></div>
        <div><span>Artifact</span><strong>{{ attempt.render_artifact_id }}</strong></div>
        <div><span>Uploaded</span><strong>{{ attempt.uploaded_bytes.toLocaleString() }} bytes</strong></div>
        <div><span>Updated</span><strong>{{ new Date(attempt.updated_at).toLocaleString() }}</strong></div>
      </div>

      <div v-if="uploadPercent !== null" class="progress-block">
        <div class="progress-copy"><span>Upload progress</span><strong>{{ uploadPercent }}%</strong></div>
        <progress :value="uploadPercent" max="100">{{ uploadPercent }}%</progress>
        <small v-if="selectedAttemptArtifact">{{ formatBytes(attempt.uploaded_bytes) }} of {{ formatBytes(selectedAttemptArtifact.byte_size) }}</small>
      </div>
      <p v-else-if="attempt.state === 'uploading' || attempt.state === 'retryable_failure'" class="state-text">Upload progress is indeterminate because artifact total is unavailable in this workspace snapshot.</p>
      <p v-if="isLiveProgressState(attempt.state)" class="live-refresh-status" role="status" data-testid="live-progress-status">
        {{ liveRefreshing ? 'Refreshing saved upload progress…' : liveRefreshMessage || 'Upload progress refreshes automatically while this attempt is queued or uploading.' }}
      </p>

      <div v-if="attempt.last_error_code" class="notice error">Publishing needs attention: {{ attempt.last_error_code }}</div>
      <p v-if="attempt.last_error_code === 'youtube_status_retryable'" class="warning-copy">The remote video already exists. Retry only the YouTube status check; do not restart the upload.</p>
      <p v-if="attempt.state === 'retryable_failure'" class="warning-copy">Retry preserves this logical attempt and resumable offset; it does not create a duplicate remote upload.</p>
      <div v-if="attempt.state === 'reconnect_required'" class="recovery-box">
        <p class="warning-copy">Reconnect YouTube, then refresh this attempt before continuing.</p>
        <button class="primary-button" type="button" :disabled="connecting" @click="connectYouTube">
          {{ connecting ? 'Opening YouTube…' : 'Reconnect YouTube' }}
        </button>
      </div>
      <a v-if="attempt.remote_video_id" class="text-link" :href="`https://www.youtube.com/watch?v=${attempt.remote_video_id}`" target="_blank" rel="noopener noreferrer">Open remote video</a>
      <div class="attempt-actions">
        <button v-if="attempt.state === 'retryable_failure'" class="primary-button" type="button" :disabled="retrying || reconciling" @click="retryAttempt">
          {{ retrying ? 'Queueing retry…' : 'Retry upload' }}
        </button>
        <button v-if="canReconcile" class="primary-button" type="button" :disabled="reconciling || refreshing || retrying" @click="reconcileAttempt">
          {{ reconciling ? 'Checking YouTube…' : attempt.last_error_code === 'youtube_status_retryable' ? 'Retry YouTube status check' : 'Check YouTube status' }}
        </button>
        <button class="secondary-button" type="button" :disabled="refreshing || liveRefreshing || retrying || reconciling" @click="refreshAttempt()">
          {{ refreshing || liveRefreshing ? 'Refreshing…' : 'Refresh saved state' }}
        </button>
      </div>
    </section>

    <section class="panel history-panel" aria-labelledby="history-title">
      <div class="panel-heading">
        <div>
          <p class="eyebrow">Durable history</p>
          <h2 id="history-title">Publish attempts</h2>
        </div>
        <span class="count-badge">{{ attempts.length }}</span>
      </div>
      <p v-if="loading" class="state-text">Loading publish history…</p>
      <div v-else-if="attempts.length === 0" class="empty-state">
        <strong>No publish attempts yet</strong>
        <p>Create an attempt above; it remains visible here after refresh or restart.</p>
      </div>
      <div v-else class="history-list">
        <button v-for="item in attempts" :key="item.id" type="button" class="history-row" :data-attempt-id="item.id" @click="inspectAttempt(item)">
          <span class="history-main">
            <strong>{{ item.title }}</strong>
            <span>{{ new Date(item.created_at).toLocaleString() }} · {{ item.uploaded_bytes.toLocaleString() }} bytes</span>
          </span>
          <span class="status-pill" :class="attemptTone(item)">{{ stateLabel(item.state) }}</span>
        </button>
      </div>
    </section>
  </section>
</template>

<style scoped>
.channel-hub { max-width: 1180px; }
.hub-heading, .panel-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 20px; }
.hub-grid { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1.1fr); gap: 20px; margin-top: 24px; }
.panel { padding: 22px; border: 1px solid #d4ddd8; border-radius: 14px; background: #fff; box-shadow: 0 8px 24px rgba(20, 61, 54, 0.05); }
.panel h2 { margin: 4px 0 0; }
.eyebrow { margin: 0; font-size: 12px; font-weight: 800; letter-spacing: .08em; text-transform: uppercase; color: #527069; }
.body-copy, .state-text { color: #587068; }
.text-link { color: #245c50; font-weight: 700; }
.count-badge { min-width: 32px; padding: 6px 10px; border-radius: 999px; background: #edf3f0; text-align: center; font-weight: 700; }
.connection-list, .history-list { display: grid; gap: 10px; margin-top: 18px; }
.connection-card { display: flex; min-height: 64px; gap: 12px; align-items: flex-start; padding: 14px; border: 1px solid #cbd6d0; border-radius: 10px; cursor: pointer; }
.connection-card.selected { border-color: #2f695c; box-shadow: 0 0 0 2px rgba(47, 105, 92, 0.12); }
.connection-card input { margin-top: 4px; }
.connection-copy { display: grid; gap: 5px; min-width: 0; }
.connection-title { font-weight: 800; }
.connection-meta { color: #64776f; overflow-wrap: anywhere; }
.status-pill { display: inline-flex; width: fit-content; padding: 4px 8px; border-radius: 999px; font-size: 12px; font-weight: 800; text-transform: capitalize; }
.status-pill.good { background: #e8f5ef; color: #1f654f; }
.status-pill.warn { background: #fff4d8; color: #805d00; }
.status-pill.bad { background: #fdebea; color: #9b302b; }
.capability-box, .recovery-box, .empty-state { margin-top: 18px; padding: 14px; border-radius: 10px; background: #f6f9f7; }
.capability-box ul { padding-left: 20px; }
.recovery-box { background: #fff8e7; }
.warning-copy { color: #775b13; }
.notice { margin-top: 16px; padding: 12px 14px; border-radius: 10px; }
.notice.error { background: #fdebea; color: #8f2d28; }
.notice.success { background: #e8f5ef; color: #205f4c; }
.publish-form { display: grid; gap: 16px; margin-top: 18px; }
.publish-form label { display: grid; gap: 7px; font-weight: 700; }
.publish-form input, .publish-form textarea, .publish-form select { width: 100%; min-height: 44px; padding: 10px 12px; border: 1px solid #bccac3; border-radius: 9px; background: #fff; font: inherit; }
.publish-form textarea { resize: vertical; }
.publish-form small { color: #61746c; font-weight: 400; }
.primary-button, .secondary-button { min-height: 44px; padding: 10px 16px; border-radius: 9px; font: inherit; font-weight: 800; cursor: pointer; }
.primary-button { border: 1px solid #285e53; background: #285e53; color: #fff; }
.secondary-button { border: 1px solid #b7c6bf; background: #fff; color: #264f47; }
.primary-button:disabled, .secondary-button:disabled { opacity: .55; cursor: not-allowed; }
.primary-button:focus-visible, .secondary-button:focus-visible, .history-row:focus-visible, .connection-card:focus-within, input:focus-visible, textarea:focus-visible, select:focus-visible { outline: 3px solid rgba(47, 105, 92, .28); outline-offset: 2px; }
.attempt-panel, .history-panel { margin-top: 20px; }
.attempt-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; margin-top: 18px; }
.attempt-grid div { display: grid; gap: 5px; min-width: 0; padding: 12px; border-radius: 9px; background: #f7f9f8; }
.attempt-grid span { color: #687b73; font-size: 12px; }
.attempt-grid strong { overflow-wrap: anywhere; }
.progress-block { display: grid; gap: 8px; margin-top: 18px; }
.progress-copy { display: flex; justify-content: space-between; gap: 12px; }
progress { width: 100%; height: 12px; }
.live-refresh-status { margin: 10px 0 0; color: #587068; font-size: 13px; }
.attempt-actions { display: flex; flex-wrap: wrap; gap: 10px; margin-top: 18px; }
.history-row { display: flex; width: 100%; min-height: 58px; align-items: center; justify-content: space-between; gap: 14px; padding: 12px 14px; border: 1px solid #d3ddd8; border-radius: 10px; background: #fff; text-align: left; cursor: pointer; }
.history-row:hover { border-color: #8daaa0; }
.history-main { display: grid; gap: 4px; min-width: 0; }
.history-main span { color: #65766f; overflow-wrap: anywhere; }

@media (max-width: 860px) {
  .hub-grid { grid-template-columns: 1fr; }
  .hub-heading { align-items: stretch; }
  .hub-heading > .secondary-button { flex: 0 0 auto; }
}

@media (max-width: 620px) {
  .channel-hub { padding-inline: 14px; }
  .hub-heading, .panel-heading { flex-direction: column; }
  .hub-heading > .secondary-button, .primary-button, .secondary-button { width: 100%; }
  .panel { padding: 16px; border-radius: 12px; }
  .attempt-grid { grid-template-columns: 1fr; }
  .history-row { align-items: flex-start; flex-direction: column; }
  .attempt-actions { display: grid; }
}
</style>