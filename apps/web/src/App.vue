<script setup lang="ts">
import { RouterLink, RouterView } from 'vue-router'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
</script>

<template>
  <div class="app-shell">
    <header class="app-header">
      <RouterLink
        class="brand"
        to="/"
      >
        {{ t('app.name') }}
      </RouterLink>
      <nav
        class="nav"
        :aria-label="t('navigation.primary')"
      >
        <RouterLink to="/">
          {{ t('navigation.home') }}
        </RouterLink>
        <RouterLink to="/status">
          {{ t('navigation.status') }}
        </RouterLink>
        <RouterLink to="/projects">
          {{ t('navigation.projects') }}
        </RouterLink>
        <RouterLink to="/settings/ai-providers">
          {{ t('navigation.providerSettings') }}
        </RouterLink>
      </nav>
    </header>
    <main>
      <RouterView />
    </main>
  </div>
</template>
