<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'

import { ApiError, getProject, type Project } from '@/api/projects'
import { getScenePlan, listScenePlans, type Scene, type ScenePlan, type ScenePlanSummary } from '@/features/scene-plan/api'
import {
  assignPrimaryVisual,
  deleteMediaAsset,
  listMediaAssets,
  listPrimaryVisualHistory,
  listSceneMediaBindings,
  mediaAssetContentURL,
  uploadMediaAsset,
  type MediaAsset,
  type SceneMediaEntry,
} from './api'

const { t, d } = useI18n()
const route = useRoute()
const project = ref<Project | null>(null)
const assets = ref<MediaAsset[]>([])
const plans = ref<ScenePlanSummary[]>([])
const selectedPlan = ref<ScenePlan | null>(null)
const targetPlanVersion = ref<number | null>(null)
const bindings = ref<Record<string, SceneMediaEntry>>({})
const histories = reactive<Record<string, SceneMediaEntry[]>>({})
const historyOpen = ref<Record<string, boolean>>({})
const loading = ref(true)
const mediaLoading = ref(true)
const mediaLoaded = ref(false)
const mediaErrorCode = ref('')
const planListErrorCode = ref('')
const planLoadErrorCode = ref('')
const planLoading = ref(false)
const assignmentErrors = ref<Record<string, string>>({})
const pendingRequests = ref<Set<string>>(new Set())
const deleteTarget = ref<MediaAsset | null>(null)
const deleteErrorCode = ref('')
const uploadErrorCode = ref('')
const uploading = ref(false)
const uploadProgress = ref(0)
const fileInput = ref<HTMLInputElement | null>(null)
const filterKind = ref('all')
const filterOrigin = ref('all')
const previewFailures = ref<string[]>([])
const uploadController = ref<AbortController | null>(null)

let workspaceLoadSeq = 0
let planLoadSeq = 0

const projectID = computed(() => String(route.params.id))
const approvedPlans = computed(() => plans.value.filter((plan) => plan.status === 'approved'))
const scenes = computed<Scene[]>(() => selectedPlan.value?.scenes ?? [])
const filteredAssets = computed(() => {
  return assets.value.filter((asset) => {
    return (
      (filterKind.value === 'all' || asset.kind === filterKind.value) &&
      (filterOrigin.value === 'all' || asset.origin === filterOrigin.value)
    )
  })
})
const visualAssets = computed(() => assets.value.filter((asset) => asset.kind === 'image' || asset.kind === 'video'))
const hasMediaEmptyState = computed(() => mediaLoaded.value && assets.value.length === 0)

watch(projectID, () => {
  uploadController.value?.abort()
  uploading.value = false
  void loadWorkspace()
})

onMounted(() => void loadWorkspace())
onUnmounted(() => uploadController.value?.abort())

async function loadWorkspace() {
  const seq = ++workspaceLoadSeq
  loading.value = true
  mediaLoading.value = true
  mediaLoaded.value = false
  mediaErrorCode.value = ''
  planListErrorCode.value = ''
  planLoadErrorCode.value = ''
  project.value = null
  assets.value = []
  plans.value = []
  selectedPlan.value = null
  targetPlanVersion.value = null
  bindings.value = {}
  Object.keys(histories).forEach((key) => delete histories[key])
  historyOpen.value = {}
  assignmentErrors.value = {}
  pendingRequests.value.clear()
  deleteTarget.value = null
  deleteErrorCode.value = ''
  previewFailures.value = []
  try {
    const currentProject = await getProject(projectID.value)
    if (seq !== workspaceLoadSeq) return
    project.value = currentProject
    const [mediaResult, planResult] = await Promise.allSettled([
      listMediaAssets(currentProject.id),
      listScenePlans(currentProject.id),
    ])
    if (seq !== workspaceLoadSeq) return
    if (mediaResult.status === 'fulfilled') {
      assets.value = mediaResult.value.assets ?? []
      mediaLoaded.value = true
    } else {
      mediaErrorCode.value = errorCode(mediaResult.reason)
    }
    mediaLoading.value = false
    if (planResult.status === 'fulfilled') {
      plans.value = planResult.value
      const approved = planResult.value.find((plan) => plan.status === 'approved')
      if (approved) await loadApprovedPlan(approved.version, seq)
    } else {
      planListErrorCode.value = errorCode(planResult.reason)
    }
  } catch (error) {
    if (seq === workspaceLoadSeq) planListErrorCode.value = errorCode(error)
  } finally {
    if (seq === workspaceLoadSeq) loading.value = false
  }
}

async function loadApprovedPlan(version: number, workspaceSeq = workspaceLoadSeq) {
  const seq = ++planLoadSeq
  targetPlanVersion.value = version
  planLoading.value = true
  planLoadErrorCode.value = ''
  selectedPlan.value = null
  bindings.value = {}
  Object.keys(histories).forEach((key) => delete histories[key])
  historyOpen.value = {}
  assignmentErrors.value = {}
  try {
    const [plan, currentBindings] = await Promise.all([
      getScenePlan(projectID.value, version),
      listSceneMediaBindings(projectID.value, version),
    ])
    if (workspaceSeq !== workspaceLoadSeq || seq !== planLoadSeq) return
    selectedPlan.value = plan
    applyBindings(currentBindings)
  } catch (error) {
    if (workspaceSeq === workspaceLoadSeq && seq === planLoadSeq) planLoadErrorCode.value = errorCode(error)
  } finally {
    if (workspaceSeq === workspaceLoadSeq && seq === planLoadSeq) planLoading.value = false
  }
}

function applyBindings(entries: SceneMediaEntry[]) {
  bindings.value = Object.fromEntries(entries.map((entry) => [entry.scene_key, entry]))
}

function bindingFor(sceneKey: string): SceneMediaEntry | undefined {
  return bindings.value[sceneKey]
}

function assetFor(sceneKey: string): MediaAsset | undefined {
  return bindingFor(sceneKey)?.asset
}

function isPending(sceneKey: string) {
  if (!selectedPlan.value) return false
  const token = `${workspaceLoadSeq}:${planLoadSeq}:${selectedPlan.value.version}:${sceneKey}`
  return pendingRequests.value.has(token)
}

async function assign(scene: Scene, asset: MediaAsset) {
  if (!selectedPlan.value || isPending(scene.key)) return
  const workspaceSeq = workspaceLoadSeq
  const version = selectedPlan.value.version
  const planSeq = planLoadSeq
  const token = `${workspaceSeq}:${planSeq}:${version}:${scene.key}`
  pendingRequests.value.add(token)
  assignmentErrors.value = { ...assignmentErrors.value, [scene.key]: '' }
  try {
    const updated = await assignPrimaryVisual(projectID.value, version, scene.key, asset.id)
    const [refreshed, history] = await Promise.all([
      listSceneMediaBindings(projectID.value, version),
      listPrimaryVisualHistory(projectID.value, version, scene.key),
    ])
    if (workspaceSeq !== workspaceLoadSeq || planSeq !== planLoadSeq || selectedPlan.value?.version !== version) return
    applyBindings(refreshed.some((entry) => entry.scene_key === scene.key && entry.binding) ? refreshed : [
      ...refreshed.filter((entry) => entry.scene_key !== scene.key),
      updated,
    ])
    histories[scene.key] = history
    historyOpen.value = { ...historyOpen.value, [scene.key]: true }
  } catch (error) {
    if (workspaceSeq === workspaceLoadSeq && planSeq === planLoadSeq && selectedPlan.value?.version === version) {
      assignmentErrors.value = { ...assignmentErrors.value, [scene.key]: errorCode(error) }
    }
  } finally {
    pendingRequests.value.delete(token)
  }
}

async function toggleHistory(scene: Scene) {
  const open = !historyOpen.value[scene.key]
  historyOpen.value = { ...historyOpen.value, [scene.key]: open }
  if (!open || histories[scene.key] || !selectedPlan.value) return
  const workspaceSeq = workspaceLoadSeq
  const planSeq = planLoadSeq
  const version = selectedPlan.value.version
  try {
    const history = await listPrimaryVisualHistory(projectID.value, version, scene.key)
    if (workspaceSeq === workspaceLoadSeq && planSeq === planLoadSeq && selectedPlan.value?.version === version) {
      histories[scene.key] = history
    }
  } catch (error) {
    if (workspaceSeq === workspaceLoadSeq && planSeq === planLoadSeq) {
      assignmentErrors.value = { ...assignmentErrors.value, [scene.key]: errorCode(error) }
    }
  }
}

function restore(scene: Scene, entry: SceneMediaEntry) {
  if (entry.asset) void assign(scene, entry.asset)
}

function requestDelete(asset: MediaAsset) {
  deleteTarget.value = asset
  deleteErrorCode.value = ''
}

function cancelDelete() {
  deleteTarget.value = null
  deleteErrorCode.value = ''
}

async function confirmDelete() {
  const target = deleteTarget.value
  if (!target) return
  deleteErrorCode.value = ''
  try {
    await deleteMediaAsset(projectID.value, target.id)
    assets.value = assets.value.filter((asset) => asset.id !== target.id)
    cancelDelete()
  } catch (error) {
    deleteErrorCode.value = errorCode(error)
  }
}

function openPicker() {
  fileInput.value?.click()
}

function onFileInput(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0]
  if (file) void upload(file)
  ;(event.target as HTMLInputElement).value = ''
}

function onDrop(event: DragEvent) {
  event.preventDefault()
  const file = event.dataTransfer?.files[0]
  if (file) void upload(file)
}

async function upload(file: File) {
  uploadErrorCode.value = ''
  if (!supportedMIME.has(file.type)) {
    uploadErrorCode.value = 'MEDIA_ASSET_UNSUPPORTED_TYPE'
    return
  }
  if (file.size > 100 * 1024 * 1024) {
    uploadErrorCode.value = 'MEDIA_ASSET_TOO_LARGE'
    return
  }
  const seq = workspaceLoadSeq
  uploadController.value?.abort()
  const controller = new AbortController()
  uploadController.value = controller
  uploading.value = true
  uploadProgress.value = 0
  try {
    const created = await uploadMediaAsset(projectID.value, file, {
      signal: controller.signal,
      onProgress: (progress) => {
        if (seq === workspaceLoadSeq) uploadProgress.value = progress
      },
    })
    if (seq !== workspaceLoadSeq) return
    assets.value = [created, ...assets.value.filter((asset) => asset.id !== created.id)]
    mediaLoaded.value = true
    uploadProgress.value = 100
  } catch (error) {
    if (seq === workspaceLoadSeq && !(error instanceof DOMException && error.name === 'AbortError')) {
      uploadErrorCode.value = errorCode(error)
    }
  } finally {
    if (uploadController.value === controller) uploadController.value = null
    if (seq === workspaceLoadSeq) uploading.value = false
  }
}

function cancelUpload() {
  uploadController.value?.abort()
  uploading.value = false
}

function markPreviewFailed(assetId: string) {
  if (!previewFailures.value.includes(assetId)) previewFailures.value = [...previewFailures.value, assetId]
}

function errorCode(error: unknown) {
  return error instanceof ApiError ? error.code : 'request_failed'
}

function safeFilename(asset: MediaAsset) {
  return asset.original_filename || t('media.asset.unnamed')
}

function formatBytes(bytes: number) {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

function technicalDetails(asset: MediaAsset) {
  const width = asset.metadata?.width
  const height = asset.metadata?.height
  if (typeof width === 'number' && typeof height === 'number') return `${width} × ${height}px`
  return ''
}

function errorText(scope: string, code: string) {
  return t(`media.${scope}.${code}`, t(`media.errors.${code}`))
}

const supportedMIME = new Set([
  'image/avif', 'image/gif', 'image/jpeg', 'image/png', 'image/webp',
  'video/mp4', 'video/quicktime', 'video/webm',
  'audio/aac', 'audio/flac', 'audio/mpeg', 'audio/mp4', 'audio/ogg', 'audio/wav', 'audio/x-wav',
])
</script>

<template>
  <section class="page media-page">
    <RouterLink
      class="text-link"
      :to="`/projects/${projectID}`"
    >
      {{ t('media.actions.backToProject') }}
    </RouterLink>

    <p
      v-if="loading"
      class="state-text"
    >
      {{ t('media.states.loading') }}
    </p>
    <div
      v-else-if="planListErrorCode && !project"
      class="notice error"
    >
      <p>{{ errorText('errors', planListErrorCode) }}</p>
      <button
        class="secondary-button"
        type="button"
        @click="loadWorkspace"
      >
        {{ t('media.actions.retry') }}
      </button>
    </div>
    <template v-else-if="project">
      <div class="page-heading">
        <div>
          <p class="eyebrow">
            {{ t('media.eyebrow') }}
          </p>
          <h1>{{ t('media.title', { project: project.title }) }}</h1>
          <p class="body-copy">
            {{ t('media.description') }}
          </p>
        </div>
      </div>

      <div
        class="media-upload-panel"
        @dragover.prevent
        @drop="onDrop"
      >
        <div>
          <h2>{{ t('media.upload.title') }}</h2>
          <p>{{ t('media.upload.help') }}</p>
          <p class="field-help">
            {{ t('media.upload.supported') }}
          </p>
        </div>
        <input
          ref="fileInput"
          class="visually-hidden"
          type="file"
          :accept="[...supportedMIME].join(',')"
          @change="onFileInput"
        >
        <div class="upload-actions">
          <button
            class="primary-button"
            type="button"
            :disabled="uploading"
            @click="openPicker"
          >
            {{ t('media.actions.chooseFile') }}
          </button>
          <button
            v-if="uploading"
            class="secondary-button"
            data-testid="cancel-upload"
            type="button"
            @click="cancelUpload"
          >
            {{ t('media.actions.cancelUpload') }}
          </button>
        </div>
        <div
          v-if="uploading"
          class="upload-progress"
          role="status"
        >
          <span>{{ t('media.upload.progress', { value: uploadProgress }) }}</span>
          <progress
            :value="uploadProgress"
            max="100"
          >
            {{ uploadProgress }}%
          </progress>
        </div>
        <div
          v-if="uploadErrorCode"
          class="notice error"
          data-testid="upload-error"
        >
          {{ errorText('errors', uploadErrorCode) }}
        </div>
      </div>

      <div
        v-if="mediaErrorCode"
        class="notice error"
        data-testid="media-library-error"
      >
        <p>{{ errorText('errors', mediaErrorCode) }}</p>
        <button
          class="secondary-button"
          type="button"
          @click="loadWorkspace"
        >
          {{ t('media.actions.retry') }}
        </button>
      </div>
      <p
        v-else-if="mediaLoading"
        class="state-text"
      >
        {{ t('media.states.loadingAssets') }}
      </p>
      <template v-else>
        <div class="media-toolbar">
          <h2>{{ t('media.library.title') }}</h2>
          <div class="filter-controls">
            <label class="control-group">{{ t('media.library.filterKind') }}
              <select v-model="filterKind">
                <option value="all">{{ t('media.library.all') }}</option>
                <option value="image">{{ t('media.kinds.image') }}</option>
                <option value="video">{{ t('media.kinds.video') }}</option>
                <option value="audio">{{ t('media.kinds.audio') }}</option>
              </select>
            </label>
            <label class="control-group">{{ t('media.library.filterOrigin') }}
              <select v-model="filterOrigin">
                <option value="all">{{ t('media.library.all') }}</option>
                <option value="upload">{{ t('media.origins.upload') }}</option>
                <option value="creator_media">{{ t('media.origins.creator_media') }}</option>
                <option value="generated_image">{{ t('media.origins.generated_image') }}</option>
                <option value="generated_video">{{ t('media.origins.generated_video') }}</option>
              </select>
            </label>
          </div>
        </div>
        <p
          v-if="hasMediaEmptyState"
          data-testid="media-empty-state"
          class="notice info"
        >
          {{ t('media.library.empty') }}
        </p>
        <p
          v-else-if="filteredAssets.length === 0"
          class="notice info"
        >
          {{ t('media.library.noFilterResults') }}
        </p>
        <div
          v-else
          class="asset-grid"
        >
          <article
            v-for="asset in filteredAssets"
            :key="asset.id"
            class="asset-card"
            :data-testid="`media-asset-${asset.id}`"
          >
            <div class="asset-preview">
              <template v-if="!previewFailures.includes(asset.id) && asset.kind === 'image'">
                <img
                  :src="mediaAssetContentURL(project.id, asset.id)"
                  :alt="t('media.asset.previewAlt', { name: safeFilename(asset) })"
                  @error="markPreviewFailed(asset.id)"
                >
              </template>
              <template v-else-if="!previewFailures.includes(asset.id) && asset.kind === 'video'">
                <video
                  controls
                  preload="metadata"
                  :src="mediaAssetContentURL(project.id, asset.id)"
                  :aria-label="t('media.asset.previewAlt', { name: safeFilename(asset) })"
                  @error="markPreviewFailed(asset.id)"
                />
              </template>
              <span
                v-if="previewFailures.includes(asset.id)"
                class="preview-failure"
                data-testid="preview-failure"
              >{{ t('media.asset.previewFailed') }}</span>
              <span
                v-else-if="asset.kind !== 'image' && asset.kind !== 'video'"
                class="preview-placeholder"
              >{{ t(`media.kinds.${asset.kind}`) }}</span>
            </div>
            <div class="asset-card-body">
              <strong>{{ safeFilename(asset) }}</strong>
              <span>{{ t(`media.kinds.${asset.kind}`) }} · {{ t(`media.origins.${asset.origin}`) }}</span>
              <span>{{ formatBytes(asset.byte_size) }} · {{ d(new Date(asset.created_at), 'long') }}</span>
              <span v-if="technicalDetails(asset)">{{ technicalDetails(asset) }}</span>
              <button
                class="secondary-button"
                type="button"
                :data-testid="`delete-asset-${asset.id}`"
                @click="requestDelete(asset)"
              >
                {{ t('media.actions.delete') }}
              </button>
            </div>
          </article>
        </div>
      </template>

      <div
        v-if="planListErrorCode"
        class="notice error"
        data-testid="scene-plan-list-error"
      >
        <p>{{ errorText('errors', planListErrorCode) }}</p>
        <button
          class="secondary-button"
          type="button"
          @click="loadWorkspace"
        >
          {{ t('media.actions.retry') }}
        </button>
      </div>
      <section
        v-else-if="approvedPlans.length"
        class="scene-assignment-section"
      >
        <div class="media-toolbar">
          <div>
            <h2>{{ t('media.assignment.title') }}</h2>
            <p class="field-help">
              {{ t('media.assignment.help') }}
            </p>
          </div>
          <label class="control-group">{{ t('media.assignment.planVersion') }}
            <select
              :value="selectedPlan?.version ?? targetPlanVersion ?? ''"
              @change="loadApprovedPlan(Number(($event.target as HTMLSelectElement).value))"
            >
              <option
                v-for="plan in approvedPlans"
                :key="plan.version"
                :value="plan.version"
                :data-testid="`approved-plan-${plan.version}`"
              >{{ t('media.assignment.version', { value: plan.version }) }}</option>
            </select>
          </label>
        </div>
        <p
          v-if="planLoading"
          class="state-text"
        >
          {{ t('media.states.loadingPlan') }}
        </p>
        <div
          v-else-if="planLoadErrorCode"
          class="notice error"
          data-testid="scene-plan-error"
        >
          <p>{{ errorText('errors', planLoadErrorCode) }}</p>
          <button
            class="secondary-button"
            type="button"
            @click="targetPlanVersion !== null && loadApprovedPlan(targetPlanVersion)"
          >
            {{ t('media.actions.retry') }}
          </button>
        </div>
        <div
          v-else
          class="scene-list"
        >
          <article
            v-for="scene in scenes"
            :key="scene.key"
            class="scene-row"
            :data-testid="`scene-row-${scene.key}`"
          >
            <div class="scene-copy">
              <span class="scene-sequence">{{ scene.key }}</span>
              <strong>{{ scene.narration }}</strong>
              <span>{{ scene.visual_instruction }}</span>
              <small>{{ t('media.assignment.plannedSource') }}: {{ t(`media.origins.${scene.planned_source_type}`) }}</small>
            </div>
            <div class="scene-current">
              <template v-if="assetFor(scene.key)">
                <div
                  class="scene-current-preview"
                  :data-testid="`scene-current-${assetFor(scene.key)?.id}`"
                >
                  <img
                    v-if="assetFor(scene.key)?.kind === 'image'"
                    :src="mediaAssetContentURL(project.id, assetFor(scene.key)!.id)"
                    :alt="safeFilename(assetFor(scene.key)!)"
                  >
                  <video
                    v-else-if="assetFor(scene.key)?.kind === 'video'"
                    controls
                    preload="metadata"
                    :src="mediaAssetContentURL(project.id, assetFor(scene.key)!.id)"
                    :aria-label="safeFilename(assetFor(scene.key)!)"
                  />
                  <span>{{ safeFilename(assetFor(scene.key)!) }}</span>
                </div>
              </template>
              <span
                v-else
                class="unbound-state"
              >{{ t('media.assignment.unbound') }}</span>
              <div class="scene-asset-options">
                <button
                  v-for="asset in visualAssets"
                  :key="asset.id"
                  class="secondary-button"
                  type="button"
                  :disabled="isPending(scene.key)"
                  :data-testid="`asset-option-${asset.id}-${scene.key}`"
                  @click="assign(scene, asset)"
                >
                  {{ t('media.actions.assign') }} {{ safeFilename(asset) }}
                </button>
              </div>
              <span
                v-if="isPending(scene.key)"
                class="field-help"
                role="status"
              >{{ t('media.assignment.saving') }}</span>
              <span
                v-if="assignmentErrors[scene.key]"
                class="error-text"
                :data-testid="`assignment-error-${scene.key}`"
              >{{ errorText('errors', assignmentErrors[scene.key]!) }}</span>
              <button
                class="text-button"
                type="button"
                :data-testid="`history-toggle-${scene.key}`"
                @click="toggleHistory(scene)"
              >
                {{ t('media.assignment.history') }}
              </button>
              <div
                v-if="historyOpen[scene.key]"
                class="history-list"
                :data-testid="`scene-history-${scene.key}`"
              >
                <p
                  v-if="!histories[scene.key]?.length"
                  class="field-help"
                >
                  {{ t('media.assignment.noHistory') }}
                </p>
                <div
                  v-for="entry in histories[scene.key]"
                  :key="entry.binding?.id ?? `${entry.scene_key}-${entry.binding?.binding_version}`"
                  class="history-entry"
                  :data-testid="`history-entry-${entry.binding?.binding_version ?? '0'}`"
                >
                  <div class="history-preview">
                    <img
                      v-if="entry.asset?.kind === 'image'"
                      :src="mediaAssetContentURL(project.id, entry.asset.id)"
                      :alt="safeFilename(entry.asset)"
                    >
                    <video
                      v-else-if="entry.asset?.kind === 'video'"
                      controls
                      preload="metadata"
                      :src="mediaAssetContentURL(project.id, entry.asset.id)"
                      :aria-label="safeFilename(entry.asset)"
                    />
                    <span v-else>{{ entry.asset ? t(`media.kinds.${entry.asset.kind}`) : t('media.assignment.missingAsset') }}</span>
                  </div>
                  <div class="history-entry-details">
                    <strong>{{ t('media.assignment.bindingVersion', { value: entry.binding?.binding_version ?? '?' }) }}</strong>
                    <span>{{ t(`media.bindingStatus.${entry.binding?.status ?? 'superseded'}`, entry.binding?.status ?? '') }}<template v-if="entry.binding?.created_at"> · {{ d(new Date(entry.binding.created_at), 'long') }}</template></span>
                    <span v-if="entry.asset">{{ safeFilename(entry.asset) }} · {{ t(`media.kinds.${entry.asset.kind}`) }} · {{ t(`media.origins.${entry.asset.origin}`) }} · {{ formatBytes(entry.asset.byte_size) }}</span>
                    <span v-if="entry.asset && technicalDetails(entry.asset)">{{ technicalDetails(entry.asset) }}</span>
                  </div>
                  <button
                    v-if="entry.asset"
                    class="text-button"
                    type="button"
                    :data-testid="`restore-history-${entry.binding?.binding_version ?? '0'}`"
                    @click="restore(scene, entry)"
                  >
                    {{ t('media.actions.restore') }}
                  </button>
                </div>
              </div>
            </div>
          </article>
        </div>
      </section>
      <div
        v-else
        class="notice info"
        data-testid="scene-assignment-disabled"
      >
        {{ t('media.assignment.noApprovedPlan') }}
      </div>
    </template>

    <div
      v-if="deleteTarget"
      class="confirmation-panel"
      data-testid="delete-confirmation"
      role="dialog"
      aria-modal="true"
    >
      <h2>{{ t('media.delete.title') }}</h2>
      <p>{{ t('media.delete.confirm', { name: safeFilename(deleteTarget), kind: t(`media.kinds.${deleteTarget.kind}`) }) }}</p>
      <p
        v-if="deleteErrorCode"
        class="error-text"
        data-testid="delete-in-use-error"
      >
        {{ errorText('errors', deleteErrorCode) }}
      </p>
      <div class="upload-actions">
        <button
          class="primary-button"
          type="button"
          @click="confirmDelete"
        >
          {{ t('media.actions.confirmDelete') }}
        </button>
        <button
          class="secondary-button"
          type="button"
          @click="cancelDelete"
        >
          {{ t('media.actions.cancel') }}
        </button>
      </div>
    </div>
  </section>
</template>

<style scoped>
.media-page { max-width: 1180px; }
.media-upload-panel, .confirmation-panel { display: grid; gap: 14px; margin: 24px 0; border: 1px solid #d7ddd5; border-radius: 8px; padding: 20px; background: #fff; }
.media-upload-panel h2, .media-toolbar h2, .scene-assignment-section h2, .confirmation-panel h2 { margin: 0; font-size: 22px; }
.media-upload-panel p { margin: 4px 0 0; }
.upload-actions, .filter-controls, .scene-asset-options { display: flex; flex-wrap: wrap; align-items: center; gap: 10px; }
.upload-progress { display: grid; gap: 6px; max-width: 420px; }
.media-toolbar { display: flex; align-items: end; justify-content: space-between; gap: 18px; margin: 30px 0 16px; }
.control-group { min-width: 150px; }
.asset-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(210px, 1fr)); gap: 14px; }
.asset-card { overflow: hidden; border: 1px solid #d7ddd5; border-radius: 8px; background: #fff; }
.asset-preview { display: grid; place-items: center; min-height: 150px; background: #edf1ed; color: #647275; }
.asset-preview img, .asset-preview video { display: block; width: 100%; height: 150px; object-fit: cover; }
.asset-card-body { display: grid; gap: 6px; padding: 12px; font-size: 13px; }
.asset-card-body span { color: #647275; }
.asset-card-body .secondary-button { justify-self: start; margin-top: 6px; }
.scene-assignment-section { margin-top: 40px; }
.scene-list { display: grid; gap: 14px; }
.scene-row { display: grid; grid-template-columns: minmax(190px, .8fr) minmax(0, 1.2fr); gap: 18px; border: 1px solid #d7ddd5; border-radius: 8px; padding: 16px; background: #fff; }
.scene-copy, .scene-current { display: grid; align-content: start; gap: 8px; min-width: 0; }
.scene-copy span, .scene-copy small, .scene-current span { color: #647275; }
.scene-sequence { color: #27634e !important; font-weight: 700; }
.scene-current-preview { display: flex; align-items: center; gap: 10px; min-height: 56px; }
.scene-current-preview img, .scene-current-preview video { width: 76px; height: 52px; object-fit: cover; border-radius: 4px; }
.unbound-state { border: 1px dashed #cbd6d0; padding: 12px; }
.history-list { display: grid; gap: 6px; margin-top: 4px; border-top: 1px solid #e3e8e3; padding-top: 8px; }
.history-entry { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; font-size: 13px; }
.history-entry img { width: 48px; height: 34px; object-fit: cover; border-radius: 3px; }
.history-entry span:first-child { font-weight: 700; }
.text-button { width: max-content; border: 0; padding: 0; background: transparent; color: #27634e; font: inherit; font-size: 13px; font-weight: 700; cursor: pointer; }
.error-text { color: #8a2424 !important; font-size: 13px; font-weight: 700; }
.preview-failure { padding: 18px; color: #8a2424 !important; font-weight: 700; }
.visually-hidden { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; clip-path: inset(50%); }
@media (max-width: 720px) { .media-toolbar, .scene-row { grid-template-columns: 1fr; display: grid; align-items: stretch; } }
</style>
