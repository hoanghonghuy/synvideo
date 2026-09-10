<script setup lang="ts">
import { nextTick, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { isOidcConfigured } from '@/auth/config'
import { completeSignInFromCallback, pendingSignInReturnTo } from '@/auth/oidc'

const route = useRoute()
const router = useRouter()
const errorMessage = ref('')
const errorHeading = ref<HTMLElement | null>(null)
const recoveryReturnTo = ref('/projects')

async function showFailure(message: string) {
  errorMessage.value = message
  await nextTick()
  errorHeading.value?.focus()
}

async function recoverSignIn() {
  await router.replace({
    path: '/sign-in',
    query: {
      reason: 'callback-failed',
      returnTo: recoveryReturnTo.value,
    },
  })
}

onMounted(async () => {
  if (!isOidcConfigured()) {
    await showFailure('Không thể hoàn tất đăng nhập vì OIDC chưa được cấu hình.')
    return
  }

  recoveryReturnTo.value = pendingSignInReturnTo()

  try {
    const returnTo = await completeSignInFromCallback(route.fullPath.split('?')[1] ?? '')
    await router.replace(returnTo)
  } catch (error) {
    await showFailure(error instanceof Error ? error.message : 'Không thể hoàn tất đăng nhập.')
  }
})
</script>

<template>
  <section class="panel" aria-live="polite">
    <div v-if="errorMessage" class="auth-callback-error">
      <h1 ref="errorHeading" tabindex="-1">
        Đăng nhập chưa hoàn tất
      </h1>
      <p>{{ errorMessage }}</p>
      <button type="button" class="recovery-action" @click="recoverSignIn">
        Thử đăng nhập lại
      </button>
    </div>
    <div v-else role="status" aria-busy="true">
      <h1>Đang hoàn tất đăng nhập</h1>
      <p>Vui lòng chờ trong giây lát…</p>
    </div>
  </section>
</template>

<style scoped>
.auth-callback-error {
  display: grid;
  gap: 12px;
}

.recovery-action {
  min-height: 44px;
  width: fit-content;
  border: 0;
  border-radius: 6px;
  padding: 0 16px;
  font: inherit;
  font-weight: 700;
  cursor: pointer;
}

.recovery-action:focus-visible,
h1:focus-visible {
  outline: 3px solid currentColor;
  outline-offset: 3px;
}
</style>
