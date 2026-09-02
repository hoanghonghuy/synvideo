<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'

import { ApiError, getProject, type Project } from '@/api/projects'
import {
  getTextGenerationOptions,
  listScripts,
  type ScriptSummary,
  type TextGenerationOptionProvider,
} from '@/features/script/api'
import {
  approveScenePlan,
  createScenePlanGeneration,
  getScenePlan,
  getScenePlanGeneration,
  listScenePlans,
  putScenePlan,
  type Scene,
  type ScenePlan,
  type ScenePlanGenerationJob,
  type ScenePlanSummary,
} from './api'

const { t, d } = useI18n()
const route = useRoute()

const project = ref<Project | null>(null)
const summaries = ref<ScenePlanSummary[]>([])
const scripts = ref<ScriptSummary[]>([])
const selectedPlan = ref<ScenePlan | null>(null)
const selectedVersion = ref<number | null>(null)
const pendingVersion = ref<number | null>(null)
const formScenes = ref<Scene[]>([])
const savedSnapshot = ref('')
const loading = ref(true)
const plansLoaded = ref(false)
const planLoading = ref(false)
const saving = ref(false)
const approving = ref(false)
const saved = ref(false)
const dirty = ref(false)
const confirmApproval = ref(false)
const loadErrorCode = ref('')
const mutationErrorCode = ref('')
const confirmStaleReload = ref(false)
const failedVersion = ref<number | null>(null)
const failedTargetVersion = ref<number | null>(null)
const fieldErrors = ref<Record<string, string>>({})

let currentWorkspaceLoadSeq = 0
let currentVersionLoadSeq = 0

const providerOptions = ref<TextGenerationOptionProvider[]>([])
const optionsLoading = ref(false)
const optionsErrorCode = ref('')
const selectedProviderId = ref('')
const selectedModelId = ref('')
const activeJob = ref<ScenePlanGenerationJob | null>(null)
const generating = ref(false)
const generationErrorCode = ref('')
const pollErrorCode = ref('')
let pollTimer: number | null = null

// Split modal state
const splitTargetIndex = ref<number | null>(null)
const splitPoint = ref(1)
const splitNewKey = ref('')
const splitError = ref('')
const splitTargetScene = computed(() => {
  if (splitTargetIndex.value === null) return null
  return formScenes.value[splitTargetIndex.value] ?? null
})

const splitMaxCodePoints = computed(() => {
  if (!splitTargetScene.value) return 1
  return Math.max(1, Array.from(splitTargetScene.value.narration).length - 1)
})

const splitPart1Preview = computed(() => {
  if (!splitTargetScene.value) return ''
  const codePoints = Array.from(splitTargetScene.value.narration)
  return codePoints.slice(0, splitPoint.value).join('')
})

const splitPart2Preview = computed(() => {
  if (!splitTargetScene.value) return ''
  const codePoints = Array.from(splitTargetScene.value.narration)
  return codePoints.slice(splitPoint.value).join('')
})

const selectedProviderModels = computed(() => {
  return providerOptions.value.find((provider) => provider.id === selectedProviderId.value)?.models ?? []
})
const hasProviders = computed(() => providerOptions.value.some((provider) => provider.models.length > 0))
const approvedScriptVersion = computed(() => {
  const approved = scripts.value.filter((script) => script.status === 'approved')
  return approved.reduce((highest, script) => Math.max(highest, script.version), 0) || null
})
const hasApprovedScript = computed(() => approvedScriptVersion.value !== null)
const hasScenePlan = computed(() => summaries.value.length > 0)
const isReadOnly = computed(() => selectedPlan.value?.status !== 'draft')
const generationInProgress = computed(
  () => activeJob.value?.state === 'queued' || activeJob.value?.state === 'running',
)
const succeededJobUnloaded = computed(() => activeJob.value?.state === 'succeeded')
const canGenerate = computed(() => {
  return (
    hasApprovedScript.value &&
    hasProviders.value &&
    Boolean(selectedModelId.value) &&
    !dirty.value &&
    !generationInProgress.value &&
    !generating.value &&
    !succeededJobUnloaded.value
  )
})
const staleSource = computed(() => {
  return (
    selectedPlan.value !== null &&
    approvedScriptVersion.value !== null &&
    approvedScriptVersion.value > selectedPlan.value.source_script_version
  )
})

const unmappedFieldErrors = computed(() => {
  const knownKeys = new Set<string>()
  formScenes.value.forEach((_, idx) => {
    knownKeys.add(`scenes[${idx}].visual_instruction`)
    knownKeys.add(`scenes.${idx}.visual_instruction`)
    knownKeys.add(`scenes[${idx}].planned_source_type`)
    knownKeys.add(`scenes.${idx}.planned_source_type`)
    knownKeys.add(`scenes[${idx}].expected_duration_seconds`)
    knownKeys.add(`scenes.${idx}.expected_duration_seconds`)
    knownKeys.add(`scenes[${idx}].caption_intent`)
    knownKeys.add(`scenes.${idx}.caption_intent`)
    knownKeys.add(`scenes[${idx}].transition_notes`)
    knownKeys.add(`scenes.${idx}.transition_notes`)
    knownKeys.add(`scenes[${idx}].key`)
    knownKeys.add(`scenes.${idx}.key`)
    knownKeys.add(`scenes[${idx}].narration`)
    knownKeys.add(`scenes.${idx}.narration`)
    knownKeys.add(`scenes[${idx}]`)
  })
  return Object.entries(fieldErrors.value)
    .filter(([k]) => !knownKeys.has(k))
    .map(([k, v]) => `${k}: ${v}`)
})

watch(
  formScenes,
  () => {
    dirty.value = JSON.stringify(formScenes.value) !== savedSnapshot.value
    if (dirty.value) {
      saved.value = false
      confirmApproval.value = false
    }
  },
  { deep: true },
)

onMounted(() => {
  void loadWorkspace()
  void loadGenerationOptions()
  resumeActiveJob()
})

onUnmounted(() => {
  stopPolling()
})

async function loadGenerationOptions() {
  optionsLoading.value = true
  optionsErrorCode.value = ''
  try {
    const response = await getTextGenerationOptions()
    providerOptions.value = response.providers ?? []
    if (!selectedProviderId.value) {
      const provider = providerOptions.value.find((item) => item.models.length > 0)
      if (provider) {
        selectedProviderId.value = provider.id
        selectedModelId.value = provider.models[0]?.id ?? ''
      }
    }
  } catch (error) {
    providerOptions.value = []
    optionsErrorCode.value = error instanceof ApiError ? error.code : 'request_failed'
  } finally {
    optionsLoading.value = false
  }
}

function onProviderChange(providerID: string) {
  selectedProviderId.value = providerID
  selectedModelId.value =
    providerOptions.value.find((provider) => provider.id === providerID)?.models[0]?.id ?? ''
}

async function loadWorkspace() {
  loading.value = true
  loadErrorCode.value = ''
  mutationErrorCode.value = ''
  failedVersion.value = null
  failedTargetVersion.value = null
  plansLoaded.value = false
  const seq = ++currentWorkspaceLoadSeq
  try {
    project.value = await getProject(String(route.params.id))
    const [fetchedPlans, fetchedScripts] = await Promise.all([
      listScenePlans(project.value.id),
      listScripts(project.value.id),
    ])
    if (seq !== currentWorkspaceLoadSeq) return
    summaries.value = fetchedPlans
    scripts.value = fetchedScripts
    plansLoaded.value = true

    // If an active succeeded job is waiting or loaded, keep its version
    if (activeJob.value?.state === 'succeeded' && activeJob.value.scene_plan_version !== null) {
      if (selectedVersion.value === activeJob.value.scene_plan_version && selectedPlan.value) {
        return
      }
      const loaded = await loadVersion(activeJob.value.scene_plan_version, false)
      if (loaded) {
        removePersistedJob()
        activeJob.value = null
      }
      return
    }

    const initial = summaries.value.find((item) => item.status === 'draft') ?? summaries.value[0]
    if (initial) {
      await loadVersion(initial.version, true)
    } else {
      selectedPlan.value = null
      selectedVersion.value = null
      formScenes.value = []
      savedSnapshot.value = JSON.stringify([])
      dirty.value = false
    }
  } catch (error) {
    if (seq === currentWorkspaceLoadSeq) {
      loadErrorCode.value = error instanceof ApiError ? error.code : 'request_failed'
    }
  } finally {
    if (seq === currentWorkspaceLoadSeq) {
      loading.value = false
    }
  }
}

async function loadVersion(version: number, discardDirty = false): Promise<boolean> {
  if (dirty.value && !discardDirty && selectedVersion.value !== version) {
    pendingVersion.value = version
    return false
  }
  const seq = ++currentVersionLoadSeq
  const startSnapshot = JSON.stringify(formScenes.value)
  planLoading.value = true
  loadErrorCode.value = ''
  mutationErrorCode.value = ''
  fieldErrors.value = {}
  confirmStaleReload.value = false
  try {
    const plan = await getScenePlan(projectID(), version)
    if (seq !== currentVersionLoadSeq) {
      return false
    }
    // Protect any fresh edits made while this request was in-flight
    const formMutatedDuringRequest = JSON.stringify(formScenes.value) !== startSnapshot
    if (formMutatedDuringRequest || (dirty.value && !discardDirty)) {
      pendingVersion.value = version
      return false
    }
    applyPlan(plan)
    return true
  } catch (error) {
    if (seq !== currentVersionLoadSeq) return false
    loadErrorCode.value = error instanceof ApiError ? error.code : 'request_failed'
    failedVersion.value = version
    return false
  } finally {
    if (seq === currentVersionLoadSeq) {
      planLoading.value = false
    }
  }
}

function applyPlan(plan: ScenePlan) {
  selectedPlan.value = plan
  selectedVersion.value = plan.version
  formScenes.value = plan.scenes.map((s) => ({ ...s }))
  savedSnapshot.value = JSON.stringify(formScenes.value)
  dirty.value = false
  saved.value = false
  pendingVersion.value = null
  failedVersion.value = null
  confirmApproval.value = false
}

async function saveScenePlan() {
  if (!selectedPlan.value || selectedPlan.value.status !== 'draft' || !dirty.value) return
  saving.value = true
  mutationErrorCode.value = ''
  loadErrorCode.value = ''
  fieldErrors.value = {}
  try {
    const updated = await putScenePlan(projectID(), selectedPlan.value.version, {
      revision: selectedPlan.value.revision,
      scenes: formScenes.value.map((s) => ({ ...s })),
    })
    applyPlan(updated)
    saved.value = true
    upsertSummary(updated)
  } catch (error) {
    handleMutationError(error)
  } finally {
    saving.value = false
  }
}

async function saveAndSwitch() {
  if (pendingVersion.value === null || !selectedPlan.value || selectedPlan.value.status !== 'draft') return
  const targetVersion = pendingVersion.value
  saving.value = true
  mutationErrorCode.value = ''
  loadErrorCode.value = ''
  fieldErrors.value = {}
  try {
    const updated = await putScenePlan(projectID(), selectedPlan.value.version, {
      revision: selectedPlan.value.revision,
      scenes: formScenes.value.map((s) => ({ ...s })),
    })
    applyPlan(updated)
    saved.value = true
    upsertSummary(updated)
    pendingVersion.value = null

    const loaded = await loadVersion(targetVersion, true)
    if (!loaded) {
      if (pendingVersion.value === null) {
        failedTargetVersion.value = targetVersion
      }
      return
    }
    failedTargetVersion.value = null
    if (
      activeJob.value?.state === 'succeeded' &&
      activeJob.value.scene_plan_version === targetVersion
    ) {
      removePersistedJob()
      activeJob.value = null
      generationErrorCode.value = ''
    }
  } catch (error) {
    handleMutationError(error)
  } finally {
    saving.value = false
  }
}

async function retryTargetVersion() {
  if (failedTargetVersion.value === null) return
  const target = failedTargetVersion.value
  const loaded = await loadVersion(target, true)
  if (loaded) {
    failedTargetVersion.value = null
    if (
      activeJob.value?.state === 'succeeded' &&
      activeJob.value.scene_plan_version === target
    ) {
      removePersistedJob()
      activeJob.value = null
      generationErrorCode.value = ''
    }
  }
}

async function approveSelected() {
  if (!selectedPlan.value || selectedPlan.value.status !== 'draft' || dirty.value) return
  approving.value = true
  mutationErrorCode.value = ''
  try {
    const approved = await approveScenePlan(
      projectID(),
      selectedPlan.value.version,
      selectedPlan.value.revision,
    )
    summaries.value = await listScenePlans(projectID())
    applyPlan(approved)
    upsertSummary(approved)
  } catch (error) {
    handleMutationError(error)
  } finally {
    approving.value = false
  }
}

function handleMutationError(error: unknown) {
  if (error instanceof ApiError) {
    mutationErrorCode.value = error.code
    fieldErrors.value = error.fields
    return
  }
  mutationErrorCode.value = 'request_failed'
}

function requestID(): string | null {
  if (typeof crypto === 'undefined') return null
  if (typeof crypto.randomUUID === 'function') return crypto.randomUUID()
  if (typeof crypto.getRandomValues !== 'function') return null

  const bytes = new Uint8Array(16)
  crypto.getRandomValues(bytes)
  bytes[6] = ((bytes[6] ?? 0) & 0x0f) | 0x40
  bytes[8] = ((bytes[8] ?? 0) & 0x3f) | 0x80
  const hex = Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('')
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
}

async function startGeneration() {
  if (!canGenerate.value) return
  generating.value = true
  generationErrorCode.value = ''
  pollErrorCode.value = ''
  mutationErrorCode.value = ''
  try {
    const requestId = requestID()
    if (!requestId) {
      generationErrorCode.value = 'GENERATION_REQUEST_ID_UNAVAILABLE'
      return
    }
    const job = await createScenePlanGeneration(projectID(), {
      request_id: requestId,
      provider_id: selectedProviderId.value,
      model_id: selectedModelId.value,
    })
    activeJob.value = job
    persistJob(job.id)
    trackJob(job.id)
  } catch (error) {
    generationErrorCode.value = error instanceof ApiError ? error.code : 'request_failed'
  } finally {
    generating.value = false
  }
}

function trackJob(jobID: string) {
  stopPolling()
  const poll = async () => {
    try {
      const job = await getScenePlanGeneration(projectID(), jobID)
      activeJob.value = job
      pollErrorCode.value = ''
      generationErrorCode.value = ''
      if (job.state === 'succeeded') {
        await loadSucceededPlan()
        return
      }
      if (job.state === 'failed') {
        removePersistedJob()
        generationErrorCode.value = job.error_code ?? 'request_failed'
        return
      }
      schedulePoll(poll)
    } catch (error) {
      pollErrorCode.value = error instanceof ApiError ? error.code : 'request_failed'
      schedulePoll(poll)
    }
  }
  schedulePoll(poll)
}

async function loadSucceededPlan() {
  const job = activeJob.value
  if (!job || job.state !== 'succeeded' || job.scene_plan_version === null) {
    generationErrorCode.value = 'request_failed'
    return
  }
  try {
    summaries.value = await listScenePlans(projectID())
    plansLoaded.value = true
    if (dirty.value) {
      pendingVersion.value = job.scene_plan_version
      generationErrorCode.value = ''
      return
    }
    const loaded = await loadVersion(job.scene_plan_version, false)
    if (!loaded) {
      generationErrorCode.value = loadErrorCode.value || 'request_failed'
      return
    }
    removePersistedJob()
    activeJob.value = null
    generationErrorCode.value = ''
  } catch (error) {
    generationErrorCode.value = error instanceof ApiError ? error.code : 'request_failed'
  }
}

function resumeActiveJob() {
  if (typeof window === 'undefined' || !window.sessionStorage) return
  const jobID = window.sessionStorage.getItem(storageKey())
  if (jobID) trackJob(jobID)
}

function persistJob(jobID: string) {
  if (typeof window !== 'undefined' && window.sessionStorage) {
    window.sessionStorage.setItem(storageKey(), jobID)
  }
}

function removePersistedJob() {
  if (typeof window !== 'undefined' && window.sessionStorage) {
    window.sessionStorage.removeItem(storageKey())
  }
}

function storageKey() {
  return `scene_plan_job_${projectID()}`
}

function schedulePoll(poll: () => Promise<void>) {
  pollTimer = window.setTimeout(() => void poll(), 1000)
}

function stopPolling() {
  if (pollTimer !== null) {
    window.clearTimeout(pollTimer)
    pollTimer = null
  }
}

function chooseVersion(version: number) {
  void loadVersion(version)
}

async function discardAndSwitch() {
  if (pendingVersion.value === null) return
  const version = pendingVersion.value
  pendingVersion.value = null
  failedTargetVersion.value = null
  const loaded = await loadVersion(version, true)
  if (
    loaded &&
    activeJob.value?.state === 'succeeded' &&
    activeJob.value.scene_plan_version === version
  ) {
    removePersistedJob()
    activeJob.value = null
    generationErrorCode.value = ''
  }
}

function cancelSwitch() {
  pendingVersion.value = null
  failedTargetVersion.value = null
}

function requestStaleReload() {
  confirmStaleReload.value = true
}

async function confirmStaleReloadAndDiscard() {
  confirmStaleReload.value = false
  if (selectedVersion.value !== null) await loadVersion(selectedVersion.value, true)
}

async function retryLoad() {
  if (failedVersion.value !== null) {
    await loadVersion(failedVersion.value, true)
  } else {
    await loadWorkspace()
  }
}

// Split logic
function openSplitModal(index: number) {
  const target = formScenes.value[index]
  if (!target) return
  splitTargetIndex.value = index
  const codePoints = Array.from(target.narration)
  splitPoint.value = Math.max(1, Math.floor(codePoints.length / 2))
  splitNewKey.value = generateUniqueKey(target.key)
  splitError.value = ''
}

function closeSplitModal() {
  splitTargetIndex.value = null
  splitError.value = ''
}

function generateUniqueKey(baseKey: string): string {
  const cleanBase = baseKey.replace(/-part-\d+$/, '')
  let candidate = `${cleanBase}-part-2`
  let counter = 2
  const existingKeys = new Set(formScenes.value.map((s) => s.key))
  while (existingKeys.has(candidate)) {
    counter++
    candidate = `${cleanBase}-part-${counter}`
  }
  return candidate
}

function confirmSplit() {
  if (splitTargetIndex.value === null) return
  const index = splitTargetIndex.value
  const target = formScenes.value[index]
  if (!target) return

  const codePoints = Array.from(target.narration)
  const point = Number(splitPoint.value)
  if (isNaN(point) || point < 1 || point >= codePoints.length) {
    splitError.value = t('scenePlan.validation.invalid_format')
    return
  }

  const trimmedKey = splitNewKey.value.trim()
  const keyRegex = /^[a-z0-9_-]+$/
  const keyRunes = Array.from(trimmedKey)
  if (!trimmedKey || keyRunes.length < 1 || keyRunes.length > 64 || !keyRegex.test(trimmedKey)) {
    splitError.value = t('scenePlan.validation.invalid_format')
    return
  }

  // A new scene key must not duplicate any existing scene key (including target scene key which will be retained)
  const isDuplicate = formScenes.value.some((s) => s.key === trimmedKey)
  if (isDuplicate) {
    splitError.value = t('scenePlan.validation.duplicate')
    return
  }

  const part1 = codePoints.slice(0, point).join('')
  const part2 = codePoints.slice(point).join('')

  const d1 = Math.max(1, Math.floor(target.expected_duration_seconds / 2))
  const d2 = Math.max(1, Math.ceil(target.expected_duration_seconds / 2))

  const newScene1: Scene = {
    ...target,
    narration: part1,
    expected_duration_seconds: d1,
  }

  const newScene2: Scene = {
    key: trimmedKey,
    script_section_key: target.script_section_key,
    narration: part2,
    visual_instruction: target.visual_instruction,
    planned_source_type: target.planned_source_type,
    expected_duration_seconds: d2,
    caption_intent: target.caption_intent,
    transition_notes: target.transition_notes,
  }

  formScenes.value.splice(index, 1, newScene1, newScene2)
  closeSplitModal()
}

// Merge logic
function canMergeWithNext(index: number): boolean {
  if (isReadOnly.value) return false
  if (index < 0 || index >= formScenes.value.length - 1) return false
  const current = formScenes.value[index]
  const next = formScenes.value[index + 1]
  return current !== undefined && next !== undefined && current.script_section_key === next.script_section_key
}

function mergeWithNext(index: number) {
  if (!canMergeWithNext(index)) return
  const current = formScenes.value[index]
  const next = formScenes.value[index + 1]
  if (!current || !next) return

  const mergedScene: Scene = {
    ...current,
    narration: current.narration + next.narration,
    expected_duration_seconds: current.expected_duration_seconds + next.expected_duration_seconds,
  }

  formScenes.value.splice(index, 2, mergedScene)
}

function getFieldError(idx: number, field: string): string | undefined {
  return (
    fieldErrors.value[`scenes[${idx}].${field}`] ??
    fieldErrors.value[`scenes.${idx}.${field}`] ??
    fieldErrors.value[`scenes[${idx}]`]
  )
}

function projectID() {
  return project.value?.id ?? String(route.params.id)
}

function upsertSummary(plan: ScenePlan) {
  const summary: ScenePlanSummary = {
    version: plan.version,
    revision: plan.revision,
    status: plan.status,
    source_script_version: plan.source_script_version,
    source_proposal_version: plan.source_proposal_version,
    content_locale: plan.content_locale,
    created_at: plan.created_at,
    updated_at: plan.updated_at,
    approved_at: plan.approved_at,
  }
  const index = summaries.value.findIndex((item) => item.version === plan.version)
  summaries.value =
    index === -1
      ? [summary, ...summaries.value]
      : summaries.value.map((item, i) => (i === index ? summary : item))
}

function errorMessage(code: string) {
  return t(`scenePlan.errors.${code}`)
}
</script>

<template>
  <section class="page scene-plan-page">
    <RouterLink
      class="text-link"
      :to="`/projects/${projectID()}`"
    >
      {{ t('scenePlan.actions.backToProject') }}
    </RouterLink>

    <div
      v-if="loading"
      class="state-text"
      data-testid="loading-state"
    >
      {{ t('scenePlan.states.loading') }}
    </div>

    <div
      v-else-if="loadErrorCode && !selectedPlan"
      class="notice error"
      data-testid="scene-plan-list-error"
    >
      <p>{{ errorMessage(loadErrorCode) }}</p>
      <button
        class="secondary-button"
        type="button"
        data-testid="retry-scene-plan-list"
        @click="retryLoad"
      >
        {{ t('projects.actions.retry') }}
      </button>
    </div>

    <template v-else>
      <div class="page-heading">
        <div>
          <p class="eyebrow">
            {{ t('scenePlan.eyebrow') }}
          </p>
          <h1>{{ project?.title ?? t('scenePlan.eyebrow') }}</h1>
          <p class="body-copy">
            {{ t('scenePlan.description') }}
          </p>
        </div>
      </div>

      <!-- Generation Bar -->
      <section
        class="scene-plan-generation-bar"
        aria-label="Generation"
      >
        <div
          v-if="optionsErrorCode"
          class="notice error"
          data-testid="options-error-notice"
        >
          <p>{{ errorMessage(optionsErrorCode) }}</p>
          <button
            class="secondary-button"
            type="button"
            data-testid="retry-options-btn"
            @click="loadGenerationOptions"
          >
            {{ t('scenePlan.actions.retryLoadOptions') }}
          </button>
        </div>

        <div
          v-else-if="!hasProviders"
          class="notice info"
          data-testid="no-scene-plan-providers-notice"
        >
          <p>{{ t('scenePlan.generation.noProvidersConfigured') }}</p>
          <RouterLink
            class="primary-button"
            to="/settings/ai-providers"
          >
            {{ t('scenePlan.generation.openProviderSettings') }}
          </RouterLink>
        </div>

        <div
          v-else-if="!hasApprovedScript"
          class="notice warning"
          data-testid="script-required-notice"
        >
          <p>{{ t('scenePlan.generation.scriptRequired') }}</p>
          <RouterLink
            class="secondary-button"
            :to="`/projects/${projectID()}/script`"
          >
            {{ t('scenePlan.generation.openScript') }}
          </RouterLink>
        </div>

        <div
          v-else
          class="generation-controls"
        >
          <div class="control-group">
            <label for="provider-select">{{ t('scenePlan.generation.selectProvider') }}</label>
            <select
              id="provider-select"
              :value="selectedProviderId"
              :disabled="generating || generationInProgress"
              @change="onProviderChange(($event.target as HTMLSelectElement).value)"
            >
              <option
                v-for="provider in providerOptions"
                :key="provider.id"
                :value="provider.id"
              >
                {{ provider.display_name }}
              </option>
            </select>
          </div>

          <div class="control-group">
            <label for="model-select">{{ t('scenePlan.generation.selectModel') }}</label>
            <select
              id="model-select"
              v-model="selectedModelId"
              :disabled="generating || generationInProgress"
            >
              <option
                v-for="model in selectedProviderModels"
                :key="model.id"
                :value="model.id"
              >
                {{ model.display_name }}
              </option>
            </select>
          </div>

          <button
            class="primary-button"
            type="button"
            data-testid="generate-scene-plan-btn"
            :disabled="!canGenerate"
            @click="startGeneration"
          >
            {{ generating ? t('scenePlan.actions.generating') : (hasScenePlan ? t('scenePlan.actions.regenerate') : t('scenePlan.actions.generate')) }}
          </button>
        </div>

        <!-- Generation Status Banners -->
        <div
          v-if="generationInProgress && !pollErrorCode"
          class="notice info"
          :data-testid="`job-state-${activeJob?.state}`"
        >
          <p>
            {{ activeJob?.state === 'queued' ? t('scenePlan.generation.stateQueued') : t('scenePlan.generation.stateRunning') }}
          </p>
        </div>

        <div
          v-if="activeJob?.state === 'succeeded' && !generationErrorCode"
          class="notice success"
          data-testid="job-state-succeeded"
        >
          <p>{{ t('scenePlan.generation.stateSucceeded') }}</p>
        </div>

        <!-- Transient Poll Error Banner (Never POSTs fresh generation) -->
        <div
          v-if="pollErrorCode"
          class="notice error"
          data-testid="job-error-banner"
        >
          <p>{{ errorMessage(pollErrorCode) }}</p>
          <button
            class="secondary-button"
            type="button"
            data-testid="retry-poll-btn"
            @click="activeJob && trackJob(activeJob.id)"
          >
            {{ t('scenePlan.actions.retryPoll') }}
          </button>
        </div>

        <!-- Terminal Error Banner -->
        <div
          v-else-if="generationErrorCode"
          class="notice error"
          data-testid="job-error-banner"
        >
          <p>{{ errorMessage(generationErrorCode) }}</p>
          <button
            v-if="activeJob?.state === 'succeeded'"
            class="secondary-button"
            type="button"
            data-testid="retry-load-generated-btn"
            @click="loadSucceededPlan"
          >
            {{ t('scenePlan.actions.retryLoadGenerated') }}
          </button>
          <button
            v-else
            class="secondary-button"
            type="button"
            data-testid="retry-generation-btn"
            @click="startGeneration"
          >
            {{ t('scenePlan.actions.retryGeneration') }}
          </button>
        </div>
      </section>

      <!-- Empty State -->
      <div
        v-if="!hasScenePlan && !loading"
        class="notice info"
        data-testid="scene-plan-empty-state"
      >
        <p><strong>{{ t('scenePlan.states.empty') }}</strong></p>
        <p>{{ t('scenePlan.states.emptyDetail') }}</p>
      </div>

      <!-- Main Workspace -->
      <div
        v-else-if="selectedPlan"
        class="scene-plan-workspace"
      >
        <!-- Version History Sidebar -->
        <aside
          class="scene-plan-history"
          :aria-label="t('scenePlan.history.label')"
        >
          <h2>{{ t('scenePlan.history.title') }}</h2>
          <div class="scene-plan-version-list">
            <button
              v-for="item in summaries"
              :key="item.version"
              :class="['scene-plan-version-button', { active: item.version === selectedVersion }]"
              :data-testid="`version-${item.version}`"
              type="button"
              @click="chooseVersion(item.version)"
            >
              <span>{{ t('scenePlan.states.version', { value: item.version }) }}</span>
              <small>{{ t(`scenePlan.status.${item.status}`) }}</small>
              <small>{{ t('scenePlan.states.sourceScriptVersion', { value: item.source_script_version }) }}</small>
              <small>{{ t('scenePlan.states.sourceProposalVersion', { value: item.source_proposal_version }) }}</small>
              <small>{{ d(new Date(item.updated_at), 'long') }}</small>
            </button>
          </div>
        </aside>

        <!-- Editor Area -->
        <div class="scene-plan-editor">
          <!-- Failed Target Version Notice (Save-and-Switch retry) -->
          <div
            v-if="failedTargetVersion !== null"
            class="notice error"
            data-testid="failed-target-version-notice"
          >
            <p>{{ errorMessage(loadErrorCode || 'request_failed') }}</p>
            <button
              class="secondary-button"
              type="button"
              data-testid="retry-target-version-btn"
              @click="retryTargetVersion"
            >
              {{ t('projects.actions.retry') }}
            </button>
          </div>

          <!-- Dirty Switch Warning (Save / Discard / Cancel) -->
          <div
            v-if="pendingVersion !== null"
            class="notice warning"
            data-testid="dirty-switch-warning"
          >
            <p>{{ t('scenePlan.states.dirtySwitchBlocked') }}</p>
            <div class="action-row">
              <button
                class="primary-button"
                type="button"
                data-testid="confirm-save-switch"
                :disabled="saving"
                @click="saveAndSwitch"
              >
                {{ saving ? t('scenePlan.actions.saving') : t('scenePlan.actions.saveAndSwitch') }}
              </button>
              <button
                class="secondary-button"
                type="button"
                data-testid="confirm-discard-switch"
                :disabled="saving"
                @click="discardAndSwitch"
              >
                {{ t('scenePlan.actions.discardAndSwitch') }}
              </button>
              <button
                class="secondary-button"
                type="button"
                data-testid="cancel-switch"
                :disabled="saving"
                @click="cancelSwitch"
              >
                {{ t('scenePlan.actions.cancel') }}
              </button>
            </div>
          </div>

          <!-- Stale Source Warning -->
          <div
            v-if="staleSource"
            class="notice warning"
            data-testid="stale-source-warning"
          >
            <p>{{ t('scenePlan.states.staleSource') }}</p>
          </div>

          <!-- Read-Only Notice -->
          <div
            v-if="isReadOnly"
            class="notice info"
          >
            <p>{{ t('scenePlan.states.readOnly') }}</p>
          </div>

          <!-- Mutation Status Notices -->
          <div
            v-if="saved"
            class="notice success"
          >
            <p>{{ t('scenePlan.states.saved') }}</p>
          </div>

          <div
            v-if="dirty"
            class="notice warning"
            data-testid="dirty-state"
          >
            <p>{{ t('scenePlan.states.unsavedChanges') }}</p>
          </div>

          <div
            v-if="mutationErrorCode === 'STALE_REVISION'"
            class="notice error"
          >
            <p>{{ errorMessage(mutationErrorCode) }}</p>
            <button
              class="secondary-button"
              type="button"
              data-testid="reload-stale-scene-plan"
              @click="requestStaleReload"
            >
              {{ t('scenePlan.actions.reloadAfterConflict') }}
            </button>
          </div>

          <div
            v-else-if="mutationErrorCode"
            class="notice error"
          >
            <p>{{ errorMessage(mutationErrorCode) }}</p>
          </div>

          <!-- Unmapped Form-Level Field Errors Fallback -->
          <div
            v-if="unmappedFieldErrors.length > 0"
            class="notice error"
            data-testid="unmapped-field-errors"
          >
            <p
              v-for="err in unmappedFieldErrors"
              :key="err"
            >
              {{ err }}
            </p>
          </div>

          <!-- Stale Reload Confirm -->
          <div
            v-if="confirmStaleReload"
            class="notice warning"
            data-testid="confirm-stale-reload"
          >
            <p>{{ t('scenePlan.states.confirmStaleReload') }}</p>
            <div class="action-row">
              <button
                class="primary-button"
                type="button"
                data-testid="confirm-reload-stale-scene-plan"
                @click="confirmStaleReloadAndDiscard"
              >
                {{ t('scenePlan.actions.confirmReloadAfterConflict') }}
              </button>
              <button
                class="secondary-button"
                type="button"
                @click="confirmStaleReload = false"
              >
                {{ t('scenePlan.actions.cancel') }}
              </button>
            </div>
          </div>

          <!-- Scene Plan Meta Details -->
          <div class="scene-plan-meta">
            <p><strong>{{ t('scenePlan.states.version', { value: selectedPlan.version }) }}</strong> ({{ t(`scenePlan.status.${selectedPlan.status}`) }})</p>
            <p>{{ t('scenePlan.states.revision', { value: selectedPlan.revision }) }}</p>
            <p>{{ t('scenePlan.states.sourceScriptVersion', { value: selectedPlan.source_script_version }) }}</p>
            <p>{{ t('scenePlan.states.sourceProposalVersion', { value: selectedPlan.source_proposal_version }) }}</p>
            <p>{{ t('scenePlan.states.updatedAt', { value: d(new Date(selectedPlan.updated_at), 'long') }) }}</p>
          </div>

          <!-- Form / Scene Cards -->
          <form
            class="scene-plan-form"
            @submit.prevent="saveScenePlan"
          >
            <div
              v-for="(scene, idx) in formScenes"
              :key="scene.key"
              class="scene-card"
              :data-testid="`scene-card-${idx}`"
            >
              <div class="scene-card-header">
                <h3>{{ t('scenePlan.fields.scene', { index: idx + 1, key: scene.key }) }}</h3>
                <span
                  class="scene-section-badge"
                  :data-testid="`scene-section-${idx}`"
                >
                  {{ t('scenePlan.fields.scriptSectionKey', { key: scene.script_section_key }) }}
                </span>
              </div>
              <span
                v-if="getFieldError(idx, 'key')"
                :id="`error-key-${idx}`"
                class="field-error"
                :data-testid="`error-key-${idx}`"
              >
                {{ getFieldError(idx, 'key') }}
              </span>

              <!-- Read-only approved narration -->
              <div class="scene-narration-block">
                <label>{{ t('scenePlan.fields.narration') }}</label>
                <div
                  class="scene-narration-display"
                  :data-testid="`scene-narration-${idx}`"
                >
                  {{ scene.narration }}
                </div>
                <span
                  v-if="getFieldError(idx, 'narration')"
                  :id="`error-narration-${idx}`"
                  class="field-error"
                  :data-testid="`error-narration-${idx}`"
                >
                  {{ getFieldError(idx, 'narration') }}
                </span>
              </div>

              <!-- Planning Fields -->
              <div class="scene-fields-grid">
                <label>
                  <span>{{ t('scenePlan.fields.visualInstruction') }}</span>
                  <textarea
                    :id="`scene-${idx}-visual`"
                    v-model="scene.visual_instruction"
                    :name="`scene_visual_${idx}`"
                    :disabled="isReadOnly"
                    :aria-invalid="Boolean(getFieldError(idx, 'visual_instruction'))"
                    :aria-describedby="getFieldError(idx, 'visual_instruction') ? `error-visual-${idx}` : undefined"
                    rows="3"
                  />
                  <span
                    v-if="getFieldError(idx, 'visual_instruction')"
                    :id="`error-visual-${idx}`"
                    class="field-error"
                    :data-testid="`error-visual-${idx}`"
                  >
                    {{ getFieldError(idx, 'visual_instruction') }}
                  </span>
                </label>

                <div class="scene-fields-row">
                  <label>
                    <span>{{ t('scenePlan.fields.plannedSourceType') }}</span>
                    <select
                      :id="`scene-${idx}-source-type`"
                      v-model="scene.planned_source_type"
                      :name="`scene_source_type_${idx}`"
                      :disabled="isReadOnly"
                      :aria-invalid="Boolean(getFieldError(idx, 'planned_source_type'))"
                      :aria-describedby="getFieldError(idx, 'planned_source_type') ? `error-source-type-${idx}` : undefined"
                    >
                      <option value="stock">{{ t('scenePlan.sourceTypes.stock') }}</option>
                      <option value="upload">{{ t('scenePlan.sourceTypes.upload') }}</option>
                      <option value="creator_media">{{ t('scenePlan.sourceTypes.creator_media') }}</option>
                      <option value="generated_image">{{ t('scenePlan.sourceTypes.generated_image') }}</option>
                      <option value="generated_video">{{ t('scenePlan.sourceTypes.generated_video') }}</option>
                    </select>
                    <span
                      v-if="getFieldError(idx, 'planned_source_type')"
                      :id="`error-source-type-${idx}`"
                      class="field-error"
                      :data-testid="`error-source-type-${idx}`"
                    >
                      {{ getFieldError(idx, 'planned_source_type') }}
                    </span>
                  </label>

                  <label>
                    <span>{{ t('scenePlan.fields.expectedDurationSeconds') }}</span>
                    <input
                      :id="`scene-${idx}-duration`"
                      v-model.number="scene.expected_duration_seconds"
                      type="number"
                      :name="`scene_duration_${idx}`"
                      min="1"
                      max="3600"
                      :disabled="isReadOnly"
                      :aria-invalid="Boolean(getFieldError(idx, 'expected_duration_seconds'))"
                      :aria-describedby="getFieldError(idx, 'expected_duration_seconds') ? `error-duration-${idx}` : undefined"
                    >
                    <span
                      v-if="getFieldError(idx, 'expected_duration_seconds')"
                      :id="`error-duration-${idx}`"
                      class="field-error"
                      :data-testid="`error-duration-${idx}`"
                    >
                      {{ getFieldError(idx, 'expected_duration_seconds') }}
                    </span>
                  </label>
                </div>

                <div class="scene-fields-row">
                  <label>
                    <span>{{ t('scenePlan.fields.captionIntent') }}</span>
                    <input
                      :id="`scene-${idx}-caption`"
                      v-model="scene.caption_intent"
                      type="text"
                      :name="`scene_caption_${idx}`"
                      :disabled="isReadOnly"
                      :aria-invalid="Boolean(getFieldError(idx, 'caption_intent'))"
                      :aria-describedby="getFieldError(idx, 'caption_intent') ? `error-caption-${idx}` : undefined"
                    >
                    <span
                      v-if="getFieldError(idx, 'caption_intent')"
                      :id="`error-caption-${idx}`"
                      class="field-error"
                      :data-testid="`error-caption-${idx}`"
                    >
                      {{ getFieldError(idx, 'caption_intent') }}
                    </span>
                  </label>

                  <label>
                    <span>{{ t('scenePlan.fields.transitionNotes') }}</span>
                    <input
                      :id="`scene-${idx}-transition`"
                      v-model="scene.transition_notes"
                      type="text"
                      :name="`scene_transition_${idx}`"
                      :disabled="isReadOnly"
                      :aria-invalid="Boolean(getFieldError(idx, 'transition_notes'))"
                      :aria-describedby="getFieldError(idx, 'transition_notes') ? `error-transition-${idx}` : undefined"
                    >
                    <span
                      v-if="getFieldError(idx, 'transition_notes')"
                      :id="`error-transition-${idx}`"
                      class="field-error"
                      :data-testid="`error-transition-${idx}`"
                    >
                      {{ getFieldError(idx, 'transition_notes') }}
                    </span>
                  </label>
                </div>
              </div>

              <!-- Scene Action Buttons: Split & Merge -->
              <div
                v-if="!isReadOnly"
                class="scene-card-actions"
              >
                <button
                  class="secondary-button"
                  type="button"
                  :data-testid="`split-scene-${idx}`"
                  @click="openSplitModal(idx)"
                >
                  {{ t('scenePlan.actions.splitScene') }}
                </button>

                <button
                  v-if="canMergeWithNext(idx)"
                  class="secondary-button"
                  type="button"
                  :data-testid="`merge-scene-${idx}`"
                  @click="mergeWithNext(idx)"
                >
                  {{ t('scenePlan.actions.mergeWithNext') }}
                </button>
              </div>
            </div>

            <!-- Workspace Actions: Save & Approve -->
            <div
              v-if="!isReadOnly"
              class="scene-plan-actions"
            >
              <button
                class="primary-button"
                type="submit"
                data-testid="save-scene-plan-btn"
                :disabled="saving || !dirty"
              >
                {{ saving ? t('scenePlan.actions.saving') : t('scenePlan.actions.save') }}
              </button>

              <button
                class="secondary-button"
                type="button"
                data-testid="approve-scene-plan-btn"
                :disabled="approving || dirty"
                @click="confirmApproval = true"
              >
                {{ approving ? t('scenePlan.actions.approving') : t('scenePlan.actions.approve') }}
              </button>
            </div>
          </form>

          <!-- Approval Confirmation Modal/Notice -->
          <div
            v-if="confirmApproval"
            class="notice warning"
            data-testid="confirm-approve-modal"
          >
            <p>{{ t('scenePlan.states.confirmApproval') }}</p>
            <div class="action-row">
              <button
                class="primary-button"
                type="button"
                data-testid="confirm-approve-btn"
                :disabled="approving"
                @click="approveSelected"
              >
                {{ t('scenePlan.actions.confirmApprove') }}
              </button>
              <button
                class="secondary-button"
                type="button"
                @click="confirmApproval = false"
              >
                {{ t('scenePlan.actions.cancel') }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </template>

    <!-- Split Scene Modal (Unicode Code-Point Safe) -->
    <div
      v-if="splitTargetScene"
      class="modal-backdrop"
      data-testid="split-modal"
    >
      <div class="modal-card">
        <h3>{{ t('scenePlan.splitModal.title') }}</h3>
        <p class="body-copy">
          {{ t('scenePlan.splitModal.description') }}
        </p>

        <div class="split-controls">
          <label>
            <span>{{ t('scenePlan.splitModal.splitPointLabel', { max: splitMaxCodePoints }) }}</span>
            <input
              id="split-point-input"
              v-model.number="splitPoint"
              type="number"
              min="1"
              :max="splitMaxCodePoints"
              data-testid="split-index-input"
            >
          </label>

          <label>
            <span>{{ t('scenePlan.splitModal.newKeyLabel') }}</span>
            <input
              id="split-key-input"
              v-model="splitNewKey"
              type="text"
              data-testid="split-new-key-input"
            >
          </label>
        </div>

        <div class="split-preview">
          <div class="preview-box">
            <strong>{{ t('scenePlan.splitModal.part1Preview') }}:</strong>
            <p data-testid="split-preview-part1">{{ splitPart1Preview }}</p>
          </div>
          <div class="preview-box">
            <strong>{{ t('scenePlan.splitModal.part2Preview') }}:</strong>
            <p data-testid="split-preview-part2">{{ splitPart2Preview }}</p>
          </div>
        </div>

        <div
          v-if="splitError"
          class="notice error"
          data-testid="split-error-notice"
        >
          <p>{{ splitError }}</p>
        </div>

        <div class="action-row">
          <button
            class="primary-button"
            type="button"
            data-testid="confirm-split-btn"
            @click="confirmSplit"
          >
            {{ t('scenePlan.actions.confirmSplit') }}
          </button>
          <button
            class="secondary-button"
            type="button"
            data-testid="cancel-split-btn"
            @click="closeSplitModal"
          >
            {{ t('scenePlan.actions.cancel') }}
          </button>
        </div>
      </div>
    </div>
  </section>
</template>
