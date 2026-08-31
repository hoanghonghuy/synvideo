<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'

import { ApiError, getProject, type Project } from '@/api/projects'
import CreativeBriefForm, { type CreativeBriefFormState } from './CreativeBriefForm.vue'
import {
  CreativeBriefNotFoundError,
  getCreativeBrief,
  putCreativeBrief,
  type CreativeBrief,
  type CreativeBriefPayload,
} from './api'

const { t } = useI18n()
const route = useRoute()

const project = ref<Project | null>(null)
const revision = ref<number | null>(null)
const loading = ref(true)
const submitting = ref(false)
const saved = ref(false)
const dirty = ref(false)
const isNewDraft = ref(false)
const staleConflict = ref(false)
const errorCode = ref('')
const fieldErrors = ref<Record<string, string>>({})
const formValues = ref<CreativeBriefFormState>(emptyFormState())

const pageTitle = computed(() =>
  isNewDraft.value ? t('creativeBrief.states.newDraft') : t('creativeBrief.states.existing'),
)

onMounted(() => {
  void loadWorkspace()
})

async function loadWorkspace() {
  loading.value = true
  errorCode.value = ''
  saved.value = false
  dirty.value = false
  staleConflict.value = false
  try {
    project.value = await getProject(String(route.params.id))
    await loadBrief()
  } catch (error) {
    errorCode.value = error instanceof ApiError ? error.code : 'request_failed'
  } finally {
    loading.value = false
  }
}

async function loadBrief() {
  try {
    const brief = await getCreativeBrief(String(route.params.id))
    applyBrief(brief)
    isNewDraft.value = false
  } catch (error) {
    if (error instanceof CreativeBriefNotFoundError) {
      isNewDraft.value = true
      revision.value = null
      formValues.value = emptyFormState()
      return
    }
    throw error
  }
}

async function reloadLatest() {
  loading.value = true
  errorCode.value = ''
  staleConflict.value = false
  saved.value = false
  try {
    const brief = await getCreativeBrief(String(route.params.id))
    applyBrief(brief)
    isNewDraft.value = false
  } catch (error) {
    errorCode.value = error instanceof ApiError ? error.code : 'request_failed'
  } finally {
    loading.value = false
  }
}

function applyBrief(brief: CreativeBrief) {
  revision.value = brief.revision
  formValues.value = toFormState(brief)
  dirty.value = false
}

function onDirtyChange(isDirty: boolean) {
  dirty.value = isDirty
  if (isDirty) {
    saved.value = false
  }
}

async function submit(payload: CreativeBriefPayload) {
  submitting.value = true
  errorCode.value = ''
  fieldErrors.value = {}
  saved.value = false
  dirty.value = false
  staleConflict.value = false
  try {
    const body =
      revision.value === null
        ? payload
        : {
            ...payload,
            revision: revision.value,
          }
    const brief = await putCreativeBrief(String(route.params.id), body)
    applyBrief(brief)
    isNewDraft.value = false
    saved.value = true
  } catch (error) {
    if (error instanceof ApiError) {
      errorCode.value = error.code
      fieldErrors.value = error.fields
      if (error.code === 'STALE_REVISION') {
        staleConflict.value = true
      }
    } else {
      errorCode.value = 'request_failed'
    }
  } finally {
    submitting.value = false
  }
}

function emptyFormState(): CreativeBriefFormState {
  return {
    source_text: '',
    target_audience: '',
    objective: '',
    desired_style: '',
    tone: '',
    distribution_targets: [],
    call_to_action: '',
    must_include: [],
    must_avoid: [],
    mustIncludeText: '',
    mustAvoidText: '',
  }
}

function toFormState(brief: CreativeBrief): CreativeBriefFormState {
  return {
    source_text: brief.source_text,
    target_audience: brief.target_audience,
    objective: brief.objective,
    desired_style: brief.desired_style,
    tone: brief.tone,
    distribution_targets: [...brief.distribution_targets],
    call_to_action: brief.call_to_action,
    must_include: [...brief.must_include],
    must_avoid: [...brief.must_avoid],
    mustIncludeText: brief.must_include.join('\n'),
    mustAvoidText: brief.must_avoid.join('\n'),
  }
}
</script>

<template>
  <section class="page">
    <RouterLink
      v-if="project"
      class="text-link"
      :to="`/projects/${project.id}`"
    >
      {{ t('creativeBrief.actions.backToProject') }}
    </RouterLink>

    <p
      v-if="loading"
      class="state-text"
    >
      {{ t('creativeBrief.states.loading') }}
    </p>
    <div
      v-else-if="errorCode && !project"
      class="notice error"
    >
      <p>{{ t(`projects.errors.${errorCode}`) }}</p>
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
        {{ t('creativeBrief.eyebrow') }}
      </p>
      <h1>{{ project.title }}</h1>
      <p class="body-copy">
        {{ pageTitle }}
        <span v-if="revision !== null">
          · {{ t('creativeBrief.states.revision', { value: revision }) }}
        </span>
      </p>

      <div
        v-if="saved && !dirty"
        class="notice success"
      >
        {{ t('creativeBrief.states.saved') }}
      </div>
      <div
        v-else-if="dirty"
        class="notice info"
      >
        {{ t('creativeBrief.states.unsavedChanges') }}
      </div>
      <div
        v-if="staleConflict"
        class="notice warning"
      >
        <p>{{ t('creativeBrief.states.staleConflict') }}</p>
        <button
          class="secondary-button"
          data-testid="reload-latest"
          type="button"
          @click="reloadLatest"
        >
          {{ t('creativeBrief.actions.reloadLatest') }}
        </button>
      </div>
      <div
        v-else-if="errorCode"
        class="notice error"
      >
        {{ t(errorCode === 'validation_failed' ? 'projects.errors.validation_failed' : `projects.errors.${errorCode}`) }}
      </div>

      <CreativeBriefForm
        :initial-values="formValues"
        :submitting="submitting"
        :submit-label="t('creativeBrief.actions.save')"
        :field-errors="fieldErrors"
        :disabled="loading"
        @dirty-change="onDirtyChange"
        @submit="submit"
      />
    </template>
  </section>
</template>
