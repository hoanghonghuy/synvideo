<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink, useRoute } from 'vue-router'

import StockMediaPanel from './StockMediaPanel.vue'
import type { MediaAsset } from './api'

const route = useRoute()
const projectID = computed(() => String(route.params.id))
const lastAcquired = ref<MediaAsset | null>(null)

function onAcquired(asset: MediaAsset) {
  lastAcquired.value = asset
}
</script>

<template>
  <section class="page">
    <RouterLink class="text-link" :to="`/projects/${projectID}/media`">← Quay lại Media Library</RouterLink>
    <div class="page-heading">
      <div>
        <p class="eyebrow">Stock Media</p>
        <h1>Tìm và lưu media stock</h1>
        <p class="body-copy">Kết quả tìm kiếm chỉ là candidate từ provider. Media chỉ trở thành tài nguyên của dự án sau khi bấm “Lưu vào dự án”.</p>
      </div>
    </div>

    <StockMediaPanel :project-id="projectID" @acquired="onAcquired" />

    <div v-if="lastAcquired" class="notice info" data-testid="stock-acquired-notice">
      Đã lưu {{ lastAcquired.original_filename || lastAcquired.id }} vào Media Library.
      <RouterLink class="text-link" :to="`/projects/${projectID}/media`">Mở Media Library để gán cho scene</RouterLink>.
    </div>
  </section>
</template>
