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

      <nav
        class="workspace-nav"
        :aria-label="t('projects.detail.eyebrow')"
      >
        <div class="workspace-links">
          <RouterLink
            class="workspace-link"
            :to="`/projects/${project.id}/creative-brief`"
          >
            {{ t('creativeBrief.actions.open') }}
          </RouterLink>
          <RouterLink
            class="workspace-link"
            :to="`/projects/${project.id}/creative-proposal`"
          >
            {{ t('creativeProposal.actions.open') }}
          </RouterLink>
          <RouterLink
            class="workspace-link"
            :to="`/projects/${project.id}/script`"
          >
            {{ t('script.actions.open') }}
          </RouterLink>
          <RouterLink
            class="workspace-link"
            :to="`/projects/${project.id}/scene-plan`"
          >
            {{ t('scenePlan.actions.open') }}
          </RouterLink>
          <RouterLink
            class="workspace-link"
            :to="`/projects/${project.id}/media`"
          >
            {{ t('navigation.media') }}
          </RouterLink>
          <RouterLink
            class="workspace-link"
            :to="`/projects/${project.id}/media/stock`"
          >
            Stock Media
          </RouterLink>
          <RouterLink
            class="workspace-link"
            :to="`/projects/${project.id}/images`"
          >
            {{ tGeneratedImage('generatedImage.title') }}
          </RouterLink>
          <RouterLink
            class="workspace-link"
            :to="`/projects/${project.id}/narration`"
          >
            {{ t('navigation.sceneNarration') }}
          </RouterLink>
          <RouterLink
            class="workspace-link"
            :to="`/projects/${project.id}/captions`"
          >
            Captions &amp; timing
          </RouterLink>
          <RouterLink
            class="workspace-link"
            :to="`/projects/${project.id}/audio-mix`"
          >
            Background music &amp; audio mix
          </RouterLink>
          <RouterLink
            class="workspace-link"
            :to="`/projects/${project.id}/scene-editor`"
          >
            Scene Editor
          </RouterLink>
          <RouterLink
            class="workspace-link"
            :to="`/projects/${project.id}/scene-video`"
          >
            Tạo video phân cảnh
          </RouterLink>
        </div>
      </nav>

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

<style scoped>
.workspace-nav {
  margin: 24px 0;
}

.workspace-links {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 220px), 1fr));
  gap: 12px;
}

.workspace-link {
  display: flex;
  min-height: 44px;
  box-sizing: border-box;
  align-items: center;
  padding: 12px 14px;
  border: 1px solid #cbd6d0;
  border-radius: 8px;
  color: #143d36;
  font-weight: 700;
  overflow-wrap: anywhere;
  text-decoration: none;
}

.workspace-link:hover {
  background: #f4f8f6;
}

.workspace-link:focus-visible {
  outline: 3px solid #7aa995;
  outline-offset: 2px;
}

@media (max-width: 720px) {
  .workspace-links {
    grid-template-columns: 1fr;
  }
}
</style>
