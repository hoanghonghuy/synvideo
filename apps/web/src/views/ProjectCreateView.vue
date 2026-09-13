<script setup lang="ts">
import { nextTick, ref } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'

import { ApiError, createProject, type ProjectPayload } from '@/api/projects'
import ProjectForm from '@/components/ProjectForm.vue'

const { t } = useI18n()
const router = useRouter()

const submitting = ref(false)
const errorCode = ref('')
const fieldErrors = ref<Record<string, string>>({})
const errorNotice = ref<HTMLElement | null>(null)

async function submit(payload: ProjectPayload) {
  submitting.value = true
  errorCode.value = ''
  fieldErrors.value = {}
  try {
    const project = await createProject(payload)
    await router.push(`/projects/${project.id}`)
  } catch (error) {
    if (error instanceof ApiError) {
      errorCode.value = error.code
      fieldErrors.value = error.fields
      if (Object.keys(error.fields).length > 0) return
    } else {
      errorCode.value = 'request_failed'
    }
    await nextTick()
    errorNotice.value?.focus()
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <section class="page">
    <RouterLink class="text-link" to="/projects">
      {{ t('projects.actions.backToList') }}
    </RouterLink>
    <p class="eyebrow">
      {{ t('projects.create.eyebrow') }}
    </p>
    <h1>{{ t('projects.create.title') }}</h1>
    <div v-if="errorCode" ref="errorNotice" class="notice error" role="alert" aria-live="assertive" tabindex="-1">
      {{ t(`projects.errors.${errorCode}`) }}
    </div>
    <ProjectForm
      :submitting="submitting"
      :submit-label="t('projects.actions.saveCreate')"
      :field-errors="fieldErrors"
      @submit="submit"
    />
  </section>
</template>
