<script setup lang="ts">
import { nextTick, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'

import { isOidcConfigured } from '@/auth/config'
import { beginSignIn, sanitizeReturnTo } from '@/auth/oidc'

const route = useRoute()
const heading = ref<HTMLElement | null>(null)
const errorMessage = ref('')
const signingIn = ref(false)

const returnTo = sanitizeReturnTo(typeof route.query.returnTo === 'string' ? route.query.returnTo : '/projects')
const configured = isOidcConfigured()

async function signIn() {
  errorMessage.value = ''
  signingIn.value = true
  try {
    await beginSignIn(returnTo)
  } catch (error) {
    signingIn.value = false
    errorMessage.value = error instanceof Error ? error.message : 'Unable to start sign-in.'
    await nextTick()
    heading.value?.focus()
  }
}

onMounted(() => {
  if (!configured) {
    errorMessage.value = 'Sign-in is not configured for this environment.'
  }
  void nextTick(() => heading.value?.focus())
})
</script>

<template>
  <section class="page">
    <div class="panel auth-panel">
      <p class="eyebrow">SynVideo account</p>
      <h1
        ref="heading"
        tabindex="-1"
      >
        {{ errorMessage ? 'Sign-in needs attention' : 'Sign in to continue' }}
      </h1>
      <p class="body-copy">
        <template v-if="errorMessage">
          {{ errorMessage }}
        </template>
        <template v-else>
          Your session is not available in this tab. Sign in again to return to your workspace securely.
        </template>
      </p>
      <button
        v-if="configured"
        class="primary-button inline-action"
        type="button"
        :disabled="signingIn"
        @click="signIn"
      >
        {{ signingIn ? 'Opening sign-in…' : errorMessage ? 'Try sign-in again' : 'Sign in' }}
      </button>
    </div>
  </section>
</template>
