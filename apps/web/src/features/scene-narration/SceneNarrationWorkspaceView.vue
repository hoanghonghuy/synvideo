<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { getProject, type Project } from '@/api/projects'
import { listScenePlans, getScenePlan, type ScenePlanSummary, type ScenePlan } from '@/features/scene-plan/api'
import {
  fetchTTSOptions,
  listSceneNarrations,
  createSceneNarrationGeneration,
  getSceneNarrationGeneration,
  assignSceneNarration,
  listSceneNarrationHistory,
  audioContentURL,
  type TTSOptionProvider,
  type SceneNarrationEntry,
} from './api'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()

const projectID = computed(() => String(route.params.id || ''))

const project = ref<Project | null>(null)
const plans = ref<ScenePlanSummary[]>([])
const selectedPlanVersion = ref<number | null>(null)
const currentPlan = ref<ScenePlan | null>(null)
const narrationEntries = ref<Record<string, SceneNarrationEntry>>({})
const ttsProviders = ref<TTSOptionProvider[]>([])

// Settings
const selectedProviderId = ref<string>('')
const selectedModelId = ref<string>('')
const selectedVoiceId = ref<string>('')
const selectedFormat = ref<string>('mp3')
const autoAssignCurrent = ref<boolean>(true)

// Per-scene generation state
const generatingScenes = ref<Record<string, { jobID: string; state: string }>>({})
const sceneErrors = ref<Record<string, string>>({})

// History Modal
const activeHistorySceneKey = ref<string | null>(null)
const historyEntries = ref<SceneNarrationEntry[]>([])
const loadingHistory = ref(false)

const loading = ref(true)
const pageError = ref<string | null>(null)

const approvedPlans = computed(() => plans.value.filter((p: ScenePlanSummary) => p.status === 'approved'))

const selectedProvider = computed(() =>
  ttsProviders.value.find((p) => p.id === selectedProviderId.value),
)

const availableModels = computed(() => selectedProvider.value?.models || [])
const availableVoices = computed(() => selectedProvider.value?.voices || [])

async function loadInitialData() {
  loading.value = true
  pageError.value = null
  try {
    const [p, plansList, ttsOpts] = await Promise.all([
      getProject(projectID.value),
      listScenePlans(projectID.value),
      fetchTTSOptions().catch(() => ({ providers: [] })),
    ])
    project.value = p
    plans.value = plansList
    ttsProviders.value = ttsOpts.providers || []

    // Select first provider/model/voice by default
    if (ttsProviders.value.length > 0 && ttsProviders.value[0]) {
      const firstProv = ttsProviders.value[0]
      selectedProviderId.value = firstProv.id
      if (firstProv.models.length > 0 && firstProv.models[0]) {
        selectedModelId.value = firstProv.models[0].id
      }
      if (firstProv.voices.length > 0 && firstProv.voices[0]) {
        selectedVoiceId.value = firstProv.voices[0].id
      }
    }

    const approved = plansList.filter((item: ScenePlanSummary) => item.status === 'approved')
    if (approved.length > 0 && approved[0]) {
      selectedPlanVersion.value = approved[0].version
      await loadPlanVersion(approved[0].version)
    }
  } catch (err: any) {
    pageError.value = err.message || t('sceneNarration.errors.request_failed')
  } finally {
    loading.value = false
  }
}

watch(selectedProviderId, (newProviderId) => {
  const prov = ttsProviders.value.find((p) => p.id === newProviderId)
  if (prov) {
    if (prov.models.length > 0 && prov.models[0]) {
      selectedModelId.value = prov.models[0].id
    }
    if (prov.voices.length > 0 && prov.voices[0]) {
      selectedVoiceId.value = prov.voices[0].id
    }
  }
})

watch(selectedPlanVersion, async (newVersion) => {
  if (newVersion) {
    await loadPlanVersion(newVersion)
  }
})

async function loadPlanVersion(version: number) {
  try {
    const [planDetail, bindingsList] = await Promise.all([
      getScenePlan(projectID.value, version),
      listSceneNarrations(projectID.value, version).catch(() => []),
    ])
    currentPlan.value = planDetail

    const map: Record<string, SceneNarrationEntry> = {}
    for (const b of bindingsList) {
      map[b.scene_key] = b
    }
    narrationEntries.value = map
  } catch (err: any) {
    pageError.value = err.message || t('sceneNarration.errors.request_failed')
  }
}

function generateUUID(): string {
  if (typeof crypto !== 'undefined' && crypto.randomUUID) {
    return crypto.randomUUID()
  }
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0
    const v = c === 'x' ? r : (r & 0x3) | 0x8
    return v.toString(16)
  })
}

async function handleGenerate(sceneKey: string) {
  if (!selectedPlanVersion.value || !selectedProviderId.value || !selectedModelId.value || !selectedVoiceId.value) {
    return
  }

  const reqID = generateUUID()
  generatingScenes.value[sceneKey] = { jobID: reqID, state: 'queued' }
  delete sceneErrors.value[sceneKey]

  try {
    const job = await createSceneNarrationGeneration(
      projectID.value,
      selectedPlanVersion.value,
      sceneKey,
      {
        request_id: reqID,
        provider_id: selectedProviderId.value,
        model_id: selectedModelId.value,
        voice_id: selectedVoiceId.value,
        format: selectedFormat.value,
        assign_current: autoAssignCurrent.value,
      },
    )

    generatingScenes.value[sceneKey] = { jobID: job.id, state: job.state }

    if (job.state === 'succeeded') {
      await loadPlanVersion(selectedPlanVersion.value)
      delete generatingScenes.value[sceneKey]
      return
    }

    // Poll until complete
    pollJob(sceneKey, job.id)
  } catch (err: any) {
    delete generatingScenes.value[sceneKey]
    sceneErrors.value[sceneKey] = err.message || t('sceneNarration.errors.generation_failed')
  }
}

async function pollJob(sceneKey: string, jobID: string) {
  const maxAttempts = 60
  for (let i = 0; i < maxAttempts; i++) {
    await new Promise((r) => setTimeout(r, 1000))
    try {
      const job = await getSceneNarrationGeneration(projectID.value, jobID)
      if (generatingScenes.value[sceneKey]) {
        generatingScenes.value[sceneKey] = { jobID: job.id, state: job.state }
      }

      if (job.state === 'succeeded') {
        if (selectedPlanVersion.value) {
          await loadPlanVersion(selectedPlanVersion.value)
        }
        delete generatingScenes.value[sceneKey]
        return
      }

      if (job.state === 'failed') {
        delete generatingScenes.value[sceneKey]
        sceneErrors.value[sceneKey] = job.error_code || t('sceneNarration.errors.generation_failed')
        return
      }
    } catch {
      // Continue polling on transient errors
    }
  }
  delete generatingScenes.value[sceneKey]
  sceneErrors.value[sceneKey] = t('sceneNarration.errors.generation_failed')
}

async function openHistory(sceneKey: string) {
  if (!selectedPlanVersion.value) return
  activeHistorySceneKey.value = sceneKey
  loadingHistory.value = true
  historyEntries.value = []
  try {
    const list = await listSceneNarrationHistory(
      projectID.value,
      selectedPlanVersion.value,
      sceneKey,
    )
    historyEntries.value = list
  } catch (err: any) {
    sceneErrors.value[sceneKey] = err.message || t('sceneNarration.errors.request_failed')
  } finally {
    loadingHistory.value = false
  }
}

function closeHistory() {
  activeHistorySceneKey.value = null
  historyEntries.value = []
}

async function handleAssignAlternative(assetID: string) {
  if (!selectedPlanVersion.value || !activeHistorySceneKey.value) return
  const sceneKey = activeHistorySceneKey.value
  try {
    await assignSceneNarration(
      projectID.value,
      selectedPlanVersion.value,
      sceneKey,
      assetID,
    )
    await loadPlanVersion(selectedPlanVersion.value)
    closeHistory()
  } catch (err: any) {
    sceneErrors.value[sceneKey] = err.message || t('sceneNarration.errors.request_failed')
  }
}

onMounted(() => {
  loadInitialData()
})
</script>

<template>
  <div class="scene-narration-view">
    <header class="page-header">
      <div class="header-content">
        <span class="eyebrow">{{ t('sceneNarration.eyebrow') }}</span>
        <h1>{{ project?.title || t('sceneNarration.title') }}</h1>
        <p class="description">{{ t('sceneNarration.description') }}</p>
      </div>
      <div class="header-actions">
        <button class="btn btn-secondary" @click="router.push(`/projects/${projectID}`)">
          {{ t('projects.actions.backToList') }}
        </button>
      </div>
    </header>

    <div v-if="pageError" class="alert alert-error">
      {{ pageError }}
    </div>

    <div v-if="loading" class="loading-state">
      <div class="spinner"></div>
      <span>{{ t('projects.states.loading') }}...</span>
    </div>

    <div v-else-if="approvedPlans.length === 0" class="empty-state">
      <h3>{{ t('sceneNarration.noApprovedPlan') }}</h3>
      <div class="empty-actions">
        <button class="btn btn-primary" @click="router.push(`/projects/${projectID}/scene-plan`)">
          {{ t('sceneNarration.openScenePlan') }}
        </button>
      </div>
    </div>

    <div v-else class="workspace-content">
      <!-- Toolbar Controls: Plan Version & TTS Options -->
      <section class="settings-card">
        <div class="settings-grid">
          <div class="form-group" v-if="approvedPlans.length > 1">
            <label>{{ t('sceneNarration.selectPlanVersion') }}</label>
            <select v-model="selectedPlanVersion" class="form-control">
              <option v-for="plan in approvedPlans" :key="plan.version" :value="plan.version">
                v{{ plan.version }}
              </option>
            </select>
          </div>

          <div class="form-group">
            <label>{{ t('sceneNarration.ttsSettings.provider') }}</label>
            <select v-model="selectedProviderId" class="form-control" data-testid="provider-select">
              <option v-for="prov in ttsProviders" :key="prov.id" :value="prov.id">
                {{ prov.display_name }}
              </option>
            </select>
          </div>

          <div class="form-group">
            <label>{{ t('sceneNarration.ttsSettings.model') }}</label>
            <select v-model="selectedModelId" class="form-control" data-testid="model-select">
              <option v-for="m in availableModels" :key="m.id" :value="m.id">
                {{ m.display_name }}
              </option>
            </select>
          </div>

          <div class="form-group">
            <label>{{ t('sceneNarration.ttsSettings.voice') }}</label>
            <select v-model="selectedVoiceId" class="form-control" data-testid="voice-select">
              <option v-for="v in availableVoices" :key="v.id" :value="v.id">
                {{ v.display_name }}
              </option>
            </select>
          </div>

          <div class="form-group">
            <label>{{ t('sceneNarration.ttsSettings.format') }}</label>
            <select v-model="selectedFormat" class="form-control">
              <option value="mp3">MP3</option>
              <option value="wav">WAV</option>
            </select>
          </div>
        </div>

        <div v-if="ttsProviders.length === 0" class="alert alert-warning">
          {{ t('sceneNarration.ttsSettings.noProviders') }}
          <button class="btn btn-link" @click="router.push('/provider-settings')">
            {{ t('sceneNarration.openProviderSettings') }}
          </button>
        </div>
      </section>

      <!-- Scenes List -->
      <section class="scenes-list">
        <article
          v-for="scene in currentPlan?.scenes || []"
          :key="scene.key"
          class="scene-card"
          :data-testid="`scene-${scene.key}`"
        >
          <div class="scene-card-header">
            <div class="scene-title-group">
              <span class="badge badge-primary">{{ t('sceneNarration.sceneCard.sceneKey', { key: scene.key }) }}</span>
              <span class="badge badge-secondary">{{ t('sceneNarration.sceneCard.section', { section: scene.script_section_key }) }}</span>
              <span class="duration-hint">{{ t('sceneNarration.sceneCard.expectedDuration', { seconds: scene.expected_duration_seconds }) }}</span>
            </div>

            <div class="scene-actions">
              <button
                class="btn btn-secondary btn-sm"
                @click="openHistory(scene.key)"
                :data-testid="`history-btn-${scene.key}`"
              >
                {{ t('sceneNarration.sceneCard.actions.history', { count: narrationEntries[scene.key]?.binding ? 1 : 0 }) }}
              </button>

              <button
                class="btn btn-primary btn-sm"
                :disabled="Boolean(generatingScenes[scene.key]) || !selectedVoiceId"
                @click="handleGenerate(scene.key)"
                :data-testid="`generate-narration-${scene.key}`"
              >
                {{ generatingScenes[scene.key] ? t('sceneNarration.sceneCard.actions.generating') : t('sceneNarration.sceneCard.actions.generate') }}
              </button>
            </div>
          </div>

          <div class="scene-card-body">
            <div class="narration-box">
              <label class="narration-label">{{ t('sceneNarration.sceneCard.narrationText') }}</label>
              <p class="narration-text">{{ scene.narration }}</p>
            </div>

            <div v-if="sceneErrors[scene.key]" class="alert alert-error">
              {{ sceneErrors[scene.key] }}
            </div>

            <!-- Active Audio Player or Generation State -->
            <div class="audio-status-panel">
              <div v-if="generatingScenes[scene.key]" class="generating-indicator">
                <div class="spinner-sm"></div>
                <span>{{ t('sceneNarration.sceneCard.status.generating') }} ({{ generatingScenes[scene.key]?.state }})</span>
              </div>

              <div v-else-if="narrationEntries[scene.key]?.binding && narrationEntries[scene.key]?.asset" class="audio-player-container">
                <div class="audio-meta">
                  <span class="badge badge-success">{{ t('sceneNarration.sceneCard.status.active') }} (v{{ narrationEntries[scene.key]?.binding?.binding_version }})</span>
                  <span v-if="narrationEntries[scene.key]?.asset?.metadata?.duration_seconds" class="measured-duration">
                    {{ t('sceneNarration.sceneCard.measuredDuration', { seconds: Number(narrationEntries[scene.key]?.asset?.metadata?.duration_seconds).toFixed(1) }) }}
                  </span>
                </div>
                <audio
                  controls
                  class="audio-player"
                  :src="audioContentURL(projectID, narrationEntries[scene.key]?.binding?.asset_id || '')"
                  :data-testid="`audio-player-${scene.key}`"
                ></audio>
              </div>

              <div v-else class="empty-audio-notice">
                <span class="text-muted">{{ t('sceneNarration.sceneCard.status.none') }}</span>
              </div>
            </div>
          </div>
        </article>
      </section>
    </div>

    <!-- History Modal / Drawer -->
    <div v-if="activeHistorySceneKey" class="modal-backdrop" @click.self="closeHistory">
      <div class="modal-dialog">
        <div class="modal-header">
          <h3>{{ t('sceneNarration.historyModal.title', { key: activeHistorySceneKey }) }}</h3>
          <button class="btn-close" @click="closeHistory">&times;</button>
        </div>
        <div class="modal-body">
          <div v-if="loadingHistory" class="loading-state">
            <div class="spinner-sm"></div>
            <span>{{ t('projects.states.loading') }}...</span>
          </div>
          <div v-else-if="historyEntries.length === 0" class="empty-state">
            <p>{{ t('sceneNarration.historyModal.empty') }}</p>
          </div>
          <div v-else class="history-list">
            <div
              v-for="entry in historyEntries"
              :key="entry.binding?.id"
              class="history-item"
              :class="{ active: entry.binding?.status === 'active' }"
            >
              <div class="history-item-header">
                <span class="badge" :class="entry.binding?.status === 'active' ? 'badge-success' : 'badge-secondary'">
                  {{ entry.binding?.status === 'active' ? t('sceneNarration.historyModal.currentBadge') : t('sceneNarration.historyModal.supersededBadge') }}
                  (v{{ entry.binding?.binding_version }})
                </span>
                <span class="history-time">
                  {{ t('sceneNarration.historyModal.createdAt', { time: new Date(entry.binding?.created_at || '').toLocaleString() }) }}
                </span>
              </div>

              <div class="history-player">
                <audio
                  controls
                  class="audio-player"
                  :src="audioContentURL(projectID, entry.binding?.asset_id || '')"
                ></audio>
              </div>

              <div class="history-item-actions">
                <button
                  v-if="entry.binding?.status !== 'active' && entry.binding?.asset_id"
                  class="btn btn-secondary btn-sm"
                  @click="handleAssignAlternative(entry.binding?.asset_id)"
                >
                  {{ t('sceneNarration.sceneCard.actions.useAlternative') }}
                </button>
              </div>
            </div>
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="closeHistory">
            {{ t('sceneNarration.historyModal.close') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.scene-narration-view {
  max-width: 1200px;
  margin: 0 auto;
  padding: 1.5rem;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 2rem;
  border-bottom: 1px solid #e2e8f0;
  padding-bottom: 1.5rem;
}

.eyebrow {
  display: block;
  font-size: 0.875rem;
  font-weight: 600;
  text-transform: uppercase;
  color: #3b82f6;
  margin-bottom: 0.5rem;
}

.description {
  color: #64748b;
  margin-top: 0.5rem;
}

.settings-card {
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 0.5rem;
  padding: 1.25rem;
  margin-bottom: 2rem;
}

.settings-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 1rem;
}

.form-group label {
  display: block;
  font-size: 0.875rem;
  font-weight: 500;
  color: #334155;
  margin-bottom: 0.375rem;
}

.form-control {
  width: 100%;
  padding: 0.5rem 0.75rem;
  border: 1px solid #cbd5e1;
  border-radius: 0.375rem;
  background-color: white;
  font-size: 0.875rem;
}

.scenes-list {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.scene-card {
  background: white;
  border: 1px solid #e2e8f0;
  border-radius: 0.5rem;
  overflow: hidden;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.scene-card-header {
  background: #f8fafc;
  padding: 1rem 1.25rem;
  border-bottom: 1px solid #e2e8f0;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.scene-title-group {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.duration-hint {
  font-size: 0.8125rem;
  color: #64748b;
  margin-left: 0.5rem;
}

.scene-actions {
  display: flex;
  gap: 0.5rem;
}

.scene-card-body {
  padding: 1.25rem;
}

.narration-box {
  background: #fafafa;
  border-left: 4px solid #3b82f6;
  padding: 0.875rem 1rem;
  border-radius: 0.25rem;
  margin-bottom: 1rem;
}

.narration-label {
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
  color: #64748b;
  display: block;
  margin-bottom: 0.25rem;
}

.narration-text {
  font-size: 1rem;
  color: #1e293b;
  line-height: 1.5;
  margin: 0;
}

.audio-status-panel {
  background: #f1f5f9;
  border-radius: 0.375rem;
  padding: 1rem;
}

.generating-indicator {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  color: #0284c7;
  font-weight: 500;
}

.audio-player-container {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.audio-meta {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.measured-duration {
  font-size: 0.8125rem;
  font-weight: 500;
  color: #475569;
}

.audio-player {
  width: 100%;
  height: 36px;
}

.empty-audio-notice {
  font-size: 0.875rem;
  color: #94a3b8;
  font-style: italic;
}

.badge {
  display: inline-block;
  padding: 0.25rem 0.5rem;
  border-radius: 0.25rem;
  font-size: 0.75rem;
  font-weight: 600;
}

.badge-primary {
  background: #e0f2fe;
  color: #0369a1;
}

.badge-secondary {
  background: #f1f5f9;
  color: #475569;
}

.badge-success {
  background: #dcfce7;
  color: #15803d;
}

.btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0.5rem 1rem;
  font-size: 0.875rem;
  font-weight: 500;
  border-radius: 0.375rem;
  border: 1px solid transparent;
  cursor: pointer;
  transition: all 0.15s;
}

.btn-primary {
  background-color: #2563eb;
  color: white;
}

.btn-primary:hover:not(:disabled) {
  background-color: #1d4ed8;
}

.btn-primary:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.btn-secondary {
  background-color: white;
  border-color: #cbd5e1;
  color: #334155;
}

.btn-secondary:hover:not(:disabled) {
  background-color: #f8fafc;
}

.btn-sm {
  padding: 0.375rem 0.75rem;
  font-size: 0.8125rem;
}

.btn-link {
  background: none;
  border: none;
  color: #2563eb;
  text-decoration: underline;
  cursor: pointer;
  padding: 0;
  font-size: inherit;
}

.alert {
  padding: 0.875rem 1rem;
  border-radius: 0.375rem;
  margin-bottom: 1rem;
  font-size: 0.875rem;
}

.alert-error {
  background-color: #fef2f2;
  border: 1px solid #fecaca;
  color: #b91c1c;
}

.alert-warning {
  background-color: #fffbeb;
  border: 1px solid #fde68a;
  color: #b45309;
}

.loading-state,
.empty-state {
  text-align: center;
  padding: 3rem 1rem;
  color: #64748b;
}

.spinner {
  width: 2rem;
  height: 2rem;
  border: 3px solid #e2e8f0;
  border-top-color: #2563eb;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
  margin: 0 auto 1rem;
}

.spinner-sm {
  width: 1.25rem;
  height: 1.25rem;
  border: 2px solid #cbd5e1;
  border-top-color: #2563eb;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.modal-backdrop {
  position: fixed;
  top: 0;
  left: 0;
  width: 100vw;
  height: 100vh;
  background: rgba(0, 0, 0, 0.4);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.modal-dialog {
  background: white;
  border-radius: 0.5rem;
  width: 90%;
  max-width: 600px;
  max-height: 85vh;
  display: flex;
  flex-direction: column;
  box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.1);
}

.modal-header {
  padding: 1rem 1.25rem;
  border-bottom: 1px solid #e2e8f0;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.modal-header h3 {
  margin: 0;
  font-size: 1.125rem;
  color: #1e293b;
}

.btn-close {
  background: none;
  border: none;
  font-size: 1.5rem;
  color: #64748b;
  cursor: pointer;
}

.modal-body {
  padding: 1.25rem;
  overflow-y: auto;
}

.history-list {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.history-item {
  border: 1px solid #e2e8f0;
  border-radius: 0.375rem;
  padding: 0.875rem;
}

.history-item.active {
  border-color: #22c55e;
  background: #f0fdf4;
}

.history-item-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 0.5rem;
}

.history-time {
  font-size: 0.75rem;
  color: #64748b;
}

.history-player {
  margin-bottom: 0.5rem;
}

.modal-footer {
  padding: 1rem 1.25rem;
  border-top: 1px solid #e2e8f0;
  display: flex;
  justify-content: flex-end;
}
</style>
