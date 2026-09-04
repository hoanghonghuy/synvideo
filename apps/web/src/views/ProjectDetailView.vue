<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'

import { ApiError, getProject, updateProject, type Project, type UpdateProjectPayload } from '@/api/projects'
import ProjectForm from '@/components/ProjectForm.vue'
import generatedImageMessages from '@/features/generated-image/messages'

const { t, d } = useI18n()
const { t: tGeneratedImage } = useI18n({ useScope: 'local', messages: generatedImageMessages })
const route = useRoute()

const project = ref<Project | null>(null)
const loading = ref(true)
const submitting = ref(false)
const errorCode = ref('')
const fieldErrors = ref<Record<string, string>>({})
const saved = ref(false)

onMounted(() => {
  void loadProject()
})

async function loadProject() {
  loading.value = true
  errorCode.value = ''
  saved.value = false
  try {
    project.value = await getProject(String(route.params.id))
  } catch (error) {
    errorCode.value = error instanceof ApiError ? error.code : 'request_failed'
  } finally {
    loading.value = false
  }
}

async function submit(payload: UpdateProjectPayload) {
  if (!project.value) {
    return
  }
  submitting.value = true
  errorCode.value = ''
  fieldErrors.value = {}
  saved.value = false
  try {
    project.value = await updateProject(project.value.id, payload)
    saved.value = true
  } catch (error) {
    if (error instanceof ApiError) {
      errorCode.value = error.code
      fieldErrors.value = error.fields
    } else {
      errorCode.value = 'request_failed'
    }
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <section class="page">
    <RouterLink
      class="text-link"
      to="/projects"
    >
      {{ t('projects.actions.backToList') }}
    </RouterLink>

    <p
      v-if="loading"
      class="state-text"
    >
      {{ t('projects.states.loading') }}
    </p>
    <div
      v-else-if="errorCode && !project"
      class="notice error"
    >
      <p>{{ t(`projects.errors.${errorCode}`) }}</p>
      <button
        class="secondary-button"
        type="button"
        @click="loadProject"
      >
        {{ t('projects.actions.retry') }}
      </button>
    </div>
    <template v-else-if="project">
      <p class="eyebrow">
        {{ t('projects.detail.eyebrow') }}
      </p>
      <h1>{{ project.title }}</h1>
      <p class="body-copy">
        {{ t('projects.detail.updatedAt', { value: d(new Date(project.updated_at), 'long') }) }}
      </p>
      <p class="workspace-links">
        <RouterLink
          class="text-link"
          :to="`/projects/${project.id}/creative-brief`"
        >
          {{ t('creativeBrief.actions.open') }}
        </RouterLink>
        <RouterLink
          class="text-link"
          :to="`/projects/${project.id}/creative-proposal`"
        >
          {{ t('creativeProposal.actions.open') }}
        </RouterLink>
        <RouterLink
          class="text-link"
          :to="`/projects/${project.id}/script`"
        >
          {{ t('script.actions.open') }}
        </RouterLink>
        <RouterLink
          class="text-link"
          :to="`/projects/${project.id}/scene-plan`"
        >
          {{ t('scenePlan.actions.open') }}
        </RouterLink>
        <RouterLink
          class="text-link"
          :to="`/projects/${project.id}/media`"
        >
          {{ t('navigation.media') }}
        </RouterLink>
        <RouterLink
          class="text-link"
          :to="`/projects/${project.id}/images`"
        >
          {{ tGeneratedImage('generatedImage.title') }}
        </RouterLink>
        <RouterLink
          class="text-link"
          :to="`/projects/${project.id}/narration`"
        >
          {{ t('navigation.sceneNarration') }}
        </RouterLink>
      </p>

      <div
        v-if="saved"
        class="notice success"
      >
        {{ t('projects.states.saved') }}
      </div>
      <div
        v-if="errorCode"
        class="notice error"
      >
        {{ t(`projects.errors.${errorCode}`) }}
      </div>
      <ProjectForm
        include-status
        :initial-project="project"
        :submitting="submitting"
        :submit-label="t('projects.actions.saveUpdate')"
        :field-errors="fieldErrors"
        @submit="submit"
      />
    </template>
  </section>
</template>
