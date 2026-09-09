<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { completeSignInFromCallback } from '@/auth/oidc'
import { isOidcConfigured } from '@/auth/config'

const route = useRoute()
const router = useRouter()
const errorMessage = ref('')

onMounted(async () => {
  if (!isOidcConfigured()) {
    errorMessage.value = 'OIDC is not configured.'
    return
  }

  try {
    const returnTo = await completeSignInFromCallback(route.fullPath.split('?')[1] ?? '')
    await router.replace(returnTo)
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Sign-in failed.'
  }
})
</script>

<template>
  <section class="panel">
    <p v-if="errorMessage">
      {{ errorMessage }}
    </p>
    <p v-else>
      Completing sign-in...
    </p>
  </section>
</template>
