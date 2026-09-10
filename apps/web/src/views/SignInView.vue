<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'

import { isOidcConfigured } from '@/auth/config'
import { beginSignIn, sanitizeReturnTo } from '@/auth/oidc'

const route = useRoute()
const heading = ref<HTMLElement | null>(null)
const errorMessage = ref('')
const signingIn = ref(false)

const returnTo = sanitizeReturnTo(typeof route.query.returnTo === 'string' ? route.query.returnTo : '/projects')
const configured = isOidcConfigured()
const reason = computed(() => typeof route.query.reason === 'string' ? route.query.reason : '')
const stateCopy = computed(() => {
  if (errorMessage.value) {
    return {
      title: 'Đăng nhập cần được xử lý',
      body: errorMessage.value,
    }
  }
  if (reason.value === 'signed-out') {
    return {
      title: 'Bạn đã đăng xuất',
      body: 'Phiên đăng nhập trong bộ nhớ đã được xóa an toàn. Đăng nhập lại khi bạn muốn tiếp tục làm video.',
    }
  }
  if (reason.value === 'session-expired') {
    return {
      title: 'Phiên đăng nhập đã hết hạn',
      body: 'Đăng nhập lại để quay về đúng không gian làm việc bạn đang sử dụng.',
    }
  }
  return {
    title: 'Đăng nhập để tiếp tục',
    body: 'Phiên đăng nhập không còn trong tab này. Hãy đăng nhập lại để trở về không gian làm việc một cách an toàn.',
  }
})

async function signIn() {
  errorMessage.value = ''
  signingIn.value = true
  try {
    await beginSignIn(returnTo)
  } catch (error) {
    signingIn.value = false
    errorMessage.value = error instanceof Error ? error.message : 'Không thể bắt đầu đăng nhập.'
    await nextTick()
    heading.value?.focus()
  }
}

onMounted(() => {
  if (!configured) {
    errorMessage.value = 'Môi trường này chưa được cấu hình đăng nhập.'
  }
  void nextTick(() => heading.value?.focus())
})
</script>

<template>
  <section class="page">
    <div class="panel auth-panel">
      <p class="eyebrow">Tài khoản SynVideo</p>
      <h1
        ref="heading"
        tabindex="-1"
      >
        {{ stateCopy.title }}
      </h1>
      <p class="body-copy">
        {{ stateCopy.body }}
      </p>
      <button
        v-if="configured"
        class="primary-button inline-action"
        type="button"
        :disabled="signingIn"
        @click="signIn"
      >
        {{ signingIn ? 'Đang mở đăng nhập…' : errorMessage ? 'Thử đăng nhập lại' : 'Đăng nhập' }}
      </button>
    </div>
  </section>
</template>
