<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
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
    notice.value = `Đã lưu audio mix revision ${value.revision}.`
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
    notice.value = 'Đã rebind tới narration hiện tại bằng một revision mới.'
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
    notice.value = `Composition snapshot sẵn sàng: revision ${snapshot.revision}.`
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
    notice.value = 'Đã upload nhạc vào Media Library và chọn làm nguồn mix.'
  } catch (cause) {
    error.value = messageFor(cause)
  } finally {
    uploading.value = false
    input.value = ''
  }
}

function messageFor(cause: unknown): string {
  if (cause instanceof ApiError) return `${cause.code}: ${cause.message}`
  return cause instanceof Error ? cause.message : 'Không thể hoàn tất yêu cầu.'
}

onMounted(load)
</script>

<template>
  <main class="mix-page">
    <header class="mix-header">
      <div>
        <p class="eyebrow">F1 Creative Workflow</p>
        <h1>Background Music & Audio Mix</h1>
        <p>Chọn nhạc bền vững từ Media Library, cân level và ducking theo narration, rồi khóa một snapshot cho editor/render.</p>
      </div>
      <RouterLink :to="`/projects/${projectID}`">Về project</RouterLink>
    </header>

    <p v-if="error" class="alert error" role="alert">{{ error }}</p>
    <p v-if="notice" class="alert notice" role="status">{{ notice }}</p>
    <p v-if="loading" role="status">Đang tải audio mix…</p>

    <template v-else>
      <section v-if="mix" class="state-card" :class="stateClass" aria-live="polite">
        <strong>{{ mix.state }}</strong>
        <span>Revision {{ mix.revision }} · Scene plan v{{ mix.scene_plan_version }} · Narration {{ Math.round(mix.narration_duration_ms / 1000) }}s</span>
        <p v-if="mix.state === 'STALE'">Narration đã thay đổi. Các chỉnh sửa mix vẫn được giữ nguyên nhưng snapshot bị khóa cho tới khi bạn rebind rõ ràng.</p>
        <p v-else-if="mix.state === 'BROKEN'">Music asset đã mất hoặc không còn hợp lệ. Hãy chọn một audio asset khác và lưu revision mới.</p>
      </section>

      <form class="mix-grid" @submit.prevent="save">
        <section class="panel">
          <h2>Nguồn nhạc</h2>
          <label for="music-asset">Audio asset</label>
          <select id="music-asset" v-model="selectedMusicID" required>
            <option value="" disabled>Chọn audio asset</option>
            <option v-for="asset in audioAssets" :key="asset.id" :value="asset.id">
              {{ asset.original_filename || asset.id }}
            </option>
          </select>
          <audio v-if="previewURL" :src="previewURL" controls preload="metadata">Trình duyệt không hỗ trợ audio preview.</audio>
          <label class="upload-button">
            <span>{{ uploading ? 'Đang upload…' : 'Upload audio mới' }}</span>
            <input type="file" accept="audio/*" :disabled="uploading" @change="onUpload" />
          </label>
          <p v-if="audioAssets.length === 0">Chưa có audio asset. Upload ngay tại đây hoặc mở <RouterLink :to="`/projects/${projectID}/media`">Media Library</RouterLink>.</p>
        </section>

        <section class="panel controls">
          <h2>Timing & level</h2>
          <label>Music trim start (ms)<input v-model.number="config.music_trim_start_ms" type="number" min="0" step="1" /></label>
          <label>Start offset (ms)<input v-model.number="config.start_offset_ms" type="number" min="0" step="1" /></label>
          <label>Loop policy
            <select v-model="config.loop_policy">
              <option value="NO_LOOP">No loop</option>
              <option value="LOOP_TO_TARGET">Loop to narration target</option>
            </select>
          </label>
          <label>Music gain (dB)<input v-model.number="config.music_gain_db" type="number" min="-60" max="12" step="0.5" /></label>
          <label>Narration gain (dB)<input v-model.number="config.narration_gain_db" type="number" min="-24" max="12" step="0.5" /></label>
        </section>

        <section class="panel controls">
          <h2>Narration ducking</h2>
          <label class="check"><input v-model="config.ducking.enabled" type="checkbox" /> Bật ducking</label>
          <template v-if="config.ducking.enabled">
            <label>Reduction (dB)<input v-model.number="config.ducking.reduction_db" type="number" min="0" max="60" step="0.5" /></label>
            <label>Attack (ms)<input v-model.number="config.ducking.attack_ms" type="number" min="0" max="10000" step="10" /></label>
            <label>Release (ms)<input v-model.number="config.ducking.release_ms" type="number" min="0" max="10000" step="10" /></label>
          </template>
          <p v-else class="hint">Khi ducking tắt, backend yêu cầu các tham số ducking bằng 0 để tránh cấu hình ẩn.</p>
        </section>

        <section class="panel actions">
          <h2>Revision & snapshot</h2>
          <button type="submit" :disabled="saving || !selectedMusicID">{{ saving ? 'Đang lưu…' : mix ? 'Lưu revision mới' : 'Tạo audio mix' }}</button>
          <button v-if="mix?.state === 'STALE'" type="button" :disabled="saving" @click="rebindNarration">Rebind narration hiện tại</button>
          <button type="button" :disabled="!mix || mix.state !== 'CURRENT'" @click="verifySnapshot">Kiểm tra composition snapshot</button>
          <p v-if="mix && mix.state !== 'CURRENT'" class="hint">Snapshot bị khóa khi mix STALE/BROKEN để editor/render không dùng nguồn không còn đúng.</p>
          <details v-if="history.length">
            <summary>Lịch sử {{ history.length }} revision</summary>
            <ol>
              <li v-for="item in history" :key="`${item.id}-${item.revision}`">Revision {{ item.revision }} · {{ new Date(item.updated_at).toLocaleString() }}</li>
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
