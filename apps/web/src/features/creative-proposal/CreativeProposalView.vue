<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'

import { ApiError, getProject, type Project } from '@/api/projects'
import CreativeProposalForm, { type CreativeProposalFormState } from './CreativeProposalForm.vue'
import {
  approveCreativeProposal,
  createProposalGeneration,
  getCreativeProposal,
  getProposalGeneration,
  getTextGenerationOptions,
  listCreativeProposals,
  putCreativeProposal,
  type CreativeProposal,
  type CreativeProposalEditableContent,
  type CreativeProposalSummary,
  type ProposalGenerationJob,
  type TextGenerationOptionProvider,
} from './api'

const { t, d } = useI18n()
const route = useRoute()

const project = ref<Project | null>(null)
const summaries = ref<CreativeProposalSummary[]>([])
const selectedProposal = ref<CreativeProposal | null>(null)
const selectedVersion = ref<number | null>(null)
const pendingVersion = ref<number | null>(null)
const loading = ref(true)
const proposalLoading = ref(false)
const submitting = ref(false)
const approving = ref(false)
const saved = ref(false)
const dirty = ref(false)
const staleConflict = ref(false)
const confirmApproval = ref(false)
const loadErrorCode = ref('')
const mutationErrorCode = ref('')
const failedVersion = ref<number | null>(null)
const fieldErrors = ref<Record<string, string>>({})
const formValues = ref<CreativeProposalFormState>(emptyFormState())

const providerOptions = ref<TextGenerationOptionProvider[]>([])
const selectedProviderId = ref('')
const selectedModelId = ref('')
const activeJob = ref<ProposalGenerationJob | null>(null)
const generating = ref(false)
const generationErrorCode = ref('')
let pollTimer: number | null = null

const isReadOnly = computed(() => selectedProposal.value?.status !== 'draft')
const hasProposal = computed(() => summaries.value.length > 0)
const hasAIProviders = computed(() => providerOptions.value.length > 0)
const selectedProviderModels = computed(() => {
  const prov = providerOptions.value.find((p) => p.id === selectedProviderId.value)
  return prov ? prov.models : []
})
const isGenerationInProgress = computed(() => {
  return activeJob.value !== null && (activeJob.value.state === 'queued' || activeJob.value.state === 'running')
})
const hasSucceededJobUnloaded = computed(() => {
  return activeJob.value !== null && activeJob.value.state === 'succeeded'
})

onMounted(() => {
  void loadWorkspace()
  void loadGenerationOptions()
  resumeActiveJobFromSession()
})

onUnmounted(() => {
  if (pollTimer !== null) {
    window.clearTimeout(pollTimer)
    pollTimer = null
  }
})

async function loadGenerationOptions() {
  try {
    const res = await getTextGenerationOptions()
    providerOptions.value = res.providers ?? []
    const firstProvider = providerOptions.value[0]
    if (firstProvider && !selectedProviderId.value) {
      selectedProviderId.value = firstProvider.id
      const firstModel = firstProvider.models[0]
      if (firstModel) {
        selectedModelId.value = firstModel.id
      }
    }
  } catch {
    providerOptions.value = []
  }
}

function onProviderChange(provId: string) {
  selectedProviderId.value = provId
  const prov = providerOptions.value.find((p) => p.id === provId)
  const firstModel = prov?.models[0]
  if (firstModel) {
    selectedModelId.value = firstModel.id
  } else {
    selectedModelId.value = ''
  }
}

async function startGeneration() {
  if (dirty.value || !selectedProviderId.value || !selectedModelId.value || isGenerationInProgress.value) {
    return
  }
  generating.value = true
  generationErrorCode.value = ''
  mutationErrorCode.value = ''
  loadErrorCode.value = ''
  try {
    const requestId = crypto.randomUUID()
    const job = await createProposalGeneration(projectID(), {
      request_id: requestId,
      provider_id: selectedProviderId.value,
      model_id: selectedModelId.value,
    })
    activeJob.value = job
    if (typeof window !== 'undefined' && window.sessionStorage) {
      window.sessionStorage.setItem(`proposal_job_${projectID()}`, job.id)
    }
    trackJob(job.id)
  } catch (error) {
    if (error instanceof ApiError) {
      generationErrorCode.value = error.code
    } else {
      generationErrorCode.value = 'request_failed'
    }
  } finally {
    generating.value = false
  }
}

async function loadSucceededProposal() {
  if (!activeJob.value || activeJob.value.state !== 'succeeded') {
    return
  }
  generationErrorCode.value = ''
  try {
    summaries.value = await listCreativeProposals(projectID())
    if (activeJob.value.proposal_version) {
      const ok = await loadVersion(activeJob.value.proposal_version, true)
      if (!ok) {
        generationErrorCode.value = loadErrorCode.value || 'request_failed'
        return
      }
    }
    if (typeof window !== 'undefined' && window.sessionStorage) {
      window.sessionStorage.removeItem(`proposal_job_${projectID()}`)
    }
    activeJob.value = null
  } catch (error) {
    if (error instanceof ApiError) {
      generationErrorCode.value = error.code
    } else {
      generationErrorCode.value = 'request_failed'
    }
  }
}

function trackJob(jobId: string) {
  if (pollTimer !== null) {
    window.clearTimeout(pollTimer)
    pollTimer = null
  }

  const poll = async () => {
    try {
      const job = await getProposalGeneration(projectID(), jobId)
      activeJob.value = job
      generationErrorCode.value = ''
      if (job.state === 'succeeded') {
        await loadSucceededProposal()
        return
      }
      if (job.state === 'failed') {
        if (typeof window !== 'undefined' && window.sessionStorage) {
          window.sessionStorage.removeItem(`proposal_job_${projectID()}`)
        }
        generationErrorCode.value = job.error_code || 'request_failed'
        return
      }
      pollTimer = window.setTimeout(poll, 1000)
    } catch (error) {
      if (error instanceof ApiError) {
        generationErrorCode.value = error.code
      } else {
        generationErrorCode.value = 'request_failed'
      }
      pollTimer = window.setTimeout(poll, 1000)
    }
  }

  pollTimer = window.setTimeout(poll, 1000)
}

function resumeActiveJobFromSession() {
  if (typeof window === 'undefined' || !window.sessionStorage) {
    return
  }
  const savedJobId = window.sessionStorage.getItem(`proposal_job_${projectID()}`)
  if (savedJobId) {
    trackJob(savedJobId)
  }
}

async function loadWorkspace() {
  loading.value = true
  loadErrorCode.value = ''
  mutationErrorCode.value = ''
  failedVersion.value = null
  resetEditState()
  try {
    project.value = await getProject(String(route.params.id))
    summaries.value = await listCreativeProposals(project.value.id)
    const newestSummary = summaries.value[0]
    if (newestSummary) {
      await loadVersion(newestSummary.version, true)
    }
  } catch (error) {
    loadErrorCode.value = error instanceof ApiError ? error.code : 'request_failed'
  } finally {
    loading.value = false
  }
}

async function loadVersion(version: number, discardDirty = false): Promise<boolean> {
  if (dirty.value && !discardDirty && selectedVersion.value !== version) {
    pendingVersion.value = version
    return false
  }

  proposalLoading.value = true
  loadErrorCode.value = ''
  mutationErrorCode.value = ''
  fieldErrors.value = {}
  try {
    const proposal = await getCreativeProposal(String(route.params.id), version)
    applyProposal(proposal)
    return true
  } catch (error) {
    loadErrorCode.value = error instanceof ApiError ? error.code : 'request_failed'
    failedVersion.value = version
    return false
  } finally {
    proposalLoading.value = false
  }
}

async function reloadSelected() {
  if (selectedVersion.value === null) {
    return
  }
  await loadVersion(selectedVersion.value, true)
}

async function discardAndSwitchVersion() {
  if (pendingVersion.value === null) {
    return
  }
  const version = pendingVersion.value
  pendingVersion.value = null
  await loadVersion(version, true)
}

function applyProposal(proposal: CreativeProposal) {
  selectedProposal.value = proposal
  selectedVersion.value = proposal.version
  formValues.value = toFormState(proposal)
  upsertSummary(proposal)
  saved.value = false
  dirty.value = false
  staleConflict.value = false
  confirmApproval.value = false
  pendingVersion.value = null
  failedVersion.value = null
}

function onDirtyChange(isDirty: boolean) {
  dirty.value = isDirty
  if (isDirty) {
    saved.value = false
    confirmApproval.value = false
  }
}

async function submit(payload: CreativeProposalEditableContent) {
  if (!selectedProposal.value) {
    return
  }
  submitting.value = true
  mutationErrorCode.value = ''
  loadErrorCode.value = ''
  failedVersion.value = null
  fieldErrors.value = {}
  saved.value = false
  staleConflict.value = false
  try {
    const proposal = await putCreativeProposal(projectID(), selectedProposal.value.version, {
      ...payload,
      revision: selectedProposal.value.revision,
    })
    applyProposal(proposal)
    saved.value = true
  } catch (error) {
    handleMutationError(error)
  } finally {
    submitting.value = false
  }
}

async function approveSelected() {
  if (!selectedProposal.value) {
    return
  }
  approving.value = true
  mutationErrorCode.value = ''
  loadErrorCode.value = ''
  failedVersion.value = null
  staleConflict.value = false
  try {
    const approved = await approveCreativeProposal(
      projectID(),
      selectedProposal.value.version,
      selectedProposal.value.revision,
    )
    applyProposal(approved)
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
    if (error.code === 'STALE_REVISION') {
      staleConflict.value = true
    }
    return
  }
  mutationErrorCode.value = 'request_failed'
}

function resetEditState() {
  saved.value = false
  dirty.value = false
  staleConflict.value = false
  confirmApproval.value = false
  pendingVersion.value = null
}

function upsertSummary(proposal: CreativeProposal) {
  const nextSummary: CreativeProposalSummary = {
    version: proposal.version,
    revision: proposal.revision,
    status: proposal.status,
    source_brief_revision: proposal.source_brief_revision,
    created_at: proposal.created_at,
    updated_at: proposal.updated_at,
    approved_at: proposal.approved_at,
  }
  const index = summaries.value.findIndex((item) => item.version === proposal.version)
  if (index === -1) {
    summaries.value = [nextSummary, ...summaries.value]
    return
  }
  summaries.value = summaries.value.map((item, itemIndex) => (itemIndex === index ? nextSummary : item))
}

function emptyFormState(): CreativeProposalFormState {
  return {
    titleOptionsText: '',
    hookOptionsText: '',
    audience_summary: '',
    objective_summary: '',
    narrative_angle: '',
    estimatedDurationText: '',
    format_rationale: '',
    structure: [{ key: '', title: '', purpose: '' }],
    visual_direction: '',
    voice_direction: '',
    music_direction: '',
    caption_direction: '',
    call_to_action: '',
    researchGapsText: '',
    warningsText: '',
  }
}

function toFormState(proposal: CreativeProposal): CreativeProposalFormState {
  return {
    titleOptionsText: proposal.title_options.join('\n'),
    hookOptionsText: proposal.hook_options.join('\n'),
    audience_summary: proposal.audience_summary,
    objective_summary: proposal.objective_summary,
    narrative_angle: proposal.narrative_angle,
    estimatedDurationText: proposal.estimated_duration_seconds?.toString() ?? '',
    format_rationale: proposal.format_rationale,
    structure: proposal.structure.length > 0 ? proposal.structure.map((item) => ({ ...item })) : [{ key: '', title: '', purpose: '' }],
    visual_direction: proposal.visual_direction,
    voice_direction: proposal.voice_direction,
    music_direction: proposal.music_direction,
    caption_direction: proposal.caption_direction,
    call_to_action: proposal.call_to_action,
    researchGapsText: proposal.research_gaps.join('\n'),
    warningsText: proposal.warnings.join('\n'),
  }
}

function projectID(): string {
  return project.value?.id ?? String(route.params.id)
}

async function retryFailedVersion() {
  if (failedVersion.value === null) {
    await loadWorkspace()
    return
  }
  await loadVersion(failedVersion.value, true)
}
</script>

<template>
  <section class="page creative-proposal-page">
    <RouterLink
      v-if="project"
      class="text-link"
      :to="`/projects/${project.id}`"
    >
      {{ t('creativeProposal.actions.backToProject') }}
    </RouterLink>

    <p
      v-if="loading"
      class="state-text"
    >
      {{ t('creativeProposal.states.loading') }}
    </p>
    <div
      v-else-if="loadErrorCode && !project"
      class="notice error"
    >
      <p>{{ t(`creativeProposal.errors.${loadErrorCode}`) }}</p>
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
        {{ t('creativeProposal.eyebrow') }}
      </p>
      <h1>{{ project.title }}</h1>
      <p class="body-copy">
        {{ t('creativeProposal.description') }}
      </p>

      <section class="proposal-generation-bar">
        <div
          v-if="hasAIProviders"
          class="generation-controls"
        >
          <div class="control-group">
            <label for="provider-select">{{ t('creativeProposal.generation.selectProvider') }}</label>
            <select
              id="provider-select"
              v-model="selectedProviderId"
              data-testid="select-provider"
              :disabled="dirty || isGenerationInProgress || generating"
              @change="onProviderChange(($event.target as HTMLSelectElement).value)"
            >
              <option
                v-for="p in providerOptions"
                :key="p.id"
                :value="p.id"
              >
                {{ p.display_name }}
              </option>
            </select>
          </div>
          <div class="control-group">
            <label for="model-select">{{ t('creativeProposal.generation.selectModel') }}</label>
            <select
              id="model-select"
              v-model="selectedModelId"
              data-testid="select-model"
              :disabled="dirty || isGenerationInProgress || generating"
            >
              <option
                v-for="m in selectedProviderModels"
                :key="m.id"
                :value="m.id"
              >
                {{ m.display_name }}
              </option>
            </select>
          </div>
          <button
            class="primary-button"
            data-testid="generate-proposal-btn"
            type="button"
            :disabled="dirty || !selectedModelId || isGenerationInProgress || generating || hasSucceededJobUnloaded"
            @click="startGeneration"
          >
            {{ (isGenerationInProgress || generating) ? t('creativeProposal.actions.generating') : (hasProposal ? t('creativeProposal.actions.regenerate') : t('creativeProposal.actions.generate')) }}
          </button>
        </div>
        <div
          v-else
          class="notice info"
          data-testid="no-providers-notice"
        >
          <p>{{ t('creativeProposal.generation.noProvidersConfigured') }}</p>
        </div>

        <div
          v-if="dirty"
          class="notice warning"
          data-testid="dirty-generation-blocked"
        >
          <p>{{ t('creativeProposal.generation.dirtyBlocked') }}</p>
        </div>

        <div
          v-if="isGenerationInProgress && activeJob"
          class="notice info"
          data-testid="job-progress-banner"
        >
          <p>{{ activeJob.state === 'queued' ? t('creativeProposal.generation.stateQueued') : t('creativeProposal.generation.stateRunning') }}</p>
        </div>

        <div
          v-if="hasSucceededJobUnloaded"
          class="notice info"
          data-testid="job-succeeded-load-failed-banner"
        >
          <p>{{ t('creativeProposal.generation.stateSucceeded') }}</p>
          <p
            v-if="generationErrorCode"
            class="state-text"
          >
            {{ t(`creativeProposal.errors.${generationErrorCode}`) }}
          </p>
          <button
            class="secondary-button"
            data-testid="retry-load-generated-btn"
            type="button"
            @click="loadSucceededProposal"
          >
            {{ t('creativeProposal.actions.retryLoadGenerated') }}
          </button>
        </div>

        <div
          v-if="generationErrorCode && !hasSucceededJobUnloaded"
          class="notice error"
          data-testid="job-error-banner"
        >
          <p>{{ t(`creativeProposal.errors.${generationErrorCode}`) }}</p>
          <button
            v-if="hasAIProviders && !isGenerationInProgress"
            class="secondary-button"
            data-testid="retry-generation-btn"
            type="button"
            :disabled="dirty"
            @click="startGeneration"
          >
            {{ t('creativeProposal.actions.retryGeneration') }}
          </button>
        </div>
      </section>

      <div
        v-if="loadErrorCode && !hasProposal"
        class="notice error"
      >
        <p>{{ t(`creativeProposal.errors.${loadErrorCode}`) }}</p>
        <button
          class="secondary-button"
          data-testid="retry-proposal-list"
          type="button"
          @click="loadWorkspace"
        >
          {{ t('projects.actions.retry') }}
        </button>
      </div>

      <div
        v-else-if="!hasProposal"
        class="notice info"
      >
        <p>{{ t('creativeProposal.states.empty') }}</p>
        <p>{{ t('creativeProposal.states.emptyDetail') }}</p>
      </div>

      <div
        v-else
        class="proposal-workspace"
      >
        <aside
          class="proposal-history"
          :aria-label="t('creativeProposal.history.label')"
        >
          <h2>{{ t('creativeProposal.history.title') }}</h2>
          <button
            v-for="summary in summaries"
            :key="summary.version"
            class="proposal-version-button"
            :class="{ active: summary.version === selectedVersion }"
            type="button"
            :data-testid="`version-${summary.version}`"
            @click="loadVersion(summary.version)"
          >
            <span>{{ t('creativeProposal.states.version', { value: summary.version }) }}</span>
            <small>{{ t(`creativeProposal.status.${summary.status}`) }}</small>
          </button>
        </aside>

        <section
          class="proposal-editor"
          aria-live="polite"
        >
          <p
            v-if="proposalLoading"
            class="state-text"
          >
            {{ t('creativeProposal.states.loadingVersion') }}
          </p>
          <template v-if="selectedProposal">
            <div class="proposal-meta">
              <p>
                {{ t('creativeProposal.states.version', { value: selectedProposal.version }) }} ·
                {{ t('creativeProposal.states.revision', { value: selectedProposal.revision }) }} ·
                {{ t('creativeProposal.states.sourceBriefRevision', { value: selectedProposal.source_brief_revision }) }}
              </p>
              <p>{{ t('creativeProposal.states.updatedAt', { value: d(new Date(selectedProposal.updated_at), 'long') }) }}</p>
            </div>

            <ul class="proposal-title-options">
              <li
                v-for="title in selectedProposal.title_options"
                :key="title"
              >
                {{ title }}
              </li>
            </ul>

            <div
              v-if="saved && !dirty"
              class="notice success"
            >
              {{ t('creativeProposal.states.saved') }}
            </div>
            <div
              v-else-if="dirty"
              class="notice info"
            >
              {{ t('creativeProposal.states.unsavedChanges') }}
            </div>
            <div
              v-if="pendingVersion !== null"
              class="notice warning"
            >
              <p>{{ t('creativeProposal.states.dirtySwitchBlocked') }}</p>
              <button
                class="secondary-button"
                data-testid="discard-version-switch"
                type="button"
                @click="discardAndSwitchVersion"
              >
                {{ t('creativeProposal.actions.discardAndSwitch') }}
              </button>
            </div>
            <div
              v-if="staleConflict"
              class="notice warning"
            >
              <p>{{ t('creativeProposal.states.staleConflict') }}</p>
              <button
                class="secondary-button"
                data-testid="reload-latest-proposal"
                type="button"
                @click="reloadSelected"
              >
                {{ t('creativeProposal.actions.reloadLatest') }}
              </button>
            </div>
            <div
              v-if="loadErrorCode"
              class="notice error"
            >
              <p>{{ t(`creativeProposal.errors.${loadErrorCode}`) }}</p>
              <button
                class="secondary-button"
                data-testid="retry-proposal-load"
                type="button"
                @click="retryFailedVersion"
              >
                {{ t('projects.actions.retry') }}
              </button>
            </div>
            <div
              v-if="mutationErrorCode"
              class="notice error"
            >
              <p>{{ t(`creativeProposal.errors.${mutationErrorCode}`) }}</p>
            </div>
            <div
              v-if="isReadOnly"
              class="notice info"
            >
              {{ t('creativeProposal.states.readOnly') }}
            </div>

            <div
              v-if="selectedProposal.status === 'draft' && !confirmApproval"
              class="proposal-actions"
            >
              <button
                class="primary-button"
                data-testid="approve-proposal"
                type="button"
                :disabled="dirty || submitting || approving"
                @click="confirmApproval = true"
              >
                {{ t('creativeProposal.actions.approve') }}
              </button>
            </div>
            <div
              v-else-if="confirmApproval"
              class="notice warning"
            >
              <p>{{ t('creativeProposal.states.confirmApproval') }}</p>
              <button
                class="primary-button"
                data-testid="confirm-approve"
                type="button"
                :disabled="approving"
                @click="approveSelected"
              >
                {{ approving ? t('creativeProposal.actions.approving') : t('creativeProposal.actions.confirmApprove') }}
              </button>
              <button
                class="secondary-button"
                type="button"
                :disabled="approving"
                @click="confirmApproval = false"
              >
                {{ t('creativeProposal.actions.cancel') }}
              </button>
            </div>

            <CreativeProposalForm
              :initial-values="formValues"
              :submitting="submitting"
              :submit-label="t('creativeProposal.actions.save')"
              :field-errors="fieldErrors"
              :disabled="isReadOnly || proposalLoading"
              @dirty-change="onDirtyChange"
              @submit="submit"
            />
          </template>
          <div
            v-else-if="loadErrorCode"
            class="notice error"
          >
            <p>{{ t(`creativeProposal.errors.${loadErrorCode}`) }}</p>
            <button
              class="secondary-button"
              data-testid="retry-proposal-load"
              type="button"
              @click="retryFailedVersion"
            >
              {{ t('projects.actions.retry') }}
            </button>
          </div>
        </section>
      </div>
    </template>
  </section>
</template>
