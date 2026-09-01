<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'

import { ApiError, getProject, type Project } from '@/api/projects'
import { listCreativeProposals } from '@/features/creative-proposal/api'
import {
  approveScript,
  createScriptGeneration,
  getScript,
  getScriptGeneration,
  getTextGenerationOptions,
  listScripts,
  putScript,
  type Script,
  type ScriptEditableContent,
  type ScriptGenerationJob,
  type ScriptSummary,
  type TextGenerationOptionProvider,
} from './api'

interface ProposalSummary {
  version: number
  status: 'draft' | 'approved' | 'superseded'
}

const { t, d } = useI18n()
const route = useRoute()

const project = ref<Project | null>(null)
const summaries = ref<ScriptSummary[]>([])
const proposals = ref<ProposalSummary[]>([])
const selectedScript = ref<Script | null>(null)
const selectedVersion = ref<number | null>(null)
const pendingVersion = ref<number | null>(null)
const form = ref<ScriptEditableContent>(emptyContent())
const savedSnapshot = ref('')
const loading = ref(true)
const scriptsLoaded = ref(false)
const scriptLoading = ref(false)
const saving = ref(false)
const approving = ref(false)
const saved = ref(false)
const dirty = ref(false)
const confirmApproval = ref(false)
const loadErrorCode = ref('')
const mutationErrorCode = ref('')
const confirmStaleReload = ref(false)
const failedVersion = ref<number | null>(null)
const fieldErrors = ref<Record<string, string>>({})

const providerOptions = ref<TextGenerationOptionProvider[]>([])
const selectedProviderId = ref('')
const selectedModelId = ref('')
const activeJob = ref<ScriptGenerationJob | null>(null)
const generating = ref(false)
const generationErrorCode = ref('')
let pollTimer: number | null = null

const selectedProviderModels = computed(() => {
  return providerOptions.value.find((provider) => provider.id === selectedProviderId.value)?.models ?? []
})
const hasProviders = computed(() => providerOptions.value.some((provider) => provider.models.length > 0))
const approvedProposalVersion = computed(() => {
  const approved = proposals.value.filter((proposal) => proposal.status === 'approved')
  return approved.reduce((highest, proposal) => Math.max(highest, proposal.version), 0) || null
})
const hasApprovedProposal = computed(() => approvedProposalVersion.value !== null)
const hasScript = computed(() => summaries.value.length > 0)
const isReadOnly = computed(() => selectedScript.value?.status !== 'draft')
const generationInProgress = computed(() => activeJob.value?.state === 'queued' || activeJob.value?.state === 'running')
const succeededJobUnloaded = computed(() => activeJob.value?.state === 'succeeded')
const canGenerate = computed(() => {
  return hasApprovedProposal.value && hasProviders.value && Boolean(selectedModelId.value) &&
    !dirty.value && !generationInProgress.value && !generating.value && !succeededJobUnloaded.value
})
const staleSource = computed(() => {
  return selectedScript.value !== null && approvedProposalVersion.value !== null &&
    approvedProposalVersion.value > selectedScript.value.source_proposal_version
})

watch(form, () => {
  dirty.value = JSON.stringify(form.value) !== savedSnapshot.value
  if (dirty.value) {
    saved.value = false
    confirmApproval.value = false
  }
}, { deep: true })

onMounted(() => {
  void loadWorkspace()
  void loadGenerationOptions()
  resumeActiveJob()
})

onUnmounted(() => {
  stopPolling()
})

async function loadGenerationOptions() {
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
  } catch {
    providerOptions.value = []
  }
}

function onProviderChange(providerID: string) {
  selectedProviderId.value = providerID
  selectedModelId.value = providerOptions.value.find((provider) => provider.id === providerID)?.models[0]?.id ?? ''
}

async function loadWorkspace() {
  loading.value = true
  loadErrorCode.value = ''
  mutationErrorCode.value = ''
  failedVersion.value = null
  scriptsLoaded.value = false
  try {
    project.value = await getProject(String(route.params.id))
    scriptsLoaded.value = true
    summaries.value = await listScripts(project.value.id)
    proposals.value = await loadProposalSummaries(project.value.id)
    const initial = summaries.value.find((item) => item.status === 'draft') ?? summaries.value[0]
    if (initial) {
      await loadVersion(initial.version, true)
    } else {
      selectedScript.value = null
      selectedVersion.value = null
      form.value = emptyContent()
      savedSnapshot.value = serialize(form.value)
      dirty.value = false
    }
  } catch (error) {
    loadErrorCode.value = error instanceof ApiError ? error.code : 'request_failed'
  } finally {
    loading.value = false
  }
}

async function loadProposalSummaries(projectID: string): Promise<ProposalSummary[]> {
  return listCreativeProposals(projectID)
}

async function loadVersion(version: number, discardDirty = false): Promise<boolean> {
  if (dirty.value && !discardDirty && selectedVersion.value !== version) {
    pendingVersion.value = version
    return false
  }
  scriptLoading.value = true
  loadErrorCode.value = ''
  mutationErrorCode.value = ''
  fieldErrors.value = {}
  confirmStaleReload.value = false
  try {
    const script = await getScript(projectID(), version)
    applyScript(script)
    return true
  } catch (error) {
    loadErrorCode.value = error instanceof ApiError ? error.code : 'request_failed'
    failedVersion.value = version
    return false
  } finally {
    scriptLoading.value = false
  }
}

function applyScript(script: Script) {
  selectedScript.value = script
  selectedVersion.value = script.version
  form.value = toContent(script)
  savedSnapshot.value = serialize(form.value)
  dirty.value = false
  saved.value = false
  pendingVersion.value = null
  failedVersion.value = null
  confirmApproval.value = false
}

async function saveScript() {
  if (!selectedScript.value || selectedScript.value.status !== 'draft' || !dirty.value) return
  saving.value = true
  mutationErrorCode.value = ''
  loadErrorCode.value = ''
  fieldErrors.value = {}
  try {
    const updated = await putScript(projectID(), selectedScript.value.version, {
      revision: selectedScript.value.revision,
      ...toPayload(),
    })
    applyScript(updated)
    saved.value = true
    upsertSummary(updated)
  } catch (error) {
    handleMutationError(error)
  } finally {
    saving.value = false
  }
}

async function approveSelected() {
  if (!selectedScript.value || selectedScript.value.status !== 'draft' || dirty.value) return
  approving.value = true
  mutationErrorCode.value = ''
  try {
    const approved = await approveScript(projectID(), selectedScript.value.version, selectedScript.value.revision)
    summaries.value = await listScripts(projectID())
    applyScript(approved)
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
  mutationErrorCode.value = ''
  try {
    const requestId = requestID()
    if (!requestId) {
      generationErrorCode.value = 'GENERATION_REQUEST_ID_UNAVAILABLE'
      return
    }
    const job = await createScriptGeneration(projectID(), {
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
      const job = await getScriptGeneration(projectID(), jobID)
      activeJob.value = job
      generationErrorCode.value = ''
      if (job.state === 'succeeded') {
        await loadSucceededScript()
        return
      }
      if (job.state === 'failed') {
        removePersistedJob()
        generationErrorCode.value = job.error_code ?? 'request_failed'
        return
      }
      schedulePoll(poll)
    } catch (error) {
      generationErrorCode.value = error instanceof ApiError ? error.code : 'request_failed'
      schedulePoll(poll)
    }
  }
  schedulePoll(poll)
}

async function loadSucceededScript() {
  const job = activeJob.value
  if (!job || job.state !== 'succeeded' || job.script_version === null) {
    generationErrorCode.value = 'request_failed'
    return
  }
  try {
    summaries.value = await listScripts(projectID())
    scriptsLoaded.value = true
    if (dirty.value) {
      pendingVersion.value = job.script_version
      generationErrorCode.value = ''
      return
    }
    const loaded = await loadVersion(job.script_version, true)
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
  if (typeof window !== 'undefined' && window.sessionStorage) window.sessionStorage.setItem(storageKey(), jobID)
}

function removePersistedJob() {
  if (typeof window !== 'undefined' && window.sessionStorage) window.sessionStorage.removeItem(storageKey())
}

function storageKey() {
  return `script_job_${projectID()}`
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
  const loaded = await loadVersion(version, true)
  if (loaded && activeJob.value?.state === 'succeeded' && activeJob.value.script_version === version) {
    removePersistedJob()
    activeJob.value = null
    generationErrorCode.value = ''
  }
}

function cancelSwitch() {
  pendingVersion.value = null
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

function addSection() {
  form.value.sections.push({ key: '', heading: '', body: '' })
}

function removeSection(index: number) {
  form.value.sections.splice(index, 1)
}

function toPayload(): ScriptEditableContent {
  return {
    sections: form.value.sections.map((section) => ({ ...section })),
    estimated_duration_seconds: form.value.estimated_duration_seconds,
    notes: form.value.notes,
  }
}

function toContent(script: Script): ScriptEditableContent {
  return {
    sections: script.sections.map((section) => ({ ...section })),
    estimated_duration_seconds: script.estimated_duration_seconds,
    notes: script.notes,
  }
}

function emptyContent(): ScriptEditableContent {
  return { sections: [{ key: '', heading: '', body: '' }], estimated_duration_seconds: null, notes: '' }
}

function serialize(content: ScriptEditableContent) {
  return JSON.stringify(content)
}

function projectID() {
  return project.value?.id ?? String(route.params.id)
}

function upsertSummary(script: Script) {
  const summary: ScriptSummary = {
    version: script.version,
    revision: script.revision,
    status: script.status,
    source_proposal_version: script.source_proposal_version,
    content_locale: script.content_locale,
    created_at: script.created_at,
    updated_at: script.updated_at,
    approved_at: script.approved_at,
  }
  const index = summaries.value.findIndex((item) => item.version === script.version)
  summaries.value = index === -1 ? [summary, ...summaries.value] : summaries.value.map((item, i) => i === index ? summary : item)
}

function errorMessage(code: string) {
  return t(`script.errors.${code}`)
}
</script>

<template>
  <section class="page script-page">
    <RouterLink
      v-if="project"
      class="text-link"
      :to="`/projects/${project.id}`"
    >
      {{ t('script.actions.backToProject') }}
    </RouterLink>

    <p
      v-if="loading"
      class="state-text"
    >
      {{ t('script.states.loading') }}
    </p>
    <div
      v-else-if="loadErrorCode && !project"
      class="notice error"
    >
      <p>{{ errorMessage(loadErrorCode) }}</p>
      <button
        class="secondary-button"
        type="button"
        @click="loadWorkspace"
      >
        {{ t('projects.actions.retry') }}
      </button>
    </div>

    <template v-else-if="project">
      <p class="eyebrow">
        {{ t('script.eyebrow') }}
      </p>
      <h1>{{ project.title }}</h1>
      <p class="body-copy">
        {{ t('script.description') }}
      </p>

      <section class="script-generation-bar">
        <div
          v-if="hasProviders"
          class="generation-controls"
        >
          <div class="control-group">
            <label for="script-provider-select">{{ t('script.generation.selectProvider') }}</label>
            <select
              id="script-provider-select"
              v-model="selectedProviderId"
              data-testid="select-script-provider"
              :disabled="dirty || generationInProgress || generating"
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
            <label for="script-model-select">{{ t('script.generation.selectModel') }}</label>
            <select
              id="script-model-select"
              v-model="selectedModelId"
              data-testid="select-script-model"
              :disabled="dirty || generationInProgress || generating"
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
            data-testid="generate-script-btn"
            type="button"
            :disabled="!canGenerate"
            @click="startGeneration"
          >
            {{ generationInProgress || generating ? t('script.actions.generating') : hasScript ? t('script.actions.regenerate') : t('script.actions.generate') }}
          </button>
        </div>
        <div
          v-else
          class="notice info"
          data-testid="no-script-providers-notice"
        >
          <p>{{ t('script.generation.noProvidersConfigured') }}</p>
          <RouterLink
            class="text-link"
            to="/settings/ai-providers"
          >
            {{ t('script.generation.openProviderSettings') }}
          </RouterLink>
        </div>

        <div
          v-if="!hasApprovedProposal"
          class="notice warning"
          data-testid="proposal-readiness-warning"
        >
          <p>{{ t('script.generation.proposalRequired') }}</p>
          <RouterLink
            class="text-link"
            :to="`/projects/${project.id}/creative-proposal`"
          >
            {{ t('script.generation.openProposal') }}
          </RouterLink>
        </div>
        <div
          v-if="dirty"
          class="notice warning"
          data-testid="dirty-generation-blocked"
        >
          <p>{{ t('script.generation.dirtyBlocked') }}</p>
        </div>
        <div
          v-if="generationInProgress && activeJob"
          class="notice info"
          data-testid="job-progress-banner"
        >
          {{ activeJob.state === 'queued' ? t('script.generation.stateQueued') : t('script.generation.stateRunning') }}
        </div>
        <div
          v-if="succeededJobUnloaded"
          class="notice info"
          data-testid="job-succeeded-load-failed-banner"
        >
          <p>{{ t('script.generation.stateSucceeded') }}</p>
          <p v-if="generationErrorCode">
            {{ errorMessage(generationErrorCode) }}
          </p>
          <button
            class="secondary-button"
            data-testid="retry-load-generated-btn"
            type="button"
            @click="loadSucceededScript"
          >
            {{ t('script.actions.retryLoadGenerated') }}
          </button>
        </div>
        <div
          v-if="generationErrorCode && !succeededJobUnloaded"
          class="notice error"
          data-testid="job-error-banner"
        >
          <p>{{ errorMessage(generationErrorCode) }}</p>
          <button
            v-if="hasProviders && !generationInProgress"
            class="secondary-button"
            data-testid="retry-generation-btn"
            type="button"
            :disabled="dirty"
            @click="startGeneration"
          >
            {{ t('script.actions.retryGeneration') }}
          </button>
        </div>
      </section>

      <div
        v-if="loadErrorCode && !hasScript && scriptsLoaded"
        class="notice error"
      >
        <p>{{ errorMessage(loadErrorCode) }}</p>
        <button
          class="secondary-button"
          data-testid="retry-script-list"
          type="button"
          @click="loadWorkspace"
        >
          {{ t('projects.actions.retry') }}
        </button>
      </div>
      <div
        v-else-if="!hasScript && scriptsLoaded"
        class="notice info"
        data-testid="script-empty-state"
      >
        <p>{{ t('script.states.empty') }}</p>
        <p>{{ t('script.states.emptyDetail') }}</p>
      </div>

      <div
        v-else-if="hasScript"
        class="script-workspace"
      >
        <aside
          class="script-history"
          :aria-label="t('script.history.label')"
        >
          <h2>{{ t('script.history.title') }}</h2>
          <button
            v-for="summary in summaries"
            :key="summary.version"
            class="script-version-button"
            :class="{ active: summary.version === selectedVersion }"
            type="button"
            :data-testid="`version-${summary.version}`"
            @click="chooseVersion(summary.version)"
          >
            <span>{{ t('script.states.version', { value: summary.version }) }}</span>
            <small>{{ t(`script.status.${summary.status}`) }} · {{ t('script.states.revision', { value: summary.revision }) }}</small>
          </button>
        </aside>

        <section
          class="script-editor"
          aria-live="polite"
        >
          <p
            v-if="scriptLoading"
            class="state-text"
          >
            {{ t('script.states.loadingVersion') }}
          </p>
          <div
            v-if="loadErrorCode"
            class="notice error"
            data-testid="script-load-error"
          >
            <p>{{ errorMessage(loadErrorCode) }}</p>
            <button
              class="secondary-button"
              type="button"
              @click="retryLoad"
            >
              {{ t('projects.actions.retry') }}
            </button>
          </div>
          <template v-if="selectedScript">
            <div class="script-meta">
              <p>{{ t('script.states.version', { value: selectedScript.version }) }} · {{ t('script.states.revision', { value: selectedScript.revision }) }}</p>
              <p>{{ t('script.states.sourceProposalVersion', { value: selectedScript.source_proposal_version }) }} · {{ t('script.states.updatedAt', { value: d(new Date(selectedScript.updated_at), 'long') }) }}</p>
            </div>
            <div
              v-if="staleSource"
              class="notice warning"
              data-testid="stale-source-warning"
            >
              {{ t('script.states.staleSource') }}
            </div>
            <div
              v-if="saved && !dirty"
              class="notice success"
            >
              {{ t('script.states.saved') }}
            </div>
            <div
              v-else-if="dirty"
              class="notice info"
              data-testid="dirty-state"
            >
              {{ t('script.states.unsavedChanges') }}
            </div>
            <div
              v-if="pendingVersion !== null"
              class="notice warning"
              data-testid="dirty-switch-warning"
            >
              <p>{{ t('script.states.dirtySwitchBlocked') }}</p>
              <button
                class="secondary-button"
                type="button"
                data-testid="discard-version-switch"
                @click="discardAndSwitch"
              >
                {{ t('script.actions.discardAndSwitch') }}
              </button>
              <button
                class="secondary-button"
                type="button"
                data-testid="cancel-version-switch"
                @click="cancelSwitch"
              >
                {{ t('script.actions.cancel') }}
              </button>
            </div>
            <div
              v-if="mutationErrorCode"
              class="notice error"
            >
              <p>{{ errorMessage(mutationErrorCode) }}</p>
              <p
                v-for="(error, field) in fieldErrors"
                :key="field"
              >
                {{ field }}: {{ t(`script.validation.${error}`) }}
              </p>
              <template v-if="mutationErrorCode === 'STALE_REVISION'">
                <button
                  v-if="!confirmStaleReload"
                  class="secondary-button"
                  data-testid="reload-stale-script"
                  type="button"
                  @click="requestStaleReload"
                >
                  {{ t('script.actions.reloadAfterConflict') }}
                </button>
                <div
                  v-else
                  class="notice warning"
                  data-testid="confirm-stale-reload"
                >
                  <p>{{ t('script.states.confirmStaleReload') }}</p>
                  <button
                    class="primary-button"
                    data-testid="confirm-reload-stale-script"
                    type="button"
                    @click="confirmStaleReloadAndDiscard"
                  >
                    {{ t('script.actions.confirmReloadAfterConflict') }}
                  </button>
                  <button
                    class="secondary-button"
                    type="button"
                    @click="confirmStaleReload = false"
                  >
                    {{ t('script.actions.cancel') }}
                  </button>
                </div>
              </template>
            </div>
            <div
              v-if="isReadOnly"
              class="notice info"
            >
              {{ t('script.states.readOnly') }}
            </div>

            <form
              class="script-form"
              @submit.prevent="saveScript"
            >
              <fieldset
                v-for="(section, index) in form.sections"
                :key="index"
                class="script-section-fieldset"
                :disabled="isReadOnly"
              >
                <legend>{{ t('script.fields.section', { value: index + 1 }) }}</legend>
                <label class="field"><span>{{ t('script.fields.key') }}</span><input
                  v-model="section.key"
                  :disabled="isReadOnly"
                  :name="`section_key_${index}`"
                  autocomplete="off"
                ><small v-if="fieldErrors[`sections[${index}].key`]">{{ t(`script.validation.${fieldErrors[`sections[${index}].key`]}`) }}</small></label>
                <label class="field"><span>{{ t('script.fields.heading') }}</span><input
                  v-model="section.heading"
                  :disabled="isReadOnly"
                  :name="`section_heading_${index}`"
                ></label>
                <label class="field"><span>{{ t('script.fields.body') }}</span><textarea
                  v-model="section.body"
                  :disabled="isReadOnly"
                  :name="`section_body_${index}`"
                  rows="7"
                /><small v-if="fieldErrors[`sections[${index}].body`]">{{ t(`script.validation.${fieldErrors[`sections[${index}].body`]}`) }}</small></label>
                <button
                  v-if="!isReadOnly"
                  class="secondary-button"
                  type="button"
                  @click="removeSection(index)"
                >
                  {{ t('script.actions.removeSection') }}
                </button>
              </fieldset>
              <button
                v-if="!isReadOnly"
                class="secondary-button"
                type="button"
                @click="addSection"
              >
                {{ t('script.actions.addSection') }}
              </button>
              <label class="field"><span>{{ t('script.fields.estimatedDuration') }}</span><input
                v-model.number="form.estimated_duration_seconds"
                :disabled="isReadOnly"
                type="number"
                min="1"
                max="43200"
                name="estimated_duration_seconds"
              ></label>
              <label class="field"><span>{{ t('script.fields.notes') }}</span><textarea
                v-model="form.notes"
                :disabled="isReadOnly"
                name="notes"
                rows="4"
              /></label>
              <div
                v-if="selectedScript.status === 'draft'"
                class="script-actions"
              >
                <button
                  class="primary-button"
                  data-testid="save-script"
                  type="submit"
                  :disabled="!dirty || saving"
                >
                  {{ saving ? t('script.actions.saving') : t('script.actions.save') }}
                </button>
                <button
                  v-if="!confirmApproval"
                  class="secondary-button"
                  data-testid="approve-script"
                  type="button"
                  :disabled="dirty || saving || approving"
                  @click="confirmApproval = true"
                >
                  {{ t('script.actions.approve') }}
                </button>
                <template v-else>
                  <span>{{ t('script.states.confirmApproval') }}</span>
                  <button
                    class="primary-button"
                    data-testid="confirm-approve-script"
                    type="button"
                    :disabled="approving"
                    @click="approveSelected"
                  >
                    {{ approving ? t('script.actions.approving') : t('script.actions.confirmApprove') }}
                  </button>
                  <button
                    class="secondary-button"
                    type="button"
                    :disabled="approving"
                    @click="confirmApproval = false"
                  >
                    {{ t('script.actions.cancel') }}
                  </button>
                </template>
              </div>
            </form>
          </template>
        </section>
      </div>
    </template>
  </section>
</template>
