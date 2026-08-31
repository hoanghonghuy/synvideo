<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import type {
  AspectRatio,
  ContentFormat,
  Project,
  ProjectLocale,
  ProjectPayload,
  ProjectStatus,
} from '@/api/projects'

type ProjectFormPayload = ProjectPayload & {
  status?: ProjectStatus
}

const props = defineProps<{
  initialProject?: Project | null
  includeStatus?: boolean
  submitting: boolean
  submitLabel: string
  fieldErrors?: Record<string, string>
}>()

const emit = defineEmits<{
  submit: [ProjectFormPayload]
}>()

const { t } = useI18n()

const contentFormats: ContentFormat[] = ['short', 'long', 'flexible']
const aspectRatios: AspectRatio[] = ['16:9', '9:16', '1:1', '4:5']
const locales: ProjectLocale[] = ['vi', 'en']
const statuses: ProjectStatus[] = ['active', 'archived']

const form = reactive({
  title: '',
  description: '',
  contentFormat: 'short' as ContentFormat,
  aspectRatio: '9:16' as AspectRatio,
  targetDurationSeconds: '',
  locale: 'vi' as ProjectLocale,
  status: 'active' as ProjectStatus,
})

const clientError = reactive({
  duration: '',
})

watch(
  () => props.initialProject,
  (project) => {
    if (!project) {
      return
    }
    form.title = project.title
    form.description = project.description
    form.contentFormat = project.content_format
    form.aspectRatio = project.aspect_ratio
    form.targetDurationSeconds = project.target_duration_seconds?.toString() ?? ''
    form.locale = project.locale
    form.status = project.status
  },
  { immediate: true },
)

const durationError = computed(() => clientError.duration || props.fieldErrors?.target_duration_seconds)

function onSubmit() {
  clientError.duration = ''
  const trimmedDuration = form.targetDurationSeconds.trim()
  let duration: number | null = null
  if (trimmedDuration !== '') {
    const parsed = Number(trimmedDuration)
    if (!Number.isInteger(parsed)) {
      clientError.duration = 'invalid'
      return
    }
    duration = parsed
  }

  const payload: ProjectFormPayload = {
    title: form.title,
    description: form.description,
    content_format: form.contentFormat,
    aspect_ratio: form.aspectRatio,
    target_duration_seconds: duration,
    locale: form.locale,
  }
  if (props.includeStatus) {
    payload.status = form.status
  }
  emit('submit', payload)
}
</script>

<template>
  <form
    class="project-form"
    @submit.prevent="onSubmit"
  >
    <label class="field">
      <span>{{ t('projects.fields.title') }}</span>
      <input
        v-model="form.title"
        name="title"
      >
      <small v-if="fieldErrors?.title">{{ t(`projects.validation.${fieldErrors.title}`) }}</small>
    </label>

    <label class="field">
      <span>{{ t('projects.fields.description') }}</span>
      <textarea
        v-model="form.description"
        name="description"
        rows="5"
      />
      <small v-if="fieldErrors?.description">
        {{ t(`projects.validation.${fieldErrors.description}`) }}
      </small>
    </label>

    <div class="form-grid">
      <label class="field">
        <span>{{ t('projects.fields.contentFormat') }}</span>
        <select
          v-model="form.contentFormat"
          name="content_format"
        >
          <option
            v-for="value in contentFormats"
            :key="value"
            :value="value"
          >
            {{ t(`projects.contentFormat.${value}`) }}
          </option>
        </select>
        <small v-if="fieldErrors?.content_format">
          {{ t(`projects.validation.${fieldErrors.content_format}`) }}
        </small>
      </label>

      <label class="field">
        <span>{{ t('projects.fields.aspectRatio') }}</span>
        <select
          v-model="form.aspectRatio"
          name="aspect_ratio"
        >
          <option
            v-for="value in aspectRatios"
            :key="value"
            :value="value"
          >
            {{ value }}
          </option>
        </select>
        <small v-if="fieldErrors?.aspect_ratio">
          {{ t(`projects.validation.${fieldErrors.aspect_ratio}`) }}
        </small>
      </label>

      <label class="field">
        <span>{{ t('projects.fields.duration') }}</span>
        <input
          v-model="form.targetDurationSeconds"
          inputmode="numeric"
          name="duration"
        >
        <small v-if="durationError">{{ t(`projects.validation.${durationError}`) }}</small>
      </label>

      <label class="field">
        <span>{{ t('projects.fields.locale') }}</span>
        <select
          v-model="form.locale"
          name="locale"
        >
          <option
            v-for="value in locales"
            :key="value"
            :value="value"
          >
            {{ t(`projects.locale.${value}`) }}
          </option>
        </select>
        <small v-if="fieldErrors?.locale">{{ t(`projects.validation.${fieldErrors.locale}`) }}</small>
      </label>

      <label
        v-if="includeStatus"
        class="field"
      >
        <span>{{ t('projects.fields.status') }}</span>
        <select
          v-model="form.status"
          name="status"
        >
          <option
            v-for="value in statuses"
            :key="value"
            :value="value"
          >
            {{ t(`projects.status.${value}`) }}
          </option>
        </select>
        <small v-if="fieldErrors?.status">{{ t(`projects.validation.${fieldErrors.status}`) }}</small>
      </label>
    </div>

    <button
      class="primary-button"
      type="submit"
      :disabled="submitting"
    >
      {{ submitting ? t('projects.actions.submitting') : submitLabel }}
    </button>
  </form>
</template>
