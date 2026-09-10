<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'

import { ApiError, listProjects, type Project } from '@/api/projects'

const { t, locale } = useI18n()

const projects = ref<Project[]>([])
const nextCursor = ref('')
const loading = ref(true)
const loadingMore = ref(false)
const errorCode = ref('')

const dateFormatter = computed(
  () =>
    new Intl.DateTimeFormat(locale.value, {
      dateStyle: 'medium',
      timeStyle: 'short',
    }),
)

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

function formatUpdatedAt(value: string) {
  const parsed = new Date(value)
  return Number.isNaN(parsed.getTime()) ? value : dateFormatter.value.format(parsed)
}
</script>

<template>
  <section class="page">
    <div class="page-heading project-library-heading">
      <div>
        <p class="eyebrow">
          {{ t('projects.list.eyebrow') }}
        </p>
        <h1>{{ t('projects.list.title') }}</h1>
      </div>
      <RouterLink
        class="primary-button project-library-create"
        to="/projects/new"
      >
        {{ t('projects.actions.create') }}
      </RouterLink>
    </div>

    <div
      v-if="loading"
      class="project-library-loading"
      role="status"
      aria-live="polite"
    >
      <p class="state-text">
        {{ t('projects.states.loading') }}
      </p>
      <div
        class="project-library-grid"
        aria-hidden="true"
      >
        <div
          v-for="index in 3"
          :key="index"
          class="project-card project-card-skeleton"
        >
          <span class="skeleton-line skeleton-title" />
          <span class="skeleton-line skeleton-copy" />
          <span class="skeleton-line skeleton-meta" />
        </div>
      </div>
    </div>
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
      class="project-library"
    >
      <div class="project-library-grid">
        <RouterLink
          v-for="project in projects"
          :key="project.id"
          class="project-card"
          :to="`/projects/${project.id}`"
        >
          <div class="project-card-topline">
            <span class="project-card-format">
              {{ t(`projects.contentFormat.${project.content_format}`) }} · {{ project.aspect_ratio }}
            </span>
            <span
              class="project-status-chip"
              :class="`project-status-${project.status}`"
            >
              {{ t(`projects.status.${project.status}`) }}
            </span>
          </div>

          <div class="project-card-content">
            <strong class="project-card-title">{{ project.title }}</strong>
            <p
              v-if="project.description"
              class="project-card-description"
            >
              {{ project.description }}
            </p>
          </div>

          <small class="project-card-updated">
            {{ t('projects.detail.updatedAt', { value: formatUpdatedAt(project.updated_at) }) }}
          </small>
        </RouterLink>
      </div>

      <button
        v-if="nextCursor"
        class="secondary-button project-library-load-more"
        type="button"
        :disabled="loadingMore"
        @click="loadProjects(nextCursor)"
      >
        {{ loadingMore ? t('projects.states.loading') : t('projects.actions.loadMore') }}
      </button>
    </div>
  </section>
</template>

<style scoped>
.project-library-heading {
  align-items: center;
}

.project-library-create,
.project-library-load-more {
  min-height: 44px;
}

.project-library,
.project-library-loading {
  display: grid;
  gap: 18px;
}

.project-library-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 280px), 1fr));
  gap: 16px;
}

.project-card {
  min-width: 0;
  min-height: 170px;
  display: grid;
  grid-template-rows: auto 1fr auto;
  gap: 18px;
  border: 1px solid #d7ddd5;
  border-radius: 12px;
  padding: 18px;
  background: #ffffff;
  transition:
    border-color 140ms ease,
    box-shadow 140ms ease,
    transform 140ms ease;
}

.project-card:hover {
  border-color: #9db4aa;
  box-shadow: 0 8px 24px rgb(20 61 54 / 8%);
  transform: translateY(-1px);
}

.project-card:focus-visible,
.project-library-create:focus-visible,
.project-library-load-more:focus-visible {
  outline: 3px solid #27634e;
  outline-offset: 3px;
}

.project-card-topline {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.project-card-format,
.project-card-updated {
  color: #647275;
  font-size: 13px;
  font-weight: 600;
}

.project-status-chip {
  flex: 0 0 auto;
  border: 1px solid currentColor;
  border-radius: 999px;
  padding: 4px 8px;
  font-size: 12px;
  font-weight: 700;
  line-height: 1.2;
}

.project-status-active {
  color: #27634e;
  background: #f4fbf7;
}

.project-status-archived {
  color: #5c6668;
  background: #f4f5f4;
}

.project-card-content {
  min-width: 0;
}

.project-card-title {
  display: block;
  overflow-wrap: anywhere;
  font-size: 19px;
  line-height: 1.3;
}

.project-card-description {
  display: -webkit-box;
  overflow: hidden;
  margin: 8px 0 0;
  color: #536164;
  font-size: 14px;
  line-height: 1.5;
  overflow-wrap: anywhere;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 3;
}

.project-card-updated {
  overflow-wrap: anywhere;
}

.project-library-load-more {
  justify-self: center;
}

.project-card-skeleton {
  pointer-events: none;
}

.skeleton-line {
  display: block;
  border-radius: 6px;
  background: #e9eeeb;
  animation: skeleton-pulse 1.4s ease-in-out infinite alternate;
}

.skeleton-title {
  width: 58%;
  height: 18px;
}

.skeleton-copy {
  width: 90%;
  height: 42px;
}

.skeleton-meta {
  width: 68%;
  height: 14px;
}

@keyframes skeleton-pulse {
  from {
    opacity: 0.55;
  }
  to {
    opacity: 1;
  }
}

@media (prefers-reduced-motion: reduce) {
  .project-card,
  .skeleton-line {
    animation: none;
    transition: none;
  }
}

@media (max-width: 640px) {
  .project-library-heading {
    align-items: stretch;
  }

  .project-library-create {
    width: 100%;
    box-sizing: border-box;
  }

  .project-card {
    min-height: 0;
  }

  .project-card-topline {
    align-items: flex-start;
  }
}
</style>
