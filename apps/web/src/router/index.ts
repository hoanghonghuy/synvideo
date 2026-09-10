import { createRouter, createWebHistory } from 'vue-router'

import AudioMixWorkspaceView from '@/features/audio-mix/AudioMixWorkspaceView.vue'
import CaptionWorkspaceView from '@/features/captions/CaptionWorkspaceView.vue'
import CreativeBriefView from '@/features/creative-brief/CreativeBriefView.vue'
import CreativeProposalView from '@/features/creative-proposal/CreativeProposalView.vue'
import GeneratedImageWorkspaceView from '@/features/generated-image/GeneratedImageWorkspaceView.vue'
import ScriptView from '@/features/script/ScriptView.vue'
import SceneEditorWorkspaceView from '@/features/scene-editor/SceneEditorWorkspaceView.vue'
import ScenePlanView from '@/features/scene-plan/ScenePlanView.vue'
import MediaWorkspaceView from '@/features/media/MediaWorkspaceView.vue'
import StockMediaWorkspaceView from '@/features/media/StockMediaWorkspaceView.vue'
import SceneNarrationWorkspaceView from '@/features/scene-narration/SceneNarrationWorkspaceView.vue'
import SceneVideoWorkspaceView from '@/features/scene-video/SceneVideoWorkspaceView.vue'
import ProviderSettingsView from '@/features/provider-settings/ProviderSettingsView.vue'
import { isOidcConfigured } from '@/auth/config'
import { getAccessToken } from '@/auth/session'
import AuthCallbackView from '@/views/AuthCallbackView.vue'
import HomeView from '@/views/HomeView.vue'
import ProjectCreateView from '@/views/ProjectCreateView.vue'
import ProjectDetailView from '@/views/ProjectDetailView.vue'
import ProjectListView from '@/views/ProjectListView.vue'
import SignInView from '@/views/SignInView.vue'
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
      path: '/sign-in',
      name: 'sign-in',
      component: SignInView,
    },
    {
      path: '/auth/callback',
      name: 'auth-callback',
      component: AuthCallbackView,
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
      path: '/projects/:id/media',
      name: 'media-workspace',
      component: MediaWorkspaceView,
    },
    {
      path: '/projects/:id/media/stock',
      name: 'stock-media-workspace',
      component: StockMediaWorkspaceView,
    },
    {
      path: '/projects/:id/images',
      name: 'generated-images',
      component: GeneratedImageWorkspaceView,
    },
    {
      path: '/projects/:id/narration',
      name: 'scene-narration',
      component: SceneNarrationWorkspaceView,
    },
    {
      path: '/projects/:id/captions',
      name: 'captions',
      component: CaptionWorkspaceView,
    },
    {
      path: '/projects/:id/audio-mix',
      name: 'audio-mix',
      component: AudioMixWorkspaceView,
    },
    {
      path: '/projects/:id/scene-editor',
      name: 'scene-editor',
      component: SceneEditorWorkspaceView,
    },
    {
      path: '/projects/:id/scene-video',
      name: 'scene-video',
      component: SceneVideoWorkspaceView,
    },
    {
      path: '/settings/ai-providers',
      name: 'provider-settings',
      component: ProviderSettingsView,
    },
  ],
})

router.beforeEach((to) => {
  const protectedCreatorRoute = to.path === '/projects' || to.path.startsWith('/projects/') || to.path.startsWith('/settings/')
  if (!protectedCreatorRoute || !isOidcConfigured() || getAccessToken()) {
    return true
  }

  return {
    path: '/sign-in',
    query: { returnTo: to.fullPath },
    replace: true,
  }
})
