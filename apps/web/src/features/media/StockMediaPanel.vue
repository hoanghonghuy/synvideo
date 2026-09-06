<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { ApiError } from '@/api/projects'
import {
  acquireStockMedia,
  searchStockMedia,
  type MediaAsset,
  type StockMediaKind,
  type StockMediaOrientation,
  type StockMediaResult,
} from './api'
import stockMessages from './stockMessages'

const props = defineProps<{ projectId: string }>()
const emit = defineEmits<{ acquired: [asset: MediaAsset] }>()
const { t } = useI18n({ useScope: 'local', messages: stockMessages })

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
const hasSearched = ref(false)

const canSearch = computed(() => query.value.trim().length > 0 && !searching.value)

async function search(targetPage = 1) {
  if (!canSearch.value && targetPage === 1) return
  searching.value = true
  searchError.value = ''
  acquireError.value = ''
  hasSearched.value = true
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
  const key = `stock.errors.${code}`
  const translated = t(key)
  return translated === key ? t('stock.errors.fallback') : translated
}

function creator(result: StockMediaResult) {
  return result.creator_name || t('stock.unknownCreator')
}
</script>

<template>
  <section class="stock-panel" data-testid="stock-media-panel" aria-labelledby="stock-media-title">
    <div class="stock-heading">
      <div>
        <h2 id="stock-media-title">{{ t('stock.title') }}</h2>
        <p>{{ t('stock.description') }}</p>
      </div>
      <span class="stock-provider">Pexels</span>
    </div>

    <form class="stock-search" @submit.prevent="search(1)">
      <label>
        {{ t('stock.query') }}
        <input v-model="query" type="search" :placeholder="t('stock.queryPlaceholder')" data-testid="stock-query">
      </label>
      <label>
        {{ t('stock.kind') }}
        <select v-model="kind" data-testid="stock-kind">
          <option value="image">{{ t('stock.image') }}</option>
          <option value="video">{{ t('stock.video') }}</option>
        </select>
      </label>
      <label>
        {{ t('stock.orientation') }}
        <select v-model="orientation">
          <option value="">{{ t('stock.any') }}</option>
          <option value="landscape">{{ t('stock.landscape') }}</option>
          <option value="portrait">{{ t('stock.portrait') }}</option>
          <option value="square">{{ t('stock.square') }}</option>
        </select>
      </label>
      <button class="primary-button" type="submit" :disabled="!canSearch">
        {{ searching ? t('stock.searching') : t('stock.search') }}
      </button>
    </form>

    <div v-if="searchError" class="notice error" data-testid="stock-search-error" role="alert">
      {{ errorMessage(searchError) }}
    </div>
    <p v-else-if="hasSearched && !searching && results.length === 0 && page > 1" class="notice info">{{ t('stock.noMoreResults') }}</p>
    <p v-else-if="hasSearched && !searching && results.length === 0" class="field-help">{{ t('stock.noResults') }}</p>

    <div v-if="results.length" class="stock-grid" aria-live="polite">
      <article v-for="result in results" :key="`${result.provider_key}:${result.provider_result_id}`" class="stock-card">
        <div class="stock-preview">
          <img v-if="result.kind === 'image'" :src="result.preview_url" :alt="t('stock.previewAlt', { creator: creator(result) })">
          <video v-else :poster="result.preview_url" muted controls preload="none" :aria-label="t('stock.videoPreview', { creator: creator(result) })" />
        </div>
        <div class="stock-meta">
          <strong>{{ creator(result) }}</strong>
          <span>{{ result.license_summary }}</span>
          <span>{{ result.attribution_text }}</span>
          <div class="stock-links">
            <a :href="result.source_page_url" target="_blank" rel="noopener noreferrer">{{ t('stock.source') }}</a>
            <a v-if="result.license_reference" :href="result.license_reference" target="_blank" rel="noopener noreferrer">{{ t('stock.license') }}</a>
          </div>
          <button
            class="secondary-button"
            type="button"
            :disabled="!result.acquirable || Boolean(acquiringID)"
            :data-testid="`stock-acquire-${result.provider_result_id}`"
            @click="acquire(result)"
          >
            {{ acquiringID === result.provider_result_id ? t('stock.acquiring') : t('stock.acquire') }}
          </button>
          <span v-if="acquiredID === result.provider_result_id" class="field-help" role="status">{{ t('stock.acquired') }}</span>
        </div>
      </article>
    </div>

    <div v-if="acquireError" class="notice error" data-testid="stock-acquire-error" role="alert">
      {{ errorMessage(acquireError) }}
    </div>
    <div v-if="results.length" class="stock-pagination">
      <button class="secondary-button" type="button" :disabled="searching || page <= 1" @click="search(page - 1)">{{ t('stock.previous') }}</button>
      <span>{{ t('stock.page', { page }) }}</span>
      <button class="secondary-button" type="button" :disabled="searching || !hasNextPage" @click="search(page + 1)">{{ t('stock.next') }}</button>
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
