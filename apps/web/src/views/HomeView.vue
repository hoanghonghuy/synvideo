<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
</script>

<template>
  <section class="page">
    <p class="eyebrow">
      {{ t('home.eyebrow') }}
    </p>
    <h1>{{ t('home.title') }}</h1>
    <p class="body-copy">
      {{ t('home.description') }}
    </p>
  </section>
</template>
