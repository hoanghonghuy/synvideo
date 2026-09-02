<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  fetchProviderSettings,
  saveProviderSetting,
  deleteProviderSetting,
  ProviderApiError,
  type ProviderSettingView,
} from './api'

const { t } = useI18n()

const loading = ref(true)
const providers = ref<ProviderSettingView[]>([])
const generalError = ref<string | null>(null)
const successMessage = ref<string | null>(null)

// Local form state per provider
interface ProviderFormState {
  enabled: boolean
  selectedModels: Record<string, boolean>
  selectedVoices: Record<string, boolean>
  apiKeyInput: string
  showKey: boolean
  submitting: boolean
  deleting: boolean
  error: string | null
}

const formStates = reactive<Record<string, ProviderFormState>>({})

function getFormState(providerId: string): ProviderFormState {
  if (!formStates[providerId]) {
    formStates[providerId] = {
      enabled: false,
      selectedModels: {},
      selectedVoices: {},
      apiKeyInput: '',
      showKey: false,
      submitting: false,
      deleting: false,
      error: null,
    }
  }
  return formStates[providerId]
}

function initFormState(p: ProviderSettingView) {
  const modelsMap: Record<string, boolean> = {}
  p.models.forEach((m) => {
    modelsMap[m.id] = m.enabled
  })

  const voicesMap: Record<string, boolean> = {}
  p.voices.forEach((v) => {
    voicesMap[v.id] = v.enabled
  })

  formStates[p.id] = {
    enabled: p.enabled,
    selectedModels: modelsMap,
    selectedVoices: voicesMap,
    apiKeyInput: '',
    showKey: false,
    submitting: false,
    deleting: false,
    error: null,
  }
}

async function loadSettings(showSpinner = false) {
  if (showSpinner) {
    loading.value = true
  }
  generalError.value = null
  try {
    const res = await fetchProviderSettings()
    providers.value = res.providers
    res.providers.forEach(initFormState)
  } catch (err) {
    if (err instanceof ProviderApiError) {
      generalError.value = err.message
    } else {
      generalError.value = t('providerSettings.errors.fetchFailed')
    }
  } finally {
    if (showSpinner) {
      loading.value = false
    }
  }
}

async function handleSave(provider: ProviderSettingView) {
  const form = formStates[provider.id]
  if (!form) return

  form.error = null
  generalError.value = null
  successMessage.value = null

  const enabledModelIDs = Object.entries(form.selectedModels)
    .filter(([, isSelected]) => isSelected)
    .map(([id]) => id)
  const enabledVoiceIDs = Object.entries(form.selectedVoices)
    .filter(([, isSelected]) => isSelected)
    .map(([id]) => id)

  if (form.enabled && enabledModelIDs.length === 0 && enabledVoiceIDs.length === 0) {
    form.error = t('providerSettings.errors.atLeastOneModel')
    return
  }

  const rawKey = form.apiKeyInput
  if (!provider.configured && rawKey.length === 0) {
    form.error = t('providerSettings.errors.apiKeyRequired')
    return
  }

  form.submitting = true
  try {
    const payload: {
      revision?: number
      enabled: boolean
      enabled_text_model_ids: string[]
      enabled_image_model_ids: string[]
      enabled_voice_ids: string[]
      api_key?: string
    } = {
      enabled: form.enabled,
      enabled_text_model_ids: enabledModelIDs,
      enabled_image_model_ids: enabledModelIDs,
      enabled_voice_ids: enabledVoiceIDs,
    }

    if (provider.configured) {
      payload.revision = provider.revision
    }
    if (rawKey.length > 0) {
      payload.api_key = rawKey
    }

    const updated = await saveProviderSetting(provider.id, payload)

    // Clear API key input from memory immediately after save
    form.apiKeyInput = ''

    // Update local provider view
    const idx = providers.value.findIndex((p) => p.id === provider.id)
    if (idx !== -1) {
      providers.value[idx] = updated
      initFormState(updated)
    }

    successMessage.value = t('providerSettings.states.saved')
  } catch (err) {
    if (err instanceof ProviderApiError) {
      if (err.code === 'STALE_REVISION') {
        await loadSettings()
        getFormState(provider.id).error = t('providerSettings.errors.staleRevision')
      } else {
        form.error = err.message
      }
    } else {
      form.error = t('providerSettings.errors.saveFailed')
    }
  } finally {
    form.submitting = false
  }
}

async function handleDelete(provider: ProviderSettingView) {
  const form = formStates[provider.id]
  if (!form || !provider.configured) return

  const confirmed = window.confirm(
    t('providerSettings.actions.confirmDelete', { name: provider.display_name }),
  )
  if (!confirmed) return

  form.error = null
  generalError.value = null
  successMessage.value = null
  form.deleting = true

  try {
    await deleteProviderSetting(provider.id, provider.revision)
    successMessage.value = t('providerSettings.states.deleted')
    await loadSettings()
  } catch (err) {
    if (err instanceof ProviderApiError) {
      if (err.code === 'STALE_REVISION') {
        await loadSettings()
        getFormState(provider.id).error = t('providerSettings.errors.staleRevision')
      } else {
        form.error = err.message
      }
    } else {
      form.error = t('providerSettings.errors.deleteFailed')
    }
  } finally {
    form.deleting = false
  }
}

onMounted(() => {
  loadSettings(true)
})
</script>

<template>
  <div class="provider-settings-view">
    <header class="view-header">
      <p class="eyebrow">
        {{ t('providerSettings.eyebrow') }}
      </p>
      <h1 class="title">
        {{ t('providerSettings.title') }}
      </h1>
      <p class="description">
        {{ t('providerSettings.description') }}
      </p>
    </header>

    <div
      v-if="generalError"
      class="banner error"
      role="alert"
    >
      {{ generalError }}
    </div>
    <div
      v-if="successMessage"
      class="banner success"
      role="status"
    >
      {{ successMessage }}
    </div>

    <div
      v-if="loading"
      class="loading-state"
    >
      {{ t('providerSettings.states.loading') }}
    </div>

    <div
      v-else
      class="providers-list"
    >
      <div
        v-for="provider in providers"
        :key="provider.id"
        class="provider-card"
        :data-provider-id="provider.id"
      >
        <div class="provider-card-header">
          <div class="provider-info">
            <h2 class="provider-name">
              {{ provider.display_name }}
            </h2>
            <span class="provider-id">({{ provider.id }})</span>
          </div>

          <div class="badges">
            <span
              class="badge"
              :class="provider.configured ? 'badge-configured' : 'badge-unconfigured'"
            >
              {{
                provider.configured
                  ? t('providerSettings.configuredBadge')
                  : t('providerSettings.unconfiguredBadge')
              }}
            </span>
            <span
              v-if="provider.configured"
              class="badge"
              :class="provider.enabled ? 'badge-enabled' : 'badge-disabled'"
            >
              {{
                provider.enabled
                  ? t('providerSettings.enabledBadge')
                  : t('providerSettings.disabledBadge')
              }}
            </span>
            <span
              v-if="provider.configured"
              class="badge badge-revision"
            >
              {{ t('providerSettings.fields.revision', { value: provider.revision }) }}
            </span>
          </div>
        </div>

        <div
          v-if="getFormState(provider.id).error"
          class="provider-error"
          role="alert"
        >
          {{ getFormState(provider.id).error }}
        </div>

        <form
          class="provider-form"
          @submit.prevent="handleSave(provider)"
        >
          <div class="form-group toggle-group">
            <label class="checkbox-label toggle-label">
              <input
                v-model="getFormState(provider.id).enabled"
                type="checkbox"
                class="toggle-checkbox"
              >
              <span class="label-text">{{ t('providerSettings.fields.enabled') }}</span>
            </label>
          </div>

          <div class="form-group">
            <label class="group-label">{{ t('providerSettings.fields.models') }}</label>
            <div class="models-grid">
              <label
                v-for="model in provider.models"
                :key="model.id"
                class="checkbox-label model-option"
              >
                <input
                  v-model="getFormState(provider.id).selectedModels[model.id]"
                  type="checkbox"
                >
                <span class="model-name">{{ model.display_name }}</span>
                <span class="model-id">({{ model.id }})</span>
              </label>
            </div>
          </div>

          <div
            v-if="provider.voices && provider.voices.length > 0"
            class="form-group"
          >
            <label class="group-label">{{ t('providerSettings.fields.voices') }}</label>
            <div class="models-grid">
              <label
                v-for="voice in provider.voices"
                :key="voice.id"
                class="checkbox-label model-option"
              >
                <input
                  v-model="getFormState(provider.id).selectedVoices[voice.id]"
                  type="checkbox"
                >
                <span class="model-name">{{ voice.display_name }}</span>
                <span class="model-id">({{ voice.id }})</span>
              </label>
            </div>
          </div>

          <div class="form-group">
            <label
              :for="`api-key-${provider.id}`"
              class="group-label"
            >
              {{ t('providerSettings.fields.apiKey') }}
            </label>
            <div class="input-with-toggle">
              <input
                :id="`api-key-${provider.id}`"
                v-model="getFormState(provider.id).apiKeyInput"
                :type="getFormState(provider.id).showKey ? 'text' : 'password'"
                class="text-input"
                :placeholder="
                  provider.has_api_key
                    ? t('providerSettings.fields.apiKeyPreservedPlaceholder')
                    : t('providerSettings.fields.apiKeyPlaceholder')
                "
                autocomplete="off"
                spellcheck="false"
              >
              <button
                type="button"
                class="toggle-visibility-btn"
                @click="getFormState(provider.id).showKey = !getFormState(provider.id).showKey"
              >
                {{
                  getFormState(provider.id).showKey
                    ? t('providerSettings.actions.hideKey')
                    : t('providerSettings.actions.showKey')
                }}
              </button>
            </div>
          </div>

          <div class="form-actions">
            <button
              type="submit"
              class="btn btn-primary"
              :disabled="
                getFormState(provider.id).submitting || getFormState(provider.id).deleting
              "
            >
              {{
                getFormState(provider.id).submitting
                  ? t('providerSettings.actions.saving')
                  : t('providerSettings.actions.save')
              }}
            </button>

            <button
              v-if="provider.configured"
              type="button"
              class="btn btn-danger"
              :disabled="
                getFormState(provider.id).submitting || getFormState(provider.id).deleting
              "
              @click="handleDelete(provider)"
            >
              {{
                getFormState(provider.id).deleting
                  ? t('providerSettings.actions.deleting')
                  : t('providerSettings.actions.delete')
              }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<style scoped>
.provider-settings-view {
  max-width: 800px;
  margin: 0 auto;
  padding: 2rem 1rem;
}

.view-header {
  margin-bottom: 2rem;
}

.eyebrow {
  font-size: 0.875rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: #6366f1;
  margin-bottom: 0.5rem;
}

.title {
  font-size: 1.75rem;
  font-weight: 700;
  color: #111827;
  margin-bottom: 0.5rem;
}

.description {
  font-size: 0.95rem;
  color: #4b5563;
  line-height: 1.5;
}

.banner {
  padding: 0.875rem 1.25rem;
  border-radius: 6px;
  margin-bottom: 1.5rem;
  font-size: 0.9rem;
}

.banner.error,
.provider-error {
  background-color: #fef2f2;
  color: #b91c1c;
  border: 1px solid #f87171;
}

.provider-error {
  padding: 0.5rem 0.75rem;
  border-radius: 4px;
  margin-bottom: 1rem;
  font-size: 0.85rem;
}

.banner.success {
  background-color: #f0fdf4;
  color: #15803d;
  border: 1px solid #86efac;
}

.loading-state {
  text-align: center;
  padding: 3rem;
  color: #6b7280;
}

.providers-list {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.provider-card {
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  padding: 1.5rem;
  background-color: #ffffff;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.provider-card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1.25rem;
  flex-wrap: wrap;
  gap: 0.75rem;
}

.provider-info {
  display: flex;
  align-items: baseline;
  gap: 0.5rem;
}

.provider-name {
  font-size: 1.25rem;
  font-weight: 600;
  color: #1f2937;
  margin: 0;
}

.provider-id {
  font-size: 0.85rem;
  color: #6b7280;
  font-family: monospace;
}

.badges {
  display: flex;
  gap: 0.5rem;
  align-items: center;
}

.badge {
  font-size: 0.75rem;
  font-weight: 600;
  padding: 0.25rem 0.5rem;
  border-radius: 9999px;
}

.badge-configured {
  background-color: #dbeafe;
  color: #1e40af;
}

.badge-unconfigured {
  background-color: #f3f4f6;
  color: #4b5563;
}

.badge-enabled {
  background-color: #dcfce7;
  color: #166534;
}

.badge-disabled {
  background-color: #fee2e2;
  color: #991b1b;
}

.badge-revision {
  background-color: #f3e8ff;
  color: #6b21a8;
}

.form-group {
  margin-bottom: 1.25rem;
}

.group-label {
  display: block;
  font-size: 0.875rem;
  font-weight: 600;
  color: #374151;
  margin-bottom: 0.5rem;
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  cursor: pointer;
}

.models-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 0.75rem;
  background-color: #f9fafb;
  padding: 0.75rem;
  border-radius: 6px;
}

.model-option {
  font-size: 0.875rem;
}

.model-id {
  font-size: 0.75rem;
  color: #9ca3af;
  font-family: monospace;
}

.input-with-toggle {
  display: flex;
  gap: 0.5rem;
}

.text-input {
  flex: 1;
  padding: 0.5rem 0.75rem;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  font-size: 0.9rem;
}

.toggle-visibility-btn {
  padding: 0.5rem 0.75rem;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  background-color: #f9fafb;
  font-size: 0.8rem;
  cursor: pointer;
}

.form-actions {
  display: flex;
  gap: 0.75rem;
  margin-top: 1.5rem;
}

.btn {
  padding: 0.5rem 1rem;
  border-radius: 6px;
  font-weight: 500;
  font-size: 0.875rem;
  cursor: pointer;
  border: none;
  transition: background-color 0.15s ease;
}

.btn-primary {
  background-color: #4f46e5;
  color: #ffffff;
}

.btn-primary:hover:not(:disabled) {
  background-color: #4338ca;
}

.btn-danger {
  background-color: #ef4444;
  color: #ffffff;
}

.btn-danger:hover:not(:disabled) {
  background-color: #dc2626;
}

.btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}
</style>
