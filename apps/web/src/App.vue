<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'

import { isOidcConfigured } from '@/auth/config'
import { signOut } from '@/auth/oidc'
import { getAccessToken } from '@/auth/session'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const mainContent = ref<HTMLElement | null>(null)

const showSignOut = computed(() => {
  void route.fullPath
  return isOidcConfigured() && Boolean(getAccessToken())
})

function skipToMainContent() {
  mainContent.value?.focus()
}

watch(
  () => route.fullPath,
  async () => {
    await nextTick()
    mainContent.value?.focus()
  },
)

async function handleSignOut() {
  signOut()
  await router.replace({
    path: '/sign-in',
    query: { reason: 'signed-out' },
  })
}
</script>

<template>
  <div class="app-shell">
    <a
      class="skip-link"
      href="#main-content"
      @click.prevent="skipToMainContent"
    >
      Bỏ qua điều hướng
    </a>
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
        <button
          v-if="showSignOut"
          class="nav-auth-action"
          type="button"
          @click="handleSignOut"
        >
          Đăng xuất
        </button>
      </nav>
    </header>
    <main
      id="main-content"
      ref="mainContent"
      tabindex="-1"
    >
      <RouterView />
    </main>
  </div>
</template>

<style scoped>
.skip-link {
  position: fixed;
  top: 12px;
  left: 12px;
  z-index: 1000;
  min-height: 44px;
  display: inline-flex;
  align-items: center;
  padding: 0 14px;
  border-radius: 6px;
  background: #ffffff;
  color: #143d36;
  font-weight: 700;
  transform: translateY(calc(-100% - 24px));
}

.skip-link:focus-visible {
  transform: translateY(0);
  outline: 3px solid #7aa995;
  outline-offset: 2px;
}

.nav {
  flex-wrap: wrap;
  justify-content: flex-end;
  align-items: center;
}

.nav a,
.nav-auth-action {
  min-height: 44px;
  box-sizing: border-box;
}

.nav-auth-action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid #cbd6d0;
  border-radius: 6px;
  padding: 0 12px;
  background: #ffffff;
  color: #143d36;
  font: inherit;
  font-size: 14px;
  font-weight: 700;
  cursor: pointer;
}

.nav-auth-action:focus-visible {
  outline: 3px solid #7aa995;
  outline-offset: 2px;
}

@media (max-width: 720px) {
  .app-header {
    align-items: flex-start;
    flex-direction: column;
  }

  .nav {
    width: 100%;
    justify-content: flex-start;
  }
}
</style>
