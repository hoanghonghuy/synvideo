<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'

import { assignPrimaryVisual, mediaAssetContentURL } from '@/features/media/api'
import { getScenePlan, listScenePlans, type Scene, type ScenePlan } from '@/features/scene-plan/api'
import {
  createSceneVideoGeneration,
  fetchVideoGenerationOptions,
  getSceneVideoGeneration,
  type SceneVideoJobView,
  type VideoGenerationOptionModel,
  type VideoGenerationOptionProvider,
} from './api'

const route = useRoute()
const projectId = computed(() => String(route.params.id ?? ''))

const loading = ref(true)
const submittingSceneKey = ref<string | null>(null)
const assigningJobId = ref<string | null>(null)
const errorMessage = ref('')
const plan = ref<ScenePlan | null>(null)
const providers = ref<VideoGenerationOptionProvider[]>([])
const selectedProviderId = ref('')
const selectedModelId = ref('')
const durationSeconds = ref<number | null>(null)
const jobs = ref<Record<string, SceneVideoJobView[]>>({})
let pollTimer: number | undefined

const selectedProvider = computed(() => providers.value.find((provider) => provider.id === selectedProviderId.value))
const selectedModel = computed<VideoGenerationOptionModel | undefined>(() =>
  selectedProvider.value?.models.find((model) => model.id === selectedModelId.value),
)

const durationMin = computed(() => selectedModel.value?.min_duration_seconds ?? 1)
const durationMax = computed(() => selectedModel.value?.max_duration_seconds ?? 3600)

function storageKey(): string {
  return `synvideo:scene-video-jobs:${projectId.value}:${plan.value?.version ?? 'unknown'}`
}

function restoreJobs(): void {
  try {
    const raw = localStorage.getItem(storageKey())
    if (!raw) return
    const parsed = JSON.parse(raw) as Record<string, SceneVideoJobView[]>
    if (parsed && typeof parsed === 'object') jobs.value = parsed
  } catch {
    jobs.value = {}
  }
}

function persistJobs(): void {
  localStorage.setItem(storageKey(), JSON.stringify(jobs.value))
}

function setJob(sceneKey: string, job: SceneVideoJobView): void {
  const current = jobs.value[sceneKey] ?? []
  const index = current.findIndex((item) => item.id === job.id)
  const next = [...current]
  if (index >= 0) next[index] = job
  else next.unshift(job)
  jobs.value = { ...jobs.value, [sceneKey]: next }
  persistJobs()
}

async function load(): Promise<void> {
  loading.value = true
  errorMessage.value = ''
  try {
    const [summaries, options] = await Promise.all([listScenePlans(projectId.value), fetchVideoGenerationOptions()])
    const approved = summaries
      .filter((item) => item.status === 'approved')
      .sort((a, b) => b.version - a.version)[0]
    if (!approved) throw new Error('An approved scene plan is required before generating scene video.')
    plan.value = await getScenePlan(projectId.value, approved.version)
    providers.value = options.providers
    const firstProvider = options.providers[0]
    selectedProviderId.value = firstProvider?.id ?? ''
    selectedModelId.value = firstProvider?.models[0]?.id ?? ''
    durationSeconds.value = selectedModel.value?.min_duration_seconds ?? null
    restoreJobs()
    await refreshPendingJobs()
    schedulePolling()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Unable to load video generation workspace.'
  } finally {
    loading.value = false
  }
}

function onProviderChange(): void {
  selectedModelId.value = selectedProvider.value?.models[0]?.id ?? ''
  durationSeconds.value = selectedModel.value?.min_duration_seconds ?? null
}

function onModelChange(): void {
  durationSeconds.value = selectedModel.value?.min_duration_seconds ?? null
}

async function generate(scene: Scene): Promise<void> {
  if (!plan.value || !selectedProviderId.value || !selectedModelId.value) return
  errorMessage.value = ''
  submittingSceneKey.value = scene.key
  try {
    const duration = durationSeconds.value ?? Math.round(scene.expected_duration_seconds)
    if (duration < durationMin.value || duration > durationMax.value) {
      throw new Error(`Duration must be between ${durationMin.value} and ${durationMax.value} seconds for the selected model.`)
    }
    const job = await createSceneVideoGeneration(projectId.value, plan.value.version, scene.key, {
      request_id: crypto.randomUUID(),
      provider_id: selectedProviderId.value,
      model_id: selectedModelId.value,
      duration_seconds: duration,
      assign_primary_visual: false,
    })
    setJob(scene.key, job)
    schedulePolling()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Unable to start video generation.'
  } finally {
    submittingSceneKey.value = null
  }
}

async function assign(sceneKey: string, job: SceneVideoJobView): Promise<void> {
  if (!plan.value || !job.media_asset_id) return
  assigningJobId.value = job.id
  errorMessage.value = ''
  try {
    await assignPrimaryVisual(projectId.value, plan.value.version, sceneKey, job.media_asset_id)
    setJob(sceneKey, { ...job, assigned_primary_visual: true })
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Unable to assign generated video.'
  } finally {
    assigningJobId.value = null
  }
}

async function refreshPendingJobs(): Promise<void> {
  const pending: Array<{ sceneKey: string; job: SceneVideoJobView }> = []
  for (const [sceneKey, sceneJobs] of Object.entries(jobs.value)) {
    for (const job of sceneJobs) {
      if (job.state === 'queued' || job.state === 'running') pending.push({ sceneKey, job })
    }
  }
  await Promise.all(
    pending.map(async ({ sceneKey, job }) => {
      try {
        const refreshed = await getSceneVideoGeneration(projectId.value, job.id)
        setJob(sceneKey, refreshed)
      } catch {
        // Keep the durable local job reference so a later refresh can recover it.
      }
    }),
  )
}

function hasPendingJobs(): boolean {
  return Object.values(jobs.value).some((sceneJobs) =>
    sceneJobs.some((job) => job.state === 'queued' || job.state === 'running'),
  )
}

function schedulePolling(): void {
  if (pollTimer !== undefined || !hasPendingJobs()) return
  pollTimer = window.setTimeout(async () => {
    pollTimer = undefined
    await refreshPendingJobs()
    schedulePolling()
  }, 3000)
}

function jobLabel(job: SceneVideoJobView): string {
  if (job.state === 'failed') return job.error_code ? `Failed · ${job.error_code}` : 'Failed'
  if (job.state === 'succeeded') return job.assigned_primary_visual ? 'Succeeded · assigned' : 'Succeeded'
  return job.state === 'running' ? 'Generating…' : 'Queued'
}

onMounted(load)
onBeforeUnmount(() => {
  if (pollTimer !== undefined) window.clearTimeout(pollTimer)
})
</script>

<template>
  <main class="scene-video-workspace">
    <header class="scene-video-header">
      <div>
        <p class="eyebrow">Scene video</p>
        <h1>AI video generation</h1>
        <p>Generate alternatives per approved scene, recover status after refresh, preview results, then explicitly assign the chosen video.</p>
      </div>
    </header>

    <p v-if="errorMessage" role="alert" class="error-banner">{{ errorMessage }}</p>
    <p v-if="loading">Loading video generation workspace…</p>

    <template v-else-if="plan">
      <section class="generation-controls" aria-label="Video generation controls">
        <label>
          Provider
          <select v-model="selectedProviderId" @change="onProviderChange">
            <option v-for="provider in providers" :key="provider.id" :value="provider.id">
              {{ provider.display_name }}
            </option>
          </select>
        </label>
        <label>
          Model
          <select v-model="selectedModelId" @change="onModelChange">
            <option v-for="model in selectedProvider?.models ?? []" :key="model.id" :value="model.id">
              {{ model.display_name }}
            </option>
          </select>
        </label>
        <label>
          Duration (seconds)
          <input v-model.number="durationSeconds" type="number" :min="durationMin" :max="durationMax" />
          <small>{{ durationMin }}–{{ durationMax }}s supported by the selected model.</small>
        </label>
      </section>

      <section class="scene-list">
        <article v-for="scene in plan.scenes" :key="scene.key" class="scene-card">
          <div class="scene-copy">
            <h2>{{ scene.key }}</h2>
            <p>{{ scene.visual_instruction }}</p>
            <button
              type="button"
              :disabled="!selectedModelId || submittingSceneKey === scene.key"
              @click="generate(scene)"
            >
              {{ submittingSceneKey === scene.key ? 'Submitting…' : 'Generate alternative' }}
            </button>
          </div>

          <div class="alternatives">
            <p v-if="!(jobs[scene.key]?.length)">No generated alternatives yet.</p>
            <article v-for="job in jobs[scene.key] ?? []" :key="job.id" class="alternative-card">
              <div class="alternative-meta">
                <strong>{{ jobLabel(job) }}</strong>
                <span>Attempt {{ job.attempt }}/{{ job.max_attempts }}</span>
              </div>
              <video
                v-if="job.state === 'succeeded' && job.media_asset_id"
                controls
                preload="metadata"
                :src="mediaAssetContentURL(projectId, job.media_asset_id)"
              />
              <button
                v-if="job.state === 'succeeded' && job.media_asset_id"
                type="button"
                :disabled="job.assigned_primary_visual || assigningJobId === job.id"
                @click="assign(scene.key, job)"
              >
                {{ job.assigned_primary_visual ? 'Assigned to scene' : assigningJobId === job.id ? 'Assigning…' : 'Use as primary visual' }}
              </button>
            </article>
          </div>
        </article>
      </section>
    </template>
  </main>
</template>

<style scoped>
.scene-video-workspace {
  max-width: 1120px;
  margin: 0 auto;
  padding: 32px 24px 64px;
}
.scene-video-header,
.generation-controls,
.scene-card,
.alternative-card {
  border: 1px solid var(--border-color, #d9dde5);
  border-radius: 14px;
  background: var(--surface-color, #fff);
}
.scene-video-header,
.generation-controls,
.scene-card {
  padding: 20px;
}
.eyebrow {
  margin: 0 0 4px;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}
.generation-controls {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
  margin: 20px 0;
}
.generation-controls label {
  display: grid;
  gap: 6px;
}
.generation-controls select,
.generation-controls input,
button {
  min-height: 40px;
}
.scene-list {
  display: grid;
  gap: 20px;
}
.scene-card {
  display: grid;
  grid-template-columns: minmax(0, 0.8fr) minmax(0, 1.2fr);
  gap: 20px;
}
.alternatives {
  display: grid;
  gap: 12px;
}
.alternative-card {
  padding: 12px;
}
.alternative-meta {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 10px;
}
video {
  display: block;
  width: 100%;
  max-height: 360px;
  margin-bottom: 10px;
  background: #000;
}
.error-banner {
  padding: 12px 16px;
  border: 1px solid currentColor;
  border-radius: 10px;
}
@media (max-width: 760px) {
  .generation-controls,
  .scene-card {
    grid-template-columns: 1fr;
  }
}
</style>
