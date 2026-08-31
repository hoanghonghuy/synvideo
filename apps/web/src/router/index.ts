import { createRouter, createWebHistory } from 'vue-router'

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
  ],
})
