<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink, useRoute } from 'vue-router'

import { ApiError } from '@/api/projects'
import {
  createPublishAttempt,
  getPublishAttempt,
  listPublishingConnections,
  type ChannelConnection,
  type PublishAttempt,
} from '@/api/publishing'

const route = useRoute()
const projectID = computed(() => String(route.params.id ?? ''))

const connections = ref<ChannelConnection[]>([])
const selectedConnectionID = ref('')
const renderArtifactID = ref('')
const title = ref('')
const description = ref('')
const loading = ref(true)
const submitting = ref(false)
const refreshing = ref(false)
const errorMessage = ref('')
const attempt = ref<PublishAttempt | null>(null)

const selectedConnection = computed(() => connections.value.find((item) => item.id === selectedConnectionID.value) ?? null)
const canSubmit = computed(() => {
  const connection = selectedConnection.value
  return Boolean(
    connection?.state === 'connected'
      && connection.capabilities.can_upload
      && renderArtifactID.value.trim()
      && title.value.trim()
      && !submitting.value,
  )
})

onMounted(() => {
  void loadConnections()
})

async function loadConnections() {
  loading.value = true
  errorMessage.value = ''
  try {
    connections.value = await listPublishingConnections()
    const firstUsable = connections.value.find((item) => item.state === 'connected' && item.capabilities.can_upload)
    selectedConnectionID.value = firstUsable?.id ?? connections.value[0]?.id ?? ''
  } catch (error) {
    errorMessage.value = error instanceof ApiError ? error.message : 'Could not load publishing connections.'
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
    attempt.value = await createPublishAttempt(projectID.value, {
      connection_id: selectedConnectionID.value,
      render_artifact_id: renderArtifactID.value.trim(),
      request_id: crypto.randomUUID(),
      title: title.value.trim(),
      description: description.value.trim(),
    })
  } catch (error) {
    errorMessage.value = error instanceof ApiError ? error.message : 'Could not create the publish attempt.'
  } finally {
    submitting.value = false
  }
}

async function refreshAttempt() {
  if (!attempt.value) {
    return
  }
  refreshing.value = true
  errorMessage.value = ''
  try {
    attempt.value = await getPublishAttempt(projectID.value, attempt.value.id)
  } catch (error) {
    errorMessage.value = error instanceof ApiError ? error.message : 'Could not refresh publish status.'
  } finally {
    refreshing.value = false
  }
}

function connectionTone(connection: ChannelConnection): string {
  return connection.state === 'connected' ? 'good' : connection.state === 'reconnect_required' ? 'warn' : 'bad'
}

function stateLabel(state: string): string {
  return state.replaceAll('_', ' ')
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
      <button class="secondary-button" type="button" :disabled="loading" @click="loadConnections">
        {{ loading ? 'Refreshing…' : 'Refresh channels' }}
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
            <span>Render artifact ID</span>
            <input v-model="renderArtifactID" required autocomplete="off" inputmode="text" placeholder="UUID of a successful immutable render artifact">
            <small>The API verifies owner, project, media type and immutable render lineage before accepting it.</small>
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
          <p class="eyebrow">Latest attempt</p>
          <h2>{{ attempt.title }}</h2>
        </div>
        <span class="status-pill" :class="attempt.state === 'rejected' ? 'bad' : attempt.state.includes('failure') || attempt.state === 'reconnect_required' ? 'warn' : 'good'">
          {{ stateLabel(attempt.state) }}
        </span>
      </div>

      <div class="attempt-grid">
        <div><span>Attempt</span><strong>{{ attempt.id }}</strong></div>
        <div><span>Artifact</span><strong>{{ attempt.render_artifact_id }}</strong></div>
        <div><span>Uploaded</span><strong>{{ attempt.uploaded_bytes.toLocaleString() }} bytes</strong></div>
        <div><span>Updated</span><strong>{{ new Date(attempt.updated_at).toLocaleString() }}</strong></div>
      </div>

      <div v-if="attempt.last_error_code" class="notice error">Upload requires attention: {{ attempt.last_error_code }}</div>
      <a v-if="attempt.remote_video_id" class="text-link" :href="`https://www.youtube.com/watch?v=${attempt.remote_video_id}`" target="_blank" rel="noopener noreferrer">Open remote video</a>
      <button class="secondary-button" type="button" :disabled="refreshing" @click="refreshAttempt">
        {{ refreshing ? 'Refreshing…' : 'Refresh status' }}
      </button>
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
.connection-list { display: grid; gap: 10px; margin-top: 18px; }
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
.publish-form input, .publish-form textarea { width: 100%; box-sizing: border-box; padding: 11px 12px; border: 1px solid #aebdb6; border-radius: 8px; font: inherit; }
.publish-form input:focus-visible, .publish-form textarea:focus-visible, .connection-card:focus-within { outline: 3px solid #7aa995; outline-offset: 2px; }
.publish-form small { color: #60716b; font-weight: 400; line-height: 1.45; }
.primary-button, .secondary-button { min-height: 44px; }
.attempt-panel { margin-top: 20px; }
.attempt-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; margin: 18px 0; }
.attempt-grid div { display: grid; gap: 4px; min-width: 0; padding: 12px; border-radius: 8px; background: #f5f8f7; }
.attempt-grid span { color: #60716b; font-size: 0.82rem; }
.attempt-grid strong { overflow-wrap: anywhere; }
.attempt-panel .secondary-button { margin-top: 14px; }
@media (max-width: 860px) { .hub-grid { grid-template-columns: 1fr; } }
@media (max-width: 640px) {
  .hub-heading, .panel-heading { flex-direction: column; align-items: stretch; }
  .panel { padding: 16px; border-radius: 10px; }
  .attempt-grid { grid-template-columns: 1fr; }
  .hub-heading .secondary-button, .primary-button { width: 100%; }
}
</style>
