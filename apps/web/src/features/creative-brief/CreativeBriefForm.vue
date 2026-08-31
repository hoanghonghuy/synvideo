<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import type { CreativeBriefPayload, DistributionTarget } from './api'

export interface CreativeBriefFormState extends CreativeBriefPayload {
  mustIncludeText: string
  mustAvoidText: string
}

const props = defineProps<{
  initialValues: CreativeBriefFormState
  submitting: boolean
  submitLabel: string
  fieldErrors?: Record<string, string>
  disabled?: boolean
}>()

const emit = defineEmits<{
  submit: [CreativeBriefPayload]
}>()

const { t } = useI18n()

const distributionTargets: DistributionTarget[] = ['youtube', 'tiktok', 'instagram', 'other']

const form = reactive({
  sourceText: '',
  targetAudience: '',
  objective: '',
  desiredStyle: '',
  tone: '',
  distributionTargets: [] as DistributionTarget[],
  callToAction: '',
  mustIncludeText: '',
  mustAvoidText: '',
})

watch(
  () => props.initialValues,
  (values) => {
    form.sourceText = values.source_text
    form.targetAudience = values.target_audience
    form.objective = values.objective
    form.desiredStyle = values.desired_style
    form.tone = values.tone
    form.distributionTargets = [...values.distribution_targets]
    form.callToAction = values.call_to_action
    form.mustIncludeText = values.mustIncludeText
    form.mustAvoidText = values.mustAvoidText
  },
  { immediate: true, deep: true },
)

const isTargetSelected = computed(() => {
  const selected = new Set(form.distributionTargets)
  return (target: DistributionTarget) => selected.has(target)
})

function toggleTarget(target: DistributionTarget) {
  if (props.disabled) {
    return
  }
  if (isTargetSelected.value(target)) {
    form.distributionTargets = form.distributionTargets.filter((value) => value !== target)
    return
  }
  form.distributionTargets = [...form.distributionTargets, target]
}

function splitLines(value: string): string[] {
  return value
    .split('\n')
    .map((line) => line.trim())
    .filter((line) => line !== '')
}

function onSubmit() {
  emit('submit', {
    source_text: form.sourceText,
    target_audience: form.targetAudience,
    objective: form.objective,
    desired_style: form.desiredStyle,
    tone: form.tone,
    distribution_targets: [...form.distributionTargets],
    call_to_action: form.callToAction,
    must_include: splitLines(form.mustIncludeText),
    must_avoid: splitLines(form.mustAvoidText),
  })
}
</script>

<template>
  <form
    class="project-form creative-brief-form"
    @submit.prevent="onSubmit"
  >
    <label class="field">
      <span>{{ t('creativeBrief.fields.sourceText') }}</span>
      <textarea
        v-model="form.sourceText"
        name="source_text"
        rows="8"
        :disabled="disabled"
      />
      <small v-if="fieldErrors?.source_text">
        {{ t(`creativeBrief.validation.${fieldErrors.source_text}`) }}
      </small>
    </label>

    <div class="form-grid">
      <label class="field">
        <span>{{ t('creativeBrief.fields.targetAudience') }}</span>
        <input
          v-model="form.targetAudience"
          name="target_audience"
          :disabled="disabled"
        >
        <small v-if="fieldErrors?.target_audience">
          {{ t(`creativeBrief.validation.${fieldErrors.target_audience}`) }}
        </small>
      </label>

      <label class="field">
        <span>{{ t('creativeBrief.fields.objective') }}</span>
        <input
          v-model="form.objective"
          name="objective"
          :disabled="disabled"
        >
        <small v-if="fieldErrors?.objective">
          {{ t(`creativeBrief.validation.${fieldErrors.objective}`) }}
        </small>
      </label>

      <label class="field">
        <span>{{ t('creativeBrief.fields.desiredStyle') }}</span>
        <input
          v-model="form.desiredStyle"
          name="desired_style"
          :disabled="disabled"
        >
        <small v-if="fieldErrors?.desired_style">
          {{ t(`creativeBrief.validation.${fieldErrors.desired_style}`) }}
        </small>
      </label>

      <label class="field">
        <span>{{ t('creativeBrief.fields.tone') }}</span>
        <input
          v-model="form.tone"
          name="tone"
          :disabled="disabled"
        >
        <small v-if="fieldErrors?.tone">
          {{ t(`creativeBrief.validation.${fieldErrors.tone}`) }}
        </small>
      </label>
    </div>

    <fieldset class="field distribution-fieldset">
      <legend>{{ t('creativeBrief.fields.distributionTargets') }}</legend>
      <div class="checkbox-grid">
        <label
          v-for="target in distributionTargets"
          :key="target"
          class="checkbox-field"
        >
          <input
            type="checkbox"
            :checked="isTargetSelected(target)"
            :disabled="disabled"
            @change="toggleTarget(target)"
          >
          <span>{{ t(`creativeBrief.distributionTargets.${target}`) }}</span>
        </label>
      </div>
      <small v-if="fieldErrors?.distribution_targets">
        {{ t(`creativeBrief.validation.${fieldErrors.distribution_targets}`) }}
      </small>
    </fieldset>

    <label class="field">
      <span>{{ t('creativeBrief.fields.callToAction') }}</span>
      <input
        v-model="form.callToAction"
        name="call_to_action"
        :disabled="disabled"
      >
      <small v-if="fieldErrors?.call_to_action">
        {{ t(`creativeBrief.validation.${fieldErrors.call_to_action}`) }}
      </small>
    </label>

    <label class="field">
      <span>{{ t('creativeBrief.fields.mustInclude') }}</span>
      <textarea
        v-model="form.mustIncludeText"
        name="must_include"
        rows="4"
        :disabled="disabled"
      />
      <small class="field-help">{{ t('creativeBrief.help.lineItems') }}</small>
      <small v-if="fieldErrors?.must_include">
        {{ t(`creativeBrief.validation.${fieldErrors.must_include}`) }}
      </small>
    </label>

    <label class="field">
      <span>{{ t('creativeBrief.fields.mustAvoid') }}</span>
      <textarea
        v-model="form.mustAvoidText"
        name="must_avoid"
        rows="4"
        :disabled="disabled"
      />
      <small class="field-help">{{ t('creativeBrief.help.lineItems') }}</small>
      <small v-if="fieldErrors?.must_avoid">
        {{ t(`creativeBrief.validation.${fieldErrors.must_avoid}`) }}
      </small>
    </label>

    <button
      class="primary-button"
      type="submit"
      :disabled="submitting || disabled"
    >
      {{ submitting ? t('creativeBrief.actions.submitting') : submitLabel }}
    </button>
  </form>
</template>
