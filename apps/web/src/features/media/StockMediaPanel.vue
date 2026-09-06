<script setup lang="ts">
import { computed, ref } from 'vue'

import { ApiError } from '@/api/projects'
import {
  acquireStockMedia,
  searchStockMedia,
  type MediaAsset,
  type StockMediaKind,
  type StockMediaOrientation,
  type StockMediaResult,
} from './api'

const props = defineProps<{ projectId: string }>()
const emit = defineEmits<{ acquired: [asset: MediaAsset] }>()

const query = ref('')
const kind = ref<StockMediaKind>('image')
const orientation = ref<StockMediaOrientation>('')
const results = ref<StockMediaResult[]>([])
const searching = ref(false)
const searchError = ref('')
const acquiringID = ref('')
const acquireError = ref('')
const acquiredID = ref('')
const page = ref(1)
const hasNextPage = ref(false)

const canSearch = computed(() => query.value.trim().length > 0 && !searching.value)

async function search(targetPage = 1) {
  if (!canSearch.value && targetPage === 1) return
  searching.value = true
  searchError.value = ''
  acquireError.value = ''
  try {
    const response = await searchStockMedia(props.projectId, {
      provider: 'pexels',
      query: query.value.trim(),
      kind: kind.value,
      orientation: orientation.value,
      page: targetPage,
      perPage: 20,
    })
    results.value = response.results ?? []
    page.value = response.page
    hasNextPage.value = response.has_next_page
  } catch (error) {
    results.value = []
    searchError.value = errorCode(error)
  } finally {
    searching.value = false
  }
}

async function acquire(result: StockMediaResult) {
  if (!result.acquirable || acquiringID.value) return
  acquiringID.value = result.provider_result_id
  acquireError.value = ''
  acquiredID.value = ''
  try {
    const response = await acquireStockMedia(props.projectId, result)
    acquiredID.value = result.provider_result_id
    emit('acquired', response.asset)
  } catch (error) {
    acquireError.value = errorCode(error)
  } finally {
    acquiringID.value = ''
  }
}

function errorCode(error: unknown) {
  return error instanceof ApiError ? error.code : 'request_failed'
}

function errorMessage(code: string) {
  const messages: Record<string, string> = {
    STOCK_MEDIA_PROVIDER_UNAVAILABLE: 'Pexels chưa được cấu hình trên máy chủ.',
    STOCK_MEDIA_RATE_LIMITED: 'Pexels đang giới hạn yêu cầu. Vui lòng thử lại sau.',
    STOCK_MEDIA_SOURCE_UNAVAILABLE: 'Media nguồn không còn khả dụng.',
    STOCK_MEDIA_PROVIDER_AUTH_FAILED: 'Khóa Pexels trên máy chủ hiện không hợp lệ.',
    STOCK_MEDIA_PROVIDER_FAILED: 'Pexels chưa thể hoàn tất yêu cầu.',
    STOCK_MEDIA_TOO_LARGE: 'Media đã chọn vượt quá giới hạn lưu trữ của dự án.',
    STOCK_MEDIA_STORAGE_FAILED: 'Không thể lưu media stock vào storage của dự án.',
    STOCK_MEDIA_INVALID: 'Yêu cầu tìm kiếm media stock không hợp lệ.',
    request_failed: 'Không thể kết nối máy chủ.',
  }
  return messages[code] ?? 'Không thể hoàn tất yêu cầu media stock.'
}
</script>

<template>
  <section class="stock-panel" data-testid="stock-media-panel">
    <div class="stock-heading">
      <div>
        <h2>Kho media stock</h2>
        <p>Tìm ảnh/video từ Pexels, xem nguồn và giấy phép trước khi lưu vào dự án.</p>
      </div>
      <span class="stock-provider">Pexels</span>
    </div>

    <form class="stock-search" @submit.prevent="search(1)">
      <label>
        Từ khóa
        <input v-model="query" type="search" placeholder="Ví dụ: rainy Tokyo street" data-testid="stock-query">
      </label>
      <label>
        Loại
        <select v-model="kind" data-testid="stock-kind">
          <option value="image">Hình ảnh</option>
          <option value="video">Video</option>
        </select>
      </label>
      <label>
        Hướng
        <select v-model="orientation">
          <option value="">Bất kỳ</option>
          <option value="landscape">Ngang</option>
          <option value="portrait">Dọc</option>
          <option value="square">Vuông</option>
        </select>
      </label>
      <button class="primary-button" type="submit" :disabled="!canSearch">
        {{ searching ? 'Đang tìm...' : 'Tìm media' }}
      </button>
    </form>

    <div v-if="searchError" class="notice error" data-testid="stock-search-error">
      {{ errorMessage(searchError) }}
    </div>
    <p v-else-if="!searching && results.length === 0 && page > 1" class="notice info">Không còn kết quả.</p>
    <p v-else-if="!searching && results.length === 0 && query.trim()" class="field-help">Không tìm thấy kết quả phù hợp.</p>

    <div v-if="results.length" class="stock-grid">
      <article v-for="result in results" :key="`${result.provider_key}:${result.provider_result_id}`" class="stock-card">
        <div class="stock-preview">
          <img v-if="result.kind === 'image'" :src="result.preview_url" alt="Xem trước media stock">
          <video v-else :poster="result.preview_url" muted controls preload="none" aria-label="Xem trước video stock" />
        </div>
        <div class="stock-meta">
          <strong>{{ result.creator_name || 'Không rõ tác giả' }}</strong>
          <span>{{ result.license_summary }}</span>
          <span>{{ result.attribution_text }}</span>
          <div class="stock-links">
            <a :href="result.source_page_url" target="_blank" rel="noopener noreferrer">Nguồn</a>
            <a v-if="result.license_reference" :href="result.license_reference" target="_blank" rel="noopener noreferrer">Giấy phép</a>
          </div>
          <button
            class="secondary-button"
            type="button"
            :disabled="!result.acquirable || Boolean(acquiringID)"
            :data-testid="`stock-acquire-${result.provider_result_id}`"
            @click="acquire(result)"
          >
            {{ acquiringID === result.provider_result_id ? 'Đang lưu...' : 'Lưu vào dự án' }}
          </button>
          <span v-if="acquiredID === result.provider_result_id" class="field-help">Đã lưu. Media này có thể gán cho scene bên dưới.</span>
        </div>
      </article>
    </div>

    <div v-if="acquireError" class="notice error" data-testid="stock-acquire-error">
      {{ errorMessage(acquireError) }}
    </div>
    <div v-if="results.length" class="stock-pagination">
      <button class="secondary-button" type="button" :disabled="searching || page <= 1" @click="search(page - 1)">Trang trước</button>
      <span>Trang {{ page }}</span>
      <button class="secondary-button" type="button" :disabled="searching || !hasNextPage" @click="search(page + 1)">Trang sau</button>
    </div>
  </section>
</template>

<style scoped>
.stock-panel { margin-block: 1.5rem; padding: 1rem; border: 1px solid var(--color-border, #d8d8d8); border-radius: .75rem; }
.stock-heading, .stock-search, .stock-pagination, .stock-links { display: flex; gap: .75rem; align-items: end; flex-wrap: wrap; }
.stock-heading { justify-content: space-between; align-items: start; }
.stock-provider { font-size: .8rem; font-weight: 700; }
.stock-search label, .stock-meta { display: grid; gap: .35rem; }
.stock-search input, .stock-search select { min-height: 2.5rem; }
.stock-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); gap: 1rem; margin-top: 1rem; }
.stock-card { overflow: hidden; border: 1px solid var(--color-border, #d8d8d8); border-radius: .65rem; }
.stock-preview { aspect-ratio: 16 / 9; background: #111; }
.stock-preview img, .stock-preview video { width: 100%; height: 100%; object-fit: cover; }
.stock-meta { padding: .75rem; font-size: .9rem; }
.stock-pagination { justify-content: center; margin-top: 1rem; align-items: center; }
</style>
