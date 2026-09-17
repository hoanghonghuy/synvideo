<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue'
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
const formElement = ref<HTMLFormElement | null>(null)
const durationInput = ref<HTMLInputElement | null>(null)

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

const clientError = reactive({ duration: '' })

watch(
  () => props.initialProject,
  (project) => {
    if (!project) return
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

watch(
  () => props.fieldErrors,
  async (errors) => {
    if (!errors || Object.keys(errors).length === 0) return
    await nextTick()
    formElement.value?.querySelector<HTMLElement>('[aria-invalid="true"]')?.focus()
  },
)

const durationError = computed(() => clientError.duration || props.fieldErrors?.target_duration_seconds)
const errorId = (field: string) => `project-${field}-error`
const hasError = (field: string) => Boolean(props.fieldErrors?.[field])

async function onSubmit() {
  if (props.submitting) return

  clientError.duration = ''
  const trimmedDuration = form.targetDurationSeconds.trim()
  let duration: number | null = null
  if (trimmedDuration !== '') {
    const parsed = Number(trimmedDuration)
    if (!Number.isInteger(parsed)) {
      clientError.duration = 'invalid'
      await nextTick()
      durationInput.value?.focus()
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
  if (props.includeStatus) payload.status = form.status
  emit('submit', payload)
}
</script>

<template>
  <form ref="formElement" class="project-form" :aria-busy="submitting || undefined" @submit.prevent="onSubmit">
    <label class="field">
      <span>{{ t('projects.fields.title') }}</span>
      <input v-model="form.title" name="title" :aria-invalid="hasError('title') || undefined" :aria-describedby="hasError('title') ? errorId('title') : undefined">
      <small v-if="fieldErrors?.title" :id="errorId('title')">{{ t(`projects.validation.${fieldErrors.title}`) }}</small>
    </label>

    <label class="field">
      <span>{{ t('projects.fields.description') }}</span>
      <textarea v-model="form.description" name="description" rows="5" :aria-invalid="hasError('description') || undefined" :aria-describedby="hasError('description') ? errorId('description') : undefined" />
      <small v-if="fieldErrors?.description" :id="errorId('description')">{{ t(`projects.validation.${fieldErrors.description}`) }}</small>
    </label>

    <div class="form-grid">
      <label class="field">
        <span>{{ t('projects.fields.contentFormat') }}</span>
        <select v-model="form.contentFormat" name="content_format" :aria-invalid="hasError('content_format') || undefined" :aria-describedby="hasError('content_format') ? errorId('content-format') : undefined">
          <option v-for="value in contentFormats" :key="value" :value="value">{{ t(`projects.contentFormat.${value}`) }}</option>
        </select>
        <small v-if="fieldErrors?.content_format" :id="errorId('content-format')">{{ t(`projects.validation.${fieldErrors.content_format}`) }}</small>
      </label>

      <label class="field">
        <span>{{ t('projects.fields.aspectRatio') }}</span>
        <select v-model="form.aspectRatio" name="aspect_ratio" :aria-invalid="hasError('aspect_ratio') || undefined" :aria-describedby="hasError('aspect_ratio') ? errorId('aspect-ratio') : undefined">
          <option v-for="value in aspectRatios" :key="value" :value="value">{{ value }}</option>
        </select>
        <small v-if="fieldErrors?.aspect_ratio" :id="errorId('aspect-ratio')">{{ t(`projects.validation.${fieldErrors.aspect_ratio}`) }}</small>
      </label>

      <label class="field">
        <span>{{ t('projects.fields.duration') }}</span>
        <input ref="durationInput" v-model="form.targetDurationSeconds" inputmode="numeric" name="duration" :aria-invalid="Boolean(durationError) || undefined" :aria-describedby="durationError ? errorId('duration') : undefined">
        <small v-if="durationError" :id="errorId('duration')">{{ t(`projects.validation.${durationError}`) }}</small>
      </label>

      <label class="field">
        <span>{{ t('projects.fields.locale') }}</span>
        <select v-model="form.locale" name="locale" :aria-invalid="hasError('locale') || undefined" :aria-describedby="hasError('locale') ? errorId('locale') : undefined">
          <option v-for="value in locales" :key="value" :value="value">{{ t(`projects.locale.${value}`) }}</option>
        </select>
        <small v-if="fieldErrors?.locale" :id="errorId('locale')">{{ t(`projects.validation.${fieldErrors.locale}`) }}</small>
      </label>

      <label v-if="includeStatus" class="field">
        <span>{{ t('projects.fields.status') }}</span>
        <select v-model="form.status" name="status" :aria-invalid="hasError('status') || undefined" :aria-describedby="hasError('status') ? errorId('status') : undefined">
          <option v-for="value in statuses" :key="value" :value="value">{{ t(`projects.status.${value}`) }}</option>
        </select>
        <small v-if="fieldErrors?.status" :id="errorId('status')">{{ t(`projects.validation.${fieldErrors.status}`) }}</small>
      </label>
    </div>

    <button class="primary-button" type="submit" :aria-disabled="submitting || undefined">
      <span v-if="submitting" role="status" aria-live="polite" aria-atomic="true" data-testid="submit-status">
        {{ t('projects.actions.submitting') }}
      </span>
      <span v-else>{{ submitLabel }}</span>
    </button>
  </form>
</template>
