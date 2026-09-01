<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import type { CreativeProposalEditableContent, CreativeProposalStructureItem } from './api'

export interface CreativeProposalFormState {
  titleOptionsText: string
  hookOptionsText: string
  audience_summary: string
  objective_summary: string
  narrative_angle: string
  estimatedDurationText: string
  format_rationale: string
  structure: CreativeProposalStructureItem[]
  visual_direction: string
  voice_direction: string
  music_direction: string
  caption_direction: string
  call_to_action: string
  researchGapsText: string
  warningsText: string
}

const props = defineProps<{
  initialValues: CreativeProposalFormState
  submitting: boolean
  submitLabel: string
  fieldErrors?: Record<string, string>
  disabled?: boolean
}>()

const emit = defineEmits<{
  submit: [CreativeProposalEditableContent]
  'dirty-change': [boolean]
}>()

const { t } = useI18n()

const form = reactive({
  titleOptionsText: '',
  hookOptionsText: '',
  audienceSummary: '',
  objectiveSummary: '',
  narrativeAngle: '',
  estimatedDurationText: '',
  formatRationale: '',
  structure: [] as CreativeProposalStructureItem[],
  visualDirection: '',
  voiceDirection: '',
  musicDirection: '',
  captionDirection: '',
  callToAction: '',
  researchGapsText: '',
  warningsText: '',
})

const baseline = ref('')

watch(
  () => props.initialValues,
  (values) => {
    form.titleOptionsText = values.titleOptionsText
    form.hookOptionsText = values.hookOptionsText
    form.audienceSummary = values.audience_summary
    form.objectiveSummary = values.objective_summary
    form.narrativeAngle = values.narrative_angle
    form.estimatedDurationText = values.estimatedDurationText
    form.formatRationale = values.format_rationale
    form.structure = values.structure.map((item) => ({ ...item }))
    form.visualDirection = values.visual_direction
    form.voiceDirection = values.voice_direction
    form.musicDirection = values.music_direction
    form.captionDirection = values.caption_direction
    form.callToAction = values.call_to_action
    form.researchGapsText = values.researchGapsText
    form.warningsText = values.warningsText
    baseline.value = serializeFormState()
    emit('dirty-change', false)
  },
  { immediate: true, deep: true },
)

watch(
  form,
  () => {
    emit('dirty-change', serializeFormState() !== baseline.value)
  },
  { deep: true },
)

function addStructureItem() {
  if (props.disabled) {
    return
  }
  form.structure.push({ key: '', title: '', purpose: '' })
}

function removeStructureItem(index: number) {
  if (props.disabled || form.structure.length <= 1) {
    return
  }
  form.structure.splice(index, 1)
}

function onSubmit() {
  emit('submit', buildPayload())
}

function buildPayload(): CreativeProposalEditableContent {
  return {
    title_options: splitLines(form.titleOptionsText),
    hook_options: splitLines(form.hookOptionsText),
    audience_summary: form.audienceSummary,
    objective_summary: form.objectiveSummary,
    narrative_angle: form.narrativeAngle,
    estimated_duration_seconds: parseOptionalInteger(form.estimatedDurationText),
    format_rationale: form.formatRationale,
    structure: form.structure.map((item) => ({ ...item })),
    visual_direction: form.visualDirection,
    voice_direction: form.voiceDirection,
    music_direction: form.musicDirection,
    caption_direction: form.captionDirection,
    call_to_action: form.callToAction,
    research_gaps: splitLines(form.researchGapsText),
    warnings: splitLines(form.warningsText),
  }
}

function splitLines(value: string): string[] {
  return value
    .split('\n')
    .map((line) => line.trim())
    .filter((line) => line !== '')
}

function parseOptionalInteger(value: string): number | null {
  const trimmed = value.trim()
  return trimmed === '' ? null : Number(trimmed)
}

function serializeFormState(): string {
  return JSON.stringify(buildPayload())
}
</script>

<template>
  <form
    class="project-form creative-proposal-form"
    @submit.prevent="onSubmit"
  >
    <label class="field">
      <span>{{ t('creativeProposal.fields.titleOptions') }}</span>
      <textarea
        v-model="form.titleOptionsText"
        name="title_options"
        rows="4"
        :disabled="disabled"
      />
      <small class="field-help">{{ t('creativeProposal.help.lineItems') }}</small>
      <small v-if="fieldErrors?.title_options">
        {{ t(`creativeProposal.validation.${fieldErrors.title_options}`) }}
      </small>
    </label>

    <label class="field">
      <span>{{ t('creativeProposal.fields.hookOptions') }}</span>
      <textarea
        v-model="form.hookOptionsText"
        name="hook_options"
        rows="4"
        :disabled="disabled"
      />
      <small class="field-help">{{ t('creativeProposal.help.lineItems') }}</small>
      <small v-if="fieldErrors?.hook_options">
        {{ t(`creativeProposal.validation.${fieldErrors.hook_options}`) }}
      </small>
    </label>

    <div class="form-grid">
      <label class="field">
        <span>{{ t('creativeProposal.fields.audienceSummary') }}</span>
        <textarea
          v-model="form.audienceSummary"
          name="audience_summary"
          rows="5"
          :disabled="disabled"
        />
        <small v-if="fieldErrors?.audience_summary">
          {{ t(`creativeProposal.validation.${fieldErrors.audience_summary}`) }}
        </small>
      </label>

      <label class="field">
        <span>{{ t('creativeProposal.fields.objectiveSummary') }}</span>
        <textarea
          v-model="form.objectiveSummary"
          name="objective_summary"
          rows="5"
          :disabled="disabled"
        />
        <small v-if="fieldErrors?.objective_summary">
          {{ t(`creativeProposal.validation.${fieldErrors.objective_summary}`) }}
        </small>
      </label>
    </div>

    <label class="field">
      <span>{{ t('creativeProposal.fields.narrativeAngle') }}</span>
      <textarea
        v-model="form.narrativeAngle"
        name="narrative_angle"
        rows="5"
        :disabled="disabled"
      />
      <small v-if="fieldErrors?.narrative_angle">
        {{ t(`creativeProposal.validation.${fieldErrors.narrative_angle}`) }}
      </small>
    </label>

    <div class="form-grid">
      <label class="field">
        <span>{{ t('creativeProposal.fields.estimatedDuration') }}</span>
        <input
          v-model="form.estimatedDurationText"
          name="estimated_duration_seconds"
          type="number"
          min="1"
          max="43200"
          :disabled="disabled"
        >
        <small v-if="fieldErrors?.estimated_duration_seconds">
          {{ t(`creativeProposal.validation.${fieldErrors.estimated_duration_seconds}`) }}
        </small>
      </label>

      <label class="field">
        <span>{{ t('creativeProposal.fields.formatRationale') }}</span>
        <textarea
          v-model="form.formatRationale"
          name="format_rationale"
          rows="4"
          :disabled="disabled"
        />
        <small v-if="fieldErrors?.format_rationale">
          {{ t(`creativeProposal.validation.${fieldErrors.format_rationale}`) }}
        </small>
      </label>
    </div>

    <fieldset class="field proposal-structure-fieldset">
      <legend>{{ t('creativeProposal.fields.structure') }}</legend>
      <div
        v-for="(item, index) in form.structure"
        :key="index"
        class="proposal-structure-item"
      >
        <label>
          <span>{{ t('creativeProposal.fields.structureKey') }}</span>
          <input
            v-model="item.key"
            :name="`structure_${index}_key`"
            :disabled="disabled"
          >
        </label>
        <label>
          <span>{{ t('creativeProposal.fields.structureTitle') }}</span>
          <input
            v-model="item.title"
            :name="`structure_${index}_title`"
            :disabled="disabled"
          >
        </label>
        <label>
          <span>{{ t('creativeProposal.fields.structurePurpose') }}</span>
          <textarea
            v-model="item.purpose"
            :name="`structure_${index}_purpose`"
            rows="3"
            :disabled="disabled"
          />
        </label>
        <button
          class="secondary-button"
          type="button"
          :disabled="disabled || form.structure.length <= 1"
          @click="removeStructureItem(index)"
        >
          {{ t('creativeProposal.actions.removeStructureItem') }}
        </button>
      </div>
      <button
        class="secondary-button"
        type="button"
        :disabled="disabled"
        @click="addStructureItem"
      >
        {{ t('creativeProposal.actions.addStructureItem') }}
      </button>
      <small v-if="fieldErrors?.structure">
        {{ t(`creativeProposal.validation.${fieldErrors.structure}`) }}
      </small>
    </fieldset>

    <label class="field">
      <span>{{ t('creativeProposal.fields.visualDirection') }}</span>
      <textarea
        v-model="form.visualDirection"
        name="visual_direction"
        rows="4"
        :disabled="disabled"
      />
    </label>

    <div class="form-grid">
      <label class="field">
        <span>{{ t('creativeProposal.fields.voiceDirection') }}</span>
        <textarea
          v-model="form.voiceDirection"
          name="voice_direction"
          rows="4"
          :disabled="disabled"
        />
      </label>

      <label class="field">
        <span>{{ t('creativeProposal.fields.musicDirection') }}</span>
        <textarea
          v-model="form.musicDirection"
          name="music_direction"
          rows="4"
          :disabled="disabled"
        />
      </label>
    </div>

    <div class="form-grid">
      <label class="field">
        <span>{{ t('creativeProposal.fields.captionDirection') }}</span>
        <textarea
          v-model="form.captionDirection"
          name="caption_direction"
          rows="4"
          :disabled="disabled"
        />
      </label>

      <label class="field">
        <span>{{ t('creativeProposal.fields.callToAction') }}</span>
        <textarea
          v-model="form.callToAction"
          name="call_to_action"
          rows="4"
          :disabled="disabled"
        />
      </label>
    </div>

    <label class="field">
      <span>{{ t('creativeProposal.fields.researchGaps') }}</span>
      <textarea
        v-model="form.researchGapsText"
        name="research_gaps"
        rows="4"
        :disabled="disabled"
      />
      <small class="field-help">{{ t('creativeProposal.help.lineItems') }}</small>
    </label>

    <label class="field">
      <span>{{ t('creativeProposal.fields.warnings') }}</span>
      <textarea
        v-model="form.warningsText"
        name="warnings"
        rows="4"
        :disabled="disabled"
      />
      <small class="field-help">{{ t('creativeProposal.help.lineItems') }}</small>
    </label>

    <button
      class="primary-button"
      type="submit"
      :disabled="submitting || disabled"
    >
      {{ submitting ? t('creativeProposal.actions.saving') : submitLabel }}
    </button>
  </form>
</template>
