<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'

import { isOidcConfigured } from '@/auth/config'
import { beginSignIn, sanitizeReturnTo } from '@/auth/oidc'

const route = useRoute()
const { t } = useI18n()
const heading = ref<HTMLElement | null>(null)
const errorMessage = ref('')
const signingIn = ref(false)

const returnTo = sanitizeReturnTo(typeof route.query.returnTo === 'string' ? route.query.returnTo : '/projects')
const configured = isOidcConfigured()
const reason = computed(() => typeof route.query.reason === 'string' ? route.query.reason : '')
const stateCopy = computed(() => {
  if (errorMessage.value) {
    return {
      title: t('auth.signIn.errorTitle'),
      body: errorMessage.value,
    }
  }
  if (reason.value === 'signed-out') {
    return {
      title: t('auth.signIn.signedOutTitle'),
      body: t('auth.signIn.signedOutBody'),
    }
  }
  if (reason.value === 'session-expired') {
    return {
      title: t('auth.signIn.expiredTitle'),
      body: t('auth.signIn.expiredBody'),
    }
  }
  return {
    title: t('auth.signIn.defaultTitle'),
    body: t('auth.signIn.defaultBody'),
  }
})

async function signIn() {
  errorMessage.value = ''
  signingIn.value = true
  try {
    await beginSignIn(returnTo)
  } catch (error) {
    signingIn.value = false
    errorMessage.value = error instanceof Error ? error.message : t('auth.signIn.startFailed')
    await nextTick()
    heading.value?.focus()
  }
}

onMounted(() => {
  if (!configured) {
    errorMessage.value = t('auth.signIn.unconfigured')
  }
  void nextTick(() => heading.value?.focus())
})
</script>

<template>
  <section class="page">
    <div class="panel auth-panel">
      <p class="eyebrow">{{ t('auth.eyebrow') }}</p>
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
        {{ signingIn ? t('auth.signIn.pendingAction') : errorMessage ? t('auth.signIn.retryAction') : t('auth.signIn.action') }}
      </button>
    </div>
  </section>
</template>
