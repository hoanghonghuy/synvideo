<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'

import { ApiError, listProjects, type Project } from '@/api/projects'

const { t } = useI18n()

const projects = ref<Project[]>([])
const nextCursor = ref('')
const loading = ref(true)
const loadingMore = ref(false)
const errorCode = ref('')

onMounted(() => {
  void loadProjects()
})

async function loadProjects(cursor = '') {
  errorCode.value = ''
  if (cursor) {
    loadingMore.value = true
  } else {
    loading.value = true
  }

  try {
    const response = await listProjects(cursor)
    projects.value = cursor ? [...projects.value, ...response.projects] : response.projects
    nextCursor.value = response.next_cursor ?? ''
  } catch (error) {
    errorCode.value = error instanceof ApiError ? error.code : 'request_failed'
  } finally {
    loading.value = false
    loadingMore.value = false
  }
}
</script>

<template>
  <section class="page">
    <div class="page-heading">
      <div>
        <p class="eyebrow">
          {{ t('projects.list.eyebrow') }}
        </p>
        <h1>{{ t('projects.list.title') }}</h1>
      </div>
      <RouterLink
        class="primary-button"
        to="/projects/new"
      >
        {{ t('projects.actions.create') }}
      </RouterLink>
    </div>

    <p
      v-if="loading"
      class="state-text"
    >
      {{ t('projects.states.loading') }}
    </p>
    <div
      v-else-if="errorCode"
      class="notice error"
    >
      <p>{{ t(`projects.errors.${errorCode}`) }}</p>
      <button
        class="secondary-button"
        type="button"
        @click="loadProjects()"
      >
        {{ t('projects.actions.retry') }}
      </button>
    </div>
    <div
      v-else-if="projects.length === 0"
      class="notice"
    >
      <p>{{ t('projects.list.empty') }}</p>
      <RouterLink
        class="secondary-button"
        to="/projects/new"
      >
        {{ t('projects.actions.create') }}
      </RouterLink>
    </div>
    <div
      v-else
      class="project-list"
    >
      <RouterLink
        v-for="project in projects"
        :key="project.id"
        class="project-row"
        :to="`/projects/${project.id}`"
      >
        <span>
          <strong>{{ project.title }}</strong>
          <small>{{ t(`projects.contentFormat.${project.content_format}`) }} - {{ project.aspect_ratio }}</small>
        </span>
        <span class="status-value">{{ t(`projects.status.${project.status}`) }}</span>
      </RouterLink>
      <button
        v-if="nextCursor"
        class="secondary-button"
        type="button"
        :disabled="loadingMore"
        @click="loadProjects(nextCursor)"
      >
        {{ loadingMore ? t('projects.states.loading') : t('projects.actions.loadMore') }}
      </button>
    </div>
  </section>
</template>
