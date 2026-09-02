import { createRouter, createWebHistory } from 'vue-router'

import CreativeBriefView from '@/features/creative-brief/CreativeBriefView.vue'
import CreativeProposalView from '@/features/creative-proposal/CreativeProposalView.vue'
import ScriptView from '@/features/script/ScriptView.vue'
import ScenePlanView from '@/features/scene-plan/ScenePlanView.vue'
import ProviderSettingsView from '@/features/provider-settings/ProviderSettingsView.vue'
import HomeView from '@/views/HomeView.vue'
import ProjectCreateView from '@/views/ProjectCreateView.vue'
import ProjectDetailView from '@/views/ProjectDetailView.vue'
import ProjectListView from '@/views/ProjectListView.vue'
import StatusView from '@/views/StatusView.vue'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      name: 'home',
      component: HomeView,
    },
    {
      path: '/status',
      name: 'status',
      component: StatusView,
    },
    {
      path: '/projects',
      name: 'projects',
      component: ProjectListView,
    },
    {
      path: '/projects/new',
      name: 'project-create',
      component: ProjectCreateView,
    },
    {
      path: '/projects/:id',
      name: 'project-detail',
      component: ProjectDetailView,
    },
    {
      path: '/projects/:id/creative-brief',
      name: 'creative-brief',
      component: CreativeBriefView,
    },
    {
      path: '/projects/:id/creative-proposal',
      name: 'creative-proposal',
      component: CreativeProposalView,
    },
    {
      path: '/projects/:id/script',
      name: 'script',
      component: ScriptView,
    },
    {
      path: '/projects/:id/scene-plan',
      name: 'scene-plan',
      component: ScenePlanView,
    },
    {
      path: '/settings/ai-providers',
      name: 'provider-settings',
      component: ProviderSettingsView,
    },
  ],
})
