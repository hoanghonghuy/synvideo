<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, RouterLink } from 'vue-router'

import { ApiError } from '@/api/projects'
import { listMediaAssets, mediaAssetContentURL, uploadMediaAsset, type MediaAsset } from '@/features/media/api'
import {
  createAudioMix,
  getAudioMix,
  getAudioMixSnapshot,
  listAudioMixHistory,
  rebindAudioMixNarration,
  updateAudioMix,
  type AudioMixConfig,
  type AudioMixDocument,
  type AudioMixView,
} from './api'
import messages from './messages'

const { t } = useI18n({ useScope: 'local', messages })
const route = useRoute()
const projectID = computed(() => String(route.params.id ?? ''))
const loading = ref(true)
const saving = ref(false)
const uploading = ref(false)
const error = ref('')
const notice = ref('')
const mix = ref<AudioMixView | null>(null)
const assets = ref<MediaAsset[]>([])
const history = ref<AudioMixDocument[]>([])
const selectedMusicID = ref('')

const config = reactive<AudioMixConfig>({
  music_trim_start_ms: 0,
  start_offset_ms: 0,
  loop_policy: 'LOOP_TO_TARGET',
  music_gain_db: -12,
  narration_gain_db: 0,
  ducking: { enabled: true, reduction_db: 9, attack_ms: 120, release_ms: 350 },
})

const audioAssets = computed(() => assets.value.filter((asset) => asset.kind === 'audio'))
const selectedAsset = computed(() => audioAssets.value.find((asset) => asset.id === selectedMusicID.value))
const previewURL = computed(() => selectedAsset.value ? mediaAssetContentURL(projectID.value, selectedAsset.value.id) : '')
const stateClass = computed(() => `state-${(mix.value?.state ?? 'empty').toLowerCase()}`)

watch(
  () => config.ducking.enabled,
  (enabled) => {
    if (!enabled) {
      config.ducking.reduction_db = 0
      config.ducking.attack_ms = 0
      config.ducking.release_ms = 0
    }
  },
)

function applyMix(value: AudioMixView) {
  mix.value = value
  selectedMusicID.value = value.music_asset_id
  Object.assign(config, structuredClone(value.config))
}

async function load() {
  if (!projectID.value) return
  loading.value = true
  error.value = ''
  try {
    const media = await listMediaAssets(projectID.value)
    assets.value = media.assets
    try {
      applyMix(await getAudioMix(projectID.value))
      history.value = await listAudioMixHistory(projectID.value)
    } catch (cause) {
      if (cause instanceof ApiError && cause.status === 404) {
        mix.value = null
        history.value = []
      } else {
        throw cause
      }
    }
  } catch (cause) {
    error.value = messageFor(cause)
  } finally {
    loading.value = false
  }
}

async function save() {
  if (!selectedMusicID.value || saving.value) return
  saving.value = true
  error.value = ''
  notice.value = ''
  try {
    const value = mix.value
      ? await updateAudioMix(projectID.value, {
          expected_revision: mix.value.revision,
          music_asset_id: selectedMusicID.value,
          config: structuredClone(config),
        })
      : await createAudioMix(projectID.value, {
          music_asset_id: selectedMusicID.value,
          config: structuredClone(config),
        })
    applyMix(value)
    history.value = await listAudioMixHistory(projectID.value)
    notice.value = t('audioMix.saved', { revision: value.revision })
  } catch (cause) {
    error.value = messageFor(cause)
  } finally {
    saving.value = false
  }
}

async function rebindNarration() {
  if (!mix.value || saving.value) return
  saving.value = true
  error.value = ''
  notice.value = ''
  try {
    const value = await rebindAudioMixNarration(projectID.value, mix.value.revision)
    applyMix(value)
    history.value = await listAudioMixHistory(projectID.value)
    notice.value = t('audioMix.rebound')
  } catch (cause) {
    error.value = messageFor(cause)
  } finally {
    saving.value = false
  }
}

async function verifySnapshot() {
  if (!mix.value || mix.value.state !== 'CURRENT') return
  error.value = ''
  notice.value = ''
  try {
    const snapshot = await getAudioMixSnapshot(projectID.value)
    notice.value = t('audioMix.snapshotReady', { revision: snapshot.revision })
  } catch (cause) {
    error.value = messageFor(cause)
  }
}

async function onUpload(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file || uploading.value) return
  uploading.value = true
  error.value = ''
  notice.value = ''
  try {
    const asset = await uploadMediaAsset(projectID.value, file)
    const media = await listMediaAssets(projectID.value)
    assets.value = media.assets
    selectedMusicID.value = asset.id
    notice.value = t('audioMix.uploaded')
  } catch (cause) {
    error.value = messageFor(cause)
  } finally {
    uploading.value = false
    input.value = ''
  }
}

function messageFor(cause: unknown): string {
  if (cause instanceof ApiError) return `${cause.code}: ${cause.message}`
  return cause instanceof Error ? cause.message : t('audioMix.fallbackError')
}

onMounted(load)
</script>

<template>
  <main class="mix-page">
    <header class="mix-header">
      <div>
        <p class="eyebrow">{{ t('audioMix.eyebrow') }}</p>
        <h1>{{ t('audioMix.title') }}</h1>
        <p>{{ t('audioMix.description') }}</p>
      </div>
      <RouterLink :to="`/projects/${projectID}`">{{ t('audioMix.back') }}</RouterLink>
    </header>

    <p v-if="error" class="alert error" role="alert">{{ error }}</p>
    <p v-if="notice" class="alert notice" role="status">{{ notice }}</p>
    <p v-if="loading" role="status">{{ t('audioMix.loading') }}</p>

    <template v-else>
      <section v-if="mix" class="state-card" :class="stateClass" aria-live="polite">
        <strong>{{ mix.state }}</strong>
        <span>{{ t('audioMix.stateSummary', { revision: mix.revision, plan: mix.scene_plan_version, seconds: Math.round(mix.narration_duration_ms / 1000) }) }}</span>
        <p v-if="mix.state === 'STALE'">{{ t('audioMix.stale') }}</p>
        <p v-else-if="mix.state === 'BROKEN'">{{ t('audioMix.broken') }}</p>
      </section>

      <form class="mix-grid" @submit.prevent="save">
        <section class="panel">
          <h2>{{ t('audioMix.sourceTitle') }}</h2>
          <label for="music-asset">{{ t('audioMix.assetLabel') }}</label>
          <select id="music-asset" v-model="selectedMusicID" required>
            <option value="" disabled>{{ t('audioMix.chooseAsset') }}</option>
            <option v-for="asset in audioAssets" :key="asset.id" :value="asset.id">
              {{ asset.original_filename || asset.id }}
            </option>
          </select>
          <audio v-if="previewURL" :src="previewURL" controls preload="metadata">{{ t('audioMix.unsupportedAudio') }}</audio>
          <label class="upload-button">
            <span>{{ uploading ? t('audioMix.uploading') : t('audioMix.upload') }}</span>
            <input type="file" accept="audio/*" :disabled="uploading" @change="onUpload" />
          </label>
          <p v-if="audioAssets.length === 0">
            {{ t('audioMix.noAudioPrefix') }}
            <RouterLink :to="`/projects/${projectID}/media`">{{ t('audioMix.mediaLibrary') }}</RouterLink>.
          </p>
        </section>

        <section class="panel controls">
          <h2>{{ t('audioMix.timingTitle') }}</h2>
          <label>{{ t('audioMix.trimStart') }}<input v-model.number="config.music_trim_start_ms" type="number" min="0" step="1" /></label>
          <label>{{ t('audioMix.startOffset') }}<input v-model.number="config.start_offset_ms" type="number" min="0" step="1" /></label>
          <label>{{ t('audioMix.loopPolicy') }}
            <select v-model="config.loop_policy">
              <option value="NO_LOOP">{{ t('audioMix.noLoop') }}</option>
              <option value="LOOP_TO_TARGET">{{ t('audioMix.loopToTarget') }}</option>
            </select>
          </label>
          <label>{{ t('audioMix.musicGain') }}<input v-model.number="config.music_gain_db" type="number" min="-60" max="12" step="0.5" /></label>
          <label>{{ t('audioMix.narrationGain') }}<input v-model.number="config.narration_gain_db" type="number" min="-24" max="12" step="0.5" /></label>
        </section>

        <section class="panel controls">
          <h2>{{ t('audioMix.duckingTitle') }}</h2>
          <label class="check"><input v-model="config.ducking.enabled" type="checkbox" /> {{ t('audioMix.duckingEnabled') }}</label>
          <template v-if="config.ducking.enabled">
            <label>{{ t('audioMix.reduction') }}<input v-model.number="config.ducking.reduction_db" type="number" min="0" max="60" step="0.5" /></label>
            <label>{{ t('audioMix.attack') }}<input v-model.number="config.ducking.attack_ms" type="number" min="0" max="10000" step="10" /></label>
            <label>{{ t('audioMix.release') }}<input v-model.number="config.ducking.release_ms" type="number" min="0" max="10000" step="10" /></label>
          </template>
          <p v-else class="hint">{{ t('audioMix.duckingDisabled') }}</p>
        </section>

        <section class="panel actions">
          <h2>{{ t('audioMix.revisionTitle') }}</h2>
          <button type="submit" :disabled="saving || !selectedMusicID">{{ saving ? t('audioMix.saving') : mix ? t('audioMix.save') : t('audioMix.create') }}</button>
          <button v-if="mix?.state === 'STALE'" type="button" :disabled="saving" @click="rebindNarration">{{ t('audioMix.rebind') }}</button>
          <button type="button" :disabled="!mix || mix.state !== 'CURRENT'" @click="verifySnapshot">{{ t('audioMix.verifySnapshot') }}</button>
          <p v-if="mix && mix.state !== 'CURRENT'" class="hint">{{ t('audioMix.snapshotBlocked') }}</p>
          <details v-if="history.length">
            <summary>{{ t('audioMix.history', { count: history.length }) }}</summary>
            <ol>
              <li v-for="item in history" :key="`${item.id}-${item.revision}`">{{ t('audioMix.revision', { revision: item.revision }) }} · {{ new Date(item.updated_at).toLocaleString() }}</li>
            </ol>
          </details>
        </section>
      </form>
    </template>
  </main>
</template>

<style scoped>
.mix-page { max-width: 1180px; margin: 0 auto; padding: 2rem; }
.mix-header { display: flex; justify-content: space-between; gap: 2rem; align-items: flex-start; margin-bottom: 1.5rem; }
.eyebrow { font-size: .78rem; text-transform: uppercase; letter-spacing: .08em; opacity: .7; }
.mix-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 1rem; }
.panel, .state-card { border: 1px solid var(--color-border, #d6d6d6); border-radius: 14px; padding: 1rem; background: var(--color-surface, #fff); }
.controls label { display: grid; gap: .35rem; margin-bottom: .8rem; }
.check { display: flex !important; grid-template-columns: auto 1fr; align-items: center; }
select, input[type='number'] { width: 100%; box-sizing: border-box; padding: .65rem; border: 1px solid #aaa; border-radius: 8px; }
audio { width: 100%; margin: .8rem 0; }
button, .upload-button { display: inline-flex; justify-content: center; align-items: center; padding: .65rem .9rem; border-radius: 8px; border: 1px solid #777; cursor: pointer; margin: .25rem .35rem .25rem 0; }
button:disabled { cursor: not-allowed; opacity: .5; }
.upload-button input { position: absolute; inline-size: 1px; block-size: 1px; overflow: hidden; clip: rect(0 0 0 0); }
.alert { padding: .75rem 1rem; border-radius: 8px; }
.error { background: #fee; }
.notice { background: #eef9ee; }
.state-card { margin-bottom: 1rem; display: grid; gap: .25rem; }
.state-current { border-left: 5px solid #2f8f46; }
.state-stale { border-left: 5px solid #b77b00; }
.state-broken, .state-error { border-left: 5px solid #b02a37; }
.hint { font-size: .9rem; opacity: .75; }
@media (max-width: 760px) { .mix-grid { grid-template-columns: 1fr; } .mix-header { flex-direction: column; } .mix-page { padding: 1rem; } }
</style>
