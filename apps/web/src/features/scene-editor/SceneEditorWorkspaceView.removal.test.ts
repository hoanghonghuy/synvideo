import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import type { SceneEditorScene, SceneEditorView } from './api'

const mocks = vi.hoisted(() => ({
  getSceneEditor: vi.fn(),
  removeScene: vi.fn(),
  listRenderExportHistory: vi.fn(),
}))

vi.mock('./api', () => ({
  cancelRenderExport: vi.fn(),
  createRenderExport: vi.fn(),
  createSceneEditorSnapshot: vi.fn(),
  duplicateScene: vi.fn(),
  getRenderExport: vi.fn(),
  getSceneEditor: mocks.getSceneEditor,
  listRenderExportHistory: mocks.listRenderExportHistory,
  mediaAssetContentURL: vi.fn(() => ''),
  previewSceneEditorReconcile: vi.fn(),
  reconcileSceneEditor: vi.fn(),
  removeScene: mocks.removeScene,
  reorderScene: vi.fn(),
  retryRenderExport: vi.fn(),
  updateSceneEditor: vi.fn(),
}))

vi.mock('@/features/scene-plan/api', () => ({ listScenePlans: vi.fn().mockResolvedValue([]) }))
vi.mock('./upstreamBridge', () => ({
  buildReconcileCandidate: vi.fn(),
  forkApprovedScriptForComposition: vi.fn(),
  latestApprovedScenePlanVersion: vi.fn(() => null),
  loadUpstreamBridgeGuidance: vi.fn().mockResolvedValue(null),
}))

import SceneEditorWorkspaceView from './SceneEditorWorkspaceView.vue'

function scene(id: string, key: string): SceneEditorScene {
  return {
    id,
    scene_key: key,
    duration_ms: 2_000,
    visual_treatment: { fit: 'contain', position_x: 0, position_y: 0, scale: 1, mute_video: true },
    transition_out: { kind: 'cut', duration_ms: 0 },
  }
}

function view(scenes = [scene('scene-1', 'intro'), scene('scene-2', 'outro')]): SceneEditorView {
  return {
    id: 'composition-1', project_id: 'project-1', revision: 3, scene_plan_version: 2,
    scenes, created_at: '2026-09-13T00:00:00Z', updated_at: '2026-09-13T00:00:00Z', state: 'CURRENT',
  }
}

beforeEach(() => {
  vi.clearAllMocks()
  mocks.getSceneEditor.mockResolvedValue(view())
  mocks.listRenderExportHistory.mockResolvedValue({ items: [], next_cursor: null })
  mocks.removeScene.mockResolvedValue(view([scene('scene-2', 'outro')]))
})
describe('Scene Editor destructive scene removal', () => {
  it('requires explicit confirmation, supports cancel, and removes only after confirm', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/projects/:id/scene-editor', component: SceneEditorWorkspaceView }],
    })
    router.push('/projects/project-1/scene-editor')
    await router.isReady()

    const wrapper = mount(SceneEditorWorkspaceView, { global: { plugins: [router] } })
    await flushPromises()

    const removeButton = wrapper.findAll('button').find((button) => button.text() === 'Remove')
    expect(removeButton).toBeTruthy()
    await removeButton!.trigger('click')
    expect(mocks.removeScene).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('This permanently removes this scene from the composition.')

    await wrapper.findAll('button').find((button) => button.text() === 'Cancel')!.trigger('click')
    expect(mocks.removeScene).not.toHaveBeenCalled()
    expect(wrapper.text()).not.toContain('Confirm remove')

    await wrapper.findAll('button').find((button) => button.text() === 'Remove')!.trigger('click')
    await wrapper.findAll('button').find((button) => button.text() === 'Confirm remove')!.trigger('click')
    await flushPromises()

    expect(mocks.removeScene).toHaveBeenCalledTimes(1)
    expect(mocks.removeScene).toHaveBeenCalledWith('project-1', 'scene-1', 3)
    expect(wrapper.text()).not.toContain('Confirm remove')
  })
})

function flushPromises() {
  return new Promise((resolve) => window.setTimeout(resolve))
}
