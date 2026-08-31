<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const checks = [
  { labelKey: 'status.frontend', valueKey: 'status.ready' },
  { labelKey: 'status.router', valueKey: 'status.ready' },
  { labelKey: 'status.localization', valueKey: 'status.ready' },
] as const
</script>

<template>
  <section class="page">
    <p class="eyebrow">
      {{ t('status.eyebrow') }}
    </p>
    <h1>{{ t('status.title') }}</h1>
    <ul class="status-list">
      <li
        v-for="check in checks"
        :key="check.labelKey"
      >
        <span>{{ t(check.labelKey) }}</span>
        <span class="status-value">{{ t(check.valueKey) }}</span>
      </li>
    </ul>
  </section>
</template>
