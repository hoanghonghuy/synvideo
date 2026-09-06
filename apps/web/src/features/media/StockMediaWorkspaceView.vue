<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'

import StockMediaPanel from './StockMediaPanel.vue'
import type { MediaAsset } from './api'
import stockMessages from './stockMessages'

const route = useRoute()
const { t } = useI18n({ useScope: 'local', messages: stockMessages })
const projectID = computed(() => String(route.params.id))
const lastAcquired = ref<MediaAsset | null>(null)

function onAcquired(asset: MediaAsset) {
  lastAcquired.value = asset
}
</script>

<template>
  <section class="page">
    <RouterLink class="text-link" :to="`/projects/${projectID}/media`">← {{ t('stock.backToLibrary') }}</RouterLink>
    <div class="page-heading">
      <div>
        <p class="eyebrow">{{ t('stock.workspaceEyebrow') }}</p>
        <h1>{{ t('stock.workspaceTitle') }}</h1>
        <p class="body-copy">{{ t('stock.workspaceDescription') }}</p>
      </div>
    </div>

    <StockMediaPanel :project-id="projectID" @acquired="onAcquired" />

    <div v-if="lastAcquired" class="notice info" data-testid="stock-acquired-notice" role="status">
      {{ t('stock.acquiredNotice', { name: lastAcquired.original_filename || lastAcquired.id }) }}
      <RouterLink class="text-link" :to="`/projects/${projectID}/media`">{{ t('stock.openLibrary') }}</RouterLink>.
    </div>
  </section>
</template>
