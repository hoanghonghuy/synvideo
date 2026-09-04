<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'

import { getProject, type Project } from '@/api/projects'
import {
  assignPrimaryVisual,
  listSceneMediaBindings,
  mediaAssetContentURL,
  type SceneMediaEntry,
} from '@/features/media/api'
import { getScenePlan, listScenePlans, type Scene, type ScenePlan } from '@/features/scene-plan/api'
import {
  createSceneImageGeneration,
  fetchImageGenerationOptions,
  GeneratedImageApiError,
  getSceneImageGeneration,
  type ImageGenerationOptionProvider,
  type SceneImageGenerationJobView,
} from './api'
import messages from './messages'

interface PendingGeneration {
  requestId: string
  providerId?: string
  modelId?: string
  prompt?: string
  assignPrimaryVisual?: boolean
}

interface SceneGenerationState {
  prompt: string
  submitted?: PendingGeneration
  jobState?: SceneImageGenerationJobView['state'] | 'recoverable'
  assetId?: string
  assigned: boolean
  keptAlternative: boolean
  errorCode?: string
  busy: boolean
}

const route = useRoute()
const { t } = useI18n({ useScope: 'local', messages })

const project = ref<Project | null>(null)
const plan = ref<ScenePlan | null>(null)
const providers = ref<ImageGenerationOptionProvider[]>([])
const selectedProviderId = ref('')
const selectedModelId = ref('')
const assignOnSuccess = ref(false)
const loading = ref(true)
const loadFailed = ref(false)
const sceneStates = reactive<Record<string, SceneGenerationState>>({})
const bindings = reactive<Record<string, SceneMediaEntry>>({})
const pollTimers = new Map<string, ReturnType<typeof setTimeout>>()

const projectId = computed(() => String(route.params.id ?? ''))
const selectedProvider = computed(() =>
  providers.value.find((provider) => provider.id === selectedProviderId.value),
)
const selectedModels = computed(() => selectedProvider.value?.models ?? [])

watch(selectedProviderId, () => {
  if (!selectedModels.value.some((model) => model.id === selectedModelId.value)) {
    selectedModelId.value = selectedModels.value[0]?.id ?? ''
  }
})

onMounted(() => {
  void loadWorkspace()
})

onBeforeUnmount(() => {
  for (const timer of pollTimers.values()) clearTimeout(timer)
  pollTimers.clear()
})

async function loadWorkspace() {
  loading.value = true
  loadFailed.value = false
  try {
    const [loadedProject, summaries, optionResponse] = await Promise.all([
      getProject(projectId.value),
      listScenePlans(projectId.value),
      fetchImageGenerationOptions(),
    ])
    project.value = loadedProject
    providers.value = optionResponse.providers
    selectedProviderId.value = providers.value[0]?.id ?? ''
    selectedModelId.value = providers.value[0]?.models[0]?.id ?? ''

    const approved = summaries
      .filter((summary) => summary.status === 'approved')
      .sort((a, b) => b.version - a.version)[0]
    if (!approved) return

    const [loadedPlan, currentBindings] = await Promise.all([
      getScenePlan(projectId.value, approved.version),
      listSceneMediaBindings(projectId.value, approved.version),
    ])
    plan.value = loadedPlan
    for (const entry of currentBindings) bindings[entry.scene_key] = entry

    const recoveries: Promise<void>[] = []
    for (const scene of loadedPlan.scenes) {
      sceneStates[scene.key] = {
        prompt: scene.visual_instruction,
        assigned: false,
        keptAlternative: false,
        busy: false,
      }
      const pending = readPending(scene.key)
      if (pending) {
        sceneStates[scene.key].submitted = pending
        if (pending.prompt) sceneStates[scene.key].prompt = pending.prompt
        recoveries.push(recoverScene(scene, pending))
      }
    }
    await Promise.all(recoveries)
  } catch {
    loadFailed.value = true
  } finally {
    loading.value = false
  }
}

function stateFor(sceneKey: string): SceneGenerationState {
  return sceneStates[sceneKey] ?? {
    prompt: '',
    assigned: false,
    keptAlternative: false,
    busy: false,
  }
}

function storageKey(sceneKey: string): string {
  return `synvideo:generated-image:${projectId.value}:${plan.value?.version ?? 0}:${sceneKey}`
}

function readPending(sceneKey: string): PendingGeneration | undefined {
  try {
    const raw = sessionStorage.getItem(storageKey(sceneKey))
    if (!raw) return undefined
    const parsed = JSON.parse(raw) as PendingGeneration
    return typeof parsed.requestId === 'string' && parsed.requestId ? parsed : undefined
  } catch {
    return undefined
  }
}

function persistPending(sceneKey: string, pending: PendingGeneration) {
  sessionStorage.setItem(storageKey(sceneKey), JSON.stringify(pending))
}

function makeRequestId(): string {
  return crypto.randomUUID()
}

async function generate(scene: Scene) {
  const state = stateFor(scene.key)
  if (!selectedProviderId.value || !selectedModelId.value || !state.prompt.trim()) return

  const pending: PendingGeneration = {
    requestId: makeRequestId(),
    providerId: selectedProviderId.value,
    modelId: selectedModelId.value,
    prompt: state.prompt.trim(),
    assignPrimaryVisual: assignOnSuccess.value,
  }
  state.submitted = pending
  state.assetId = undefined
  state.assigned = false
  state.keptAlternative = false
  state.errorCode = undefined
  state.jobState = undefined
  persistPending(scene.key, pending)
  await submitPending(scene, pending)
}

async function retrySameRequest(scene: Scene) {
  const state = stateFor(scene.key)
  if (!state.submitted) return
  await submitPending(scene, normalizePending(scene, state.submitted))
}

function normalizePending(scene: Scene, pending: PendingGeneration): Required<PendingGeneration> {
  return {
    requestId: pending.requestId,
    providerId: pending.providerId ?? selectedProviderId.value,
    modelId: pending.modelId ?? selectedModelId.value,
    prompt: pending.prompt ?? stateFor(scene.key).prompt,
    assignPrimaryVisual: pending.assignPrimaryVisual ?? false,
  }
}

async function submitPending(scene: Scene, rawPending: PendingGeneration) {
  const state = stateFor(scene.key)
  const pending = normalizePending(scene, rawPending)
  state.submitted = pending
  state.busy = true
  state.errorCode = undefined
  persistPending(scene.key, pending)
  try {
    const job = await createSceneImageGeneration(projectId.value, plan.value!.version, scene.key, {
      request_id: pending.requestId,
      provider_id: pending.providerId,
      model_id: pending.modelId,
      prompt: pending.prompt,
      assign_primary_visual: pending.assignPrimaryVisual,
    })
    applyJob(scene.key, job)
    if (job.state === 'queued' || job.state === 'running') await refreshJob(scene)
  } catch (error) {
    state.jobState = 'recoverable'
    state.errorCode = apiErrorCode(error)
  } finally {
    state.busy = false
  }
}

async function recoverScene(scene: Scene, pending: PendingGeneration) {
  const state = stateFor(scene.key)
  state.busy = true
  try {
    const job = await getSceneImageGeneration(projectId.value, pending.requestId)
    applyJob(scene.key, job)
    if (job.state === 'queued' || job.state === 'running') schedulePoll(scene)
  } catch (error) {
    state.jobState = 'recoverable'
    state.errorCode = apiErrorCode(error)
  } finally {
    state.busy = false
  }
}

async function refreshJob(scene: Scene) {
  const state = stateFor(scene.key)
  const requestId = state.submitted?.requestId
  if (!requestId) return
  try {
    const job = await getSceneImageGeneration(projectId.value, requestId)
    applyJob(scene.key, job)
    if (job.state === 'queued' || job.state === 'running') schedulePoll(scene)
  } catch (error) {
    state.jobState = 'recoverable'
    state.errorCode = apiErrorCode(error)
  }
}

function applyJob(sceneKey: string, job: SceneImageGenerationJobView) {
  const state = stateFor(sceneKey)
  state.jobState = job.state
  state.errorCode = job.error_code
  state.assetId = job.media_asset_id
  state.assigned = job.assigned_primary_visual
  if (job.media_asset_id && job.state === 'succeeded') state.keptAlternative = !job.assigned_primary_visual
}

function schedulePoll(scene: Scene) {
  const previous = pollTimers.get(scene.key)
  if (previous) clearTimeout(previous)
  pollTimers.set(
    scene.key,
    setTimeout(() => {
      void refreshJob(scene)
    }, 1500),
  )
}

async function assignGenerated(scene: Scene) {
  const state = stateFor(scene.key)
  if (!plan.value || !state.assetId) return
  state.busy = true
  state.errorCode = undefined
  try {
    const entry = await assignPrimaryVisual(projectId.value, plan.value.version, scene.key, state.assetId)
    bindings[scene.key] = entry
    state.assigned = true
    state.keptAlternative = false
  } catch (error) {
    state.errorCode = apiErrorCode(error)
  } finally {
    state.busy = false
  }
}

function keepAlternative(sceneKey: string) {
  const state = stateFor(sceneKey)
  state.keptAlternative = true
  state.assigned = false
}

function apiErrorCode(error: unknown): string {
  if (error instanceof GeneratedImageApiError) {
    if (error.code === 'GENERATION_PROVIDER_UNAVAILABLE') return 'providerUnavailable'
    if (error.code === 'GENERATION_REQUEST_CONFLICT') return 'requestConflict'
  }
  return 'requestFailed'
}

function jobLabel(state: SceneGenerationState): string {
  if (state.jobState === 'queued') return t('generatedImage.queued')
  if (state.jobState === 'running') return t('generatedImage.running')
  if (state.jobState === 'succeeded') return t('generatedImage.succeeded')
  if (state.jobState === 'failed') return t('generatedImage.failed')
  return ''
}
</script>

<template>
  <section class="page generated-image-workspace">
    <RouterLink class="text-link" :to="`/projects/${projectId}`">
      {{ t('generatedImage.backToProject') }}
    </RouterLink>

    <p class="eyebrow">{{ t('generatedImage.eyebrow') }}</p>
    <h1>{{ t('generatedImage.title') }}</h1>
    <p class="body-copy">{{ t('generatedImage.description') }}</p>

    <p v-if="loading" class="state-text">...</p>
    <div v-else-if="loadFailed" class="notice error" role="alert">
      {{ t('generatedImage.loadFailed') }}
    </div>
    <div v-else-if="!plan" class="notice">
      <p>{{ t('generatedImage.noApprovedPlan') }}</p>
      <RouterLink class="text-link" :to="`/projects/${projectId}/scene-plan`">
        {{ t('generatedImage.openScenePlan') }}
      </RouterLink>
    </div>

    <template v-else>
      <div v-if="providers.length" class="generation-settings">
        <label>
          <span>{{ t('generatedImage.provider') }}</span>
          <select v-model="selectedProviderId">
            <option v-for="provider in providers" :key="provider.id" :value="provider.id">
              {{ provider.display_name }}
            </option>
          </select>
        </label>
        <label>
          <span>{{ t('generatedImage.model') }}</span>
          <select v-model="selectedModelId">
            <option v-for="model in selectedModels" :key="model.id" :value="model.id">
              {{ model.display_name }}
            </option>
          </select>
        </label>
        <label class="check-field">
          <input v-model="assignOnSuccess" type="checkbox" />
          <span>{{ t('generatedImage.assignOnSuccess') }}</span>
        </label>
      </div>
      <div v-else class="notice">
        <p>{{ t('generatedImage.noProviders') }}</p>
        <RouterLink class="text-link" to="/settings/ai-providers">
          {{ t('generatedImage.openProviderSettings') }}
        </RouterLink>
      </div>

      <div class="scene-grid">
        <article v-for="scene in plan.scenes" :key="scene.key" class="scene-card">
          <header>
            <p class="eyebrow">{{ t('generatedImage.scene', { key: scene.key }) }}</p>
            <p class="scene-copy">{{ scene.narration }}</p>
          </header>

          <label class="prompt-field">
            <span>{{ t('generatedImage.prompt') }}</span>
            <textarea
              v-model="stateFor(scene.key).prompt"
              :data-testid="`image-prompt-${scene.key}`"
              rows="5"
              :disabled="stateFor(scene.key).jobState === 'queued' || stateFor(scene.key).jobState === 'running'"
            />
          </label>

          <div v-if="bindings[scene.key]?.asset" class="preview-block">
            <strong>{{ t('generatedImage.currentVisual') }}</strong>
            <img
              :src="mediaAssetContentURL(projectId, bindings[scene.key].asset!.id)"
              :alt="t('generatedImage.currentVisual')"
            />
          </div>
          <p v-else class="state-text">{{ t('generatedImage.noVisual') }}</p>

          <div v-if="stateFor(scene.key).assetId" class="preview-block">
            <strong>{{ t('generatedImage.generatedPreview') }}</strong>
            <img
              :data-testid="`generated-image-${scene.key}`"
              :src="mediaAssetContentURL(projectId, stateFor(scene.key).assetId!)"
              :alt="t('generatedImage.generatedPreview')"
            />
            <div class="action-row">
              <button
                v-if="!stateFor(scene.key).assigned"
                class="secondary-button"
                type="button"
                :disabled="stateFor(scene.key).busy"
                @click="assignGenerated(scene)"
              >
                {{ t('generatedImage.assign') }}
              </button>
              <span v-else class="success-text">{{ t('generatedImage.assigned') }}</span>
              <button
                v-if="!stateFor(scene.key).assigned"
                class="secondary-button"
                type="button"
                @click="keepAlternative(scene.key)"
              >
                {{ t('generatedImage.keepAlternative') }}
              </button>
            </div>
          </div>

          <p v-if="jobLabel(stateFor(scene.key))" aria-live="polite" class="state-text">
            {{ jobLabel(stateFor(scene.key)) }}
          </p>
          <div v-if="stateFor(scene.key).errorCode" class="notice error" role="alert">
            {{ t(`generatedImage.${stateFor(scene.key).errorCode}`) }}
          </div>

          <div class="action-row">
            <button
              class="primary-button"
              type="button"
              :data-testid="`generate-image-${scene.key}`"
              :disabled="
                !providers.length ||
                !stateFor(scene.key).prompt.trim() ||
                stateFor(scene.key).busy ||
                stateFor(scene.key).jobState === 'queued' ||
                stateFor(scene.key).jobState === 'running'
              "
              @click="generate(scene)"
            >
              {{
                stateFor(scene.key).busy
                  ? t('generatedImage.generating')
                  : stateFor(scene.key).assetId
                    ? t('generatedImage.regenerate')
                    : t('generatedImage.generate')
              }}
            </button>
            <button
              v-if="stateFor(scene.key).jobState === 'recoverable' && stateFor(scene.key).submitted"
              class="secondary-button"
              type="button"
              :disabled="stateFor(scene.key).busy"
              @click="retrySameRequest(scene)"
            >
              {{ t('generatedImage.retrySameRequest') }}
            </button>
          </div>
        </article>
      </div>
    </template>
  </section>
</template>

<style scoped>
.generated-image-workspace {
  display: grid;
  gap: 1rem;
}

.generation-settings {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(13rem, 1fr));
  gap: 1rem;
  padding: 1rem;
  border: 1px solid var(--color-border, #d7d7d7);
  border-radius: 0.75rem;
}

.generation-settings label,
.prompt-field {
  display: grid;
  gap: 0.4rem;
}

.check-field {
  display: flex !important;
  align-items: center;
  gap: 0.5rem !important;
}

select,
textarea {
  width: 100%;
  box-sizing: border-box;
  padding: 0.65rem;
  border: 1px solid var(--color-border, #c8c8c8);
  border-radius: 0.5rem;
  background: var(--color-surface, #fff);
  color: inherit;
  font: inherit;
}

.scene-grid {
  display: grid;
  gap: 1rem;
}

.scene-card {
  display: grid;
  gap: 1rem;
  padding: 1rem;
  border: 1px solid var(--color-border, #d7d7d7);
  border-radius: 0.75rem;
}

.scene-copy,
.state-text {
  margin: 0;
}

.preview-block {
  display: grid;
  gap: 0.6rem;
}

.preview-block img {
  width: min(100%, 42rem);
  max-height: 28rem;
  object-fit: contain;
  border-radius: 0.6rem;
  background: #111;
}

.action-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.6rem;
  align-items: center;
}

.success-text {
  font-weight: 600;
}

@media (max-width: 640px) {
  .generation-settings {
    grid-template-columns: 1fr;
  }

  .action-row > button {
    width: 100%;
  }
}
</style>
