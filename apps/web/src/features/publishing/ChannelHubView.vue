<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink, useRoute } from 'vue-router'

import { ApiError } from '@/api/projects'
import {
  createPublishAttempt,
  getPublishAttempt,
  listPublishArtifacts,
  listPublishAttempts,
  listPublishingConnections,
  retryPublishAttempt,
  type ChannelConnection,
  type PublishArtifactSummary,
  type PublishAttempt,
} from '@/api/publishing'

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
const retrying = ref(false)
const errorMessage = ref('')
const attempt = ref<PublishAttempt | null>(null)

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

onMounted(() => {
  void loadWorkspace()
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

async function submit() {
  if (!canSubmit.value) {
    return
  }
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

async function refreshAttempt(item: PublishAttempt = attempt.value as PublishAttempt) {
  if (!item) {
    return
  }
  refreshing.value = true
  errorMessage.value = ''
  try {
    const refreshed = await getPublishAttempt(projectID.value, item.id)
    attempt.value = refreshed
    attempts.value = attempts.value.map((entry) => entry.id === refreshed.id ? refreshed : entry)
  } catch (error) {
    errorMessage.value = error instanceof ApiError ? error.message : 'Could not refresh publish status.'
  } finally {
    refreshing.value = false
  }
}

async function retryAttempt() {
  if (!attempt.value || attempt.value.state !== 'retryable_failure' || retrying.value) {
    return
  }
  retrying.value = true
  errorMessage.value = ''
  try {
    const retried = await retryPublishAttempt(projectID.value, attempt.value.id)
    attempt.value = retried
    attempts.value = attempts.value.map((entry) => entry.id === retried.id ? retried : entry)
  } catch (error) {
    errorMessage.value = error instanceof ApiError ? error.message : 'Could not queue the publish retry.'
  } finally {
    retrying.value = false
  }
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
        <p class="body-copy">Publish an immutable rendered video to a connected YouTube channel with recoverable upload state.</p>
      </div>
      <button class="secondary-button" type="button" :disabled="loading" @click="loadWorkspace">
        {{ loading ? 'Refreshing…' : 'Refresh workspace' }}
      </button>
    </div>

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
          <p>Connect YouTube before creating a publish attempt. OAuth credentials stay on the server.</p>
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
          <p v-if="selectedConnection.state === 'reconnect_required'" class="warning-copy">Authorization must be reconnected before upload can continue.</p>
          <p v-else-if="!selectedConnection.capabilities.can_publish" class="warning-copy">This connection cannot promise public publication. Upload controls stay limited to the capability reported by the server.</p>
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
      <p v-else-if="attempt.state === 'uploading' || attempt.state === 'retryable_failure'" class="state-text">Upload progress is indeterminate because artifact total is not available in this workspace snapshot.</p>

      <div v-if="attempt.last_error_code" class="notice error">Upload requires attention: {{ attempt.last_error_code }}</div>
      <p v-if="attempt.state === 'retryable_failure'" class="warning-copy">Retry keeps this logical attempt and its durable resumable progress; it does not create another remote upload.</p>
      <p v-if="attempt.state === 'reconnect_required'" class="warning-copy">Reconnect this YouTube channel before resuming this attempt.</p>
      <a v-if="attempt.remote_video_id" class="text-link" :href="`https://www.youtube.com/watch?v=${attempt.remote_video_id}`" target="_blank" rel="noopener noreferrer">Open remote video</a>
      <div class="attempt-actions">
        <button v-if="attempt.state === 'retryable_failure'" class="primary-button" type="button" :disabled="retrying" @click="retryAttempt">
          {{ retrying ? 'Queueing retry…' : 'Retry upload' }}
        </button>
        <button class="secondary-button" type="button" :disabled="refreshing || retrying" @click="refreshAttempt()">
          {{ refreshing ? 'Refreshing…' : 'Refresh status' }}
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
        <p>Create an attempt above; it will remain visible here after refresh or restart.</p>
      </div>
      <div v-else class="history-list">
        <button v-for="item in attempts" :key="item.id" type="button" class="history-row" @click="inspectAttempt(item)">
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
.panel h2 { margin-top: 4px; }
.count-badge { min-width: 32px; padding: 6px 10px; border-radius: 999px; background: #edf3f0; text-align: center; font-weight: 700; }
.connection-list, .history-list { display: grid; gap: 10px; margin-top: 18px; }
.connection-card { display: flex; min-height: 64px; gap: 12px; align-items: flex-start; padding: 14px; border: 1px solid #cbd6d0; border-radius: 10px; cursor: pointer; }
.connection-card.selected { border-color: #2f695c; box-shadow: 0 0 0 2px rgba(47, 105, 92, 0.12); }
.connection-card input { margin-top: 4px; }
.connection-copy { display: grid; gap: 4px; min-width: 0; }
.connection-title { font-weight: 800; overflow-wrap: anywhere; }
.connection-meta { color: #60716b; font-size: 0.9rem; overflow-wrap: anywhere; }
.status-pill { display: inline-flex; width: fit-content; align-items: center; min-height: 28px; padding: 4px 9px; border-radius: 999px; font-size: 0.8rem; font-weight: 800; text-transform: capitalize; }
.status-pill.good { background: #e5f5ed; color: #176044; }
.status-pill.warn { background: #fff3d5; color: #775300; }
.status-pill.bad { background: #fde9e7; color: #8b2f2a; }
.capability-box, .empty-state { margin-top: 18px; padding: 14px; border-radius: 10px; background: #f5f8f7; }
.capability-box ul { margin: 10px 0 0; padding-left: 20px; }
.warning-copy { color: #775300; font-weight: 600; }
.publish-form { display: grid; gap: 16px; margin-top: 18px; }
.publish-form label { display: grid; gap: 7px; font-weight: 700; }
.publish-form input, .publish-form textarea, .publish-form select { width: 100%; box-sizing: border-box; min-height: 44px; padding: 11px 12px; border: 1px solid #aebdb6; border-radius: 8px; background: #fff; font: inherit; }
.publish-form textarea { min-height: 120px; }
.publish-form input:focus-visible, .publish-form textarea:focus-visible, .publish-form select:focus-visible, .connection-card:focus-within, .history-row:focus-visible, .attempt-actions button:focus-visible { outline: 3px solid #7aa995; outline-offset: 2px; }
.publish-form small { color: #60716b; font-weight: 400; line-height: 1.45; }
.primary-button, .secondary-button { min-height: 44px; }
.attempt-panel, .history-panel { margin-top: 20px; }
.attempt-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; margin: 18px 0; }
.attempt-grid div { display: grid; gap: 4px; min-width: 0; padding: 12px; border-radius: 8px; background: #f5f8f7; }
.attempt-grid span { color: #60716b; font-size: 0.82rem; }
.attempt-grid strong { overflow-wrap: anywhere; }
.progress-block { display: grid; gap: 8px; margin: 14px 0; }
.progress-copy { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.progress-block progress { width: 100%; height: 12px; }
.progress-block small { color: #60716b; }
.attempt-actions { display: flex; flex-wrap: wrap; gap: 10px; margin-top: 14px; }
.history-row { display: flex; width: 100%; min-height: 58px; align-items: center; justify-content: space-between; gap: 14px; padding: 12px 14px; border: 1px solid #d4ddd8; border-radius: 10px; background: #fff; text-align: left; cursor: pointer; }
.history-row:hover { background: #f8faf9; }
.history-main { display: grid; gap: 4px; min-width: 0; }
.history-main strong { overflow-wrap: anywhere; }
.history-main span { color: #60716b; font-size: 0.88rem; }
@media (max-width: 860px) { .hub-grid { grid-template-columns: 1fr; } }
@media (max-width: 640px) {
  .hub-heading, .panel-heading { flex-direction: column; align-items: stretch; }
  .panel { padding: 16px; border-radius: 10px; }
  .attempt-grid { grid-template-columns: 1fr; }
  .hub-heading .secondary-button, .primary-button { width: 100%; }
  .attempt-actions { display: grid; grid-template-columns: 1fr; }
  .attempt-actions button { width: 100%; }
  .history-row { align-items: flex-start; flex-direction: column; }
}
</style>