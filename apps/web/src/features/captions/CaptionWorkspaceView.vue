<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'

import {
  CaptionApiError,
  deriveCaptions,
  getCaptions,
  listCaptionHistory,
  rebuildCaptions,
  updateCaptions,
  type CaptionDocument,
  type CaptionSegment,
  type CaptionStyle,
  type CaptionView,
} from './api'

const route = useRoute()
const projectID = computed(() => String(route.params.id ?? ''))
const planVersion = ref(Number(route.query.version ?? 1))
const sceneKey = ref(String(route.query.scene ?? ''))
const current = ref<CaptionView | null>(null)
const history = ref<CaptionDocument[]>([])
const segments = ref<CaptionSegment[]>([])
const style = ref<CaptionStyle>({ alignment: 'center', position: 'bottom', size: 'medium', weight: 'normal' })
const busy = ref(false)
const message = ref('')
const errorMessage = ref('')

const canOperate = computed(() => projectID.value !== '' && planVersion.value > 0 && sceneKey.value.trim() !== '')
const isStale = computed(() => current.value?.state === 'STALE')

function copyFromView(view: CaptionView) {
  current.value = view
  segments.value = view.segments.map((segment) => ({ ...segment }))
  style.value = { ...view.style }
}

function describeError(error: unknown): string {
  if (error instanceof CaptionApiError) {
    if (error.code === 'CAPTION_REVISION_CONFLICT') return 'Bản caption đã thay đổi ở nơi khác. Hãy tải lại trước khi lưu.'
    if (error.code === 'CAPTION_SOURCE_MISSING') return 'Cảnh này chưa có narration audio hợp lệ với thời lượng đo được.'
    if (error.code === 'CAPTION_STALE') return 'Caption đang stale. Hãy rebuild có chủ đích trước khi dùng làm snapshot hiện hành.'
    if (error.code === 'CAPTION_NOT_FOUND') return 'Chưa có caption cho cảnh này.'
    if (error.fields) return Object.entries(error.fields).map(([field, value]) => `${field}: ${value}`).join(', ')
    return error.message
  }
  return error instanceof Error ? error.message : 'Không thể hoàn thành yêu cầu.'
}

async function refreshHistory() {
  if (!canOperate.value) return
  history.value = await listCaptionHistory(projectID.value, planVersion.value, sceneKey.value.trim())
}

async function load() {
  if (!canOperate.value) return
  busy.value = true
  message.value = ''
  errorMessage.value = ''
  try {
    const view = await getCaptions(projectID.value, planVersion.value, sceneKey.value.trim())
    copyFromView(view)
    await refreshHistory()
  } catch (error) {
    current.value = null
    history.value = []
    errorMessage.value = describeError(error)
  } finally {
    busy.value = false
  }
}

async function derive() {
  if (!canOperate.value) return
  busy.value = true
  message.value = ''
  errorMessage.value = ''
  try {
    const view = await deriveCaptions(projectID.value, planVersion.value, sceneKey.value.trim())
    copyFromView(view)
    await refreshHistory()
    message.value = 'Đã tạo caption từ exact narration lineage hiện tại.'
  } catch (error) {
    errorMessage.value = describeError(error)
  } finally {
    busy.value = false
  }
}

async function save() {
  if (!current.value || !canOperate.value) return
  busy.value = true
  message.value = ''
  errorMessage.value = ''
  try {
    const view = await updateCaptions(projectID.value, planVersion.value, sceneKey.value.trim(), {
      expected_revision: current.value.revision,
      segments: segments.value,
      style: style.value,
    })
    copyFromView(view)
    await refreshHistory()
    message.value = view.state === 'STALE'
      ? 'Đã lưu chỉnh sửa. Caption vẫn stale vì source narration đã thay đổi.'
      : 'Đã lưu caption.'
  } catch (error) {
    errorMessage.value = describeError(error)
  } finally {
    busy.value = false
  }
}

async function rebuild() {
  if (!current.value || !canOperate.value) return
  busy.value = true
  message.value = ''
  errorMessage.value = ''
  try {
    const view = await rebuildCaptions(projectID.value, planVersion.value, sceneKey.value.trim(), current.value.revision)
    copyFromView(view)
    await refreshHistory()
    message.value = 'Đã rebuild sang narration lineage hiện tại. Revision cũ vẫn được giữ trong history.'
  } catch (error) {
    errorMessage.value = describeError(error)
  } finally {
    busy.value = false
  }
}

function addSegment() {
  const previous = segments.value.length > 0 ? segments.value[segments.value.length - 1] : undefined
  const start = previous?.end_ms ?? 0
  const end = Math.min(current.value?.source_duration_ms ?? start + 1000, start + 1000)
  segments.value.push({ id: crypto.randomUUID(), text: '', start_ms: start, end_ms: Math.max(start + 1, end) })
}

function removeSegment(index: number) {
  segments.value.splice(index, 1)
}

onMounted(() => {
  if (sceneKey.value) void load()
})
</script>

<template>
  <section class="caption-workspace">
    <header class="workspace-header">
      <div>
        <p class="eyebrow">TASK-034 · Captions</p>
        <h1>Caption timing workspace</h1>
        <p>Caption được khóa với exact narration/audio lineage và thời lượng audio đã đo.</p>
      </div>
      <RouterLink :to="`/projects/${projectID}`">Quay lại project</RouterLink>
    </header>

    <div class="toolbar card">
      <label>
        Scene plan version
        <input v-model.number="planVersion" type="number" min="1" />
      </label>
      <label>
        Scene key
        <input v-model.trim="sceneKey" placeholder="scene-1" />
      </label>
      <button :disabled="busy || !canOperate" @click="load">Tải</button>
      <button :disabled="busy || !canOperate" @click="derive">Tạo lần đầu</button>
    </div>

    <p v-if="errorMessage" class="notice error" role="alert">{{ errorMessage }}</p>
    <p v-if="message" class="notice success" role="status">{{ message }}</p>

    <template v-if="current">
      <div class="state-card" :class="{ stale: isStale }">
        <div>
          <strong>{{ current.state }}</strong>
          · revision {{ current.revision }}
          · source {{ current.source_duration_ms }} ms
        </div>
        <button v-if="isStale" :disabled="busy" @click="rebuild">Rebuild từ narration hiện tại</button>
      </div>

      <section class="card">
        <div class="section-heading">
          <div>
            <h2>Segments</h2>
            <p>Không overlap; 0 ≤ start &lt; end ≤ source duration.</p>
          </div>
          <button type="button" @click="addSegment">Thêm segment</button>
        </div>

        <div class="segments">
          <article v-for="(segment, index) in segments" :key="segment.id" class="segment-row">
            <label class="segment-text">
              Text
              <textarea v-model="segment.text" rows="2" />
            </label>
            <label>
              Start (ms)
              <input v-model.number="segment.start_ms" type="number" min="0" />
            </label>
            <label>
              End (ms)
              <input v-model.number="segment.end_ms" type="number" min="1" :max="current.source_duration_ms" />
            </label>
            <button type="button" class="danger" @click="removeSegment(index)">Xóa</button>
          </article>
        </div>
      </section>

      <section class="card">
        <h2>Render-neutral style</h2>
        <div class="style-grid">
          <label>Alignment
            <select v-model="style.alignment"><option>left</option><option>center</option><option>right</option></select>
          </label>
          <label>Position
            <select v-model="style.position"><option>top</option><option>middle</option><option>bottom</option></select>
          </label>
          <label>Size
            <select v-model="style.size"><option>small</option><option>medium</option><option>large</option></select>
          </label>
          <label>Weight
            <select v-model="style.weight"><option>normal</option><option>semibold</option><option>bold</option></select>
          </label>
          <label>Font token
            <input v-model="style.font_family_token" placeholder="optional token" />
          </label>
        </div>
        <button class="primary" :disabled="busy" @click="save">Lưu revision mới</button>
      </section>

      <section class="card">
        <h2>Revision history</h2>
        <p v-if="history.length === 0">Chưa có history.</p>
        <ol v-else class="history-list">
          <li v-for="item in history" :key="`${item.id}:${item.revision}`">
            Revision {{ item.revision }} · {{ item.source_duration_ms }} ms · source {{ item.source_asset_id.slice(0, 8) }}…
          </li>
        </ol>
      </section>
    </template>
  </section>
</template>

<style scoped>
.caption-workspace { display: grid; gap: 1rem; max-width: 1180px; margin: 0 auto; padding: 1.5rem; }
.workspace-header, .section-heading, .state-card { display: flex; align-items: center; justify-content: space-between; gap: 1rem; }
.eyebrow { text-transform: uppercase; letter-spacing: .08em; font-size: .75rem; opacity: .65; }
.card, .state-card { border: 1px solid var(--color-border, #d8dee9); border-radius: 12px; padding: 1rem; background: var(--color-surface, #fff); }
.toolbar, .style-grid { display: flex; flex-wrap: wrap; align-items: end; gap: .75rem; }
label { display: grid; gap: .35rem; font-size: .9rem; }
input, textarea, select { border: 1px solid #c5ccd8; border-radius: 8px; padding: .55rem .65rem; font: inherit; }
button { border: 1px solid #9aa5b5; border-radius: 8px; padding: .55rem .8rem; cursor: pointer; background: #fff; }
button:disabled { cursor: not-allowed; opacity: .55; }
.primary { margin-top: 1rem; font-weight: 600; }
.state-card.stale { border-color: #d97706; background: #fffbeb; }
.notice { padding: .75rem 1rem; border-radius: 8px; }
.notice.error { background: #fef2f2; color: #991b1b; }
.notice.success { background: #ecfdf5; color: #065f46; }
.segments { display: grid; gap: .75rem; }
.segment-row { display: grid; grid-template-columns: minmax(260px, 1fr) 140px 140px auto; gap: .75rem; align-items: end; padding: .75rem 0; border-bottom: 1px solid #e5e7eb; }
.segment-text textarea { width: 100%; resize: vertical; }
.danger { color: #991b1b; }
.history-list { display: grid; gap: .4rem; }
@media (max-width: 760px) { .segment-row { grid-template-columns: 1fr 1fr; } .segment-text { grid-column: 1 / -1; } }
</style>
