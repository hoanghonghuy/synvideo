import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import type { SceneEditorScene, SceneEditorView } from './api'

const mocks = vi.hoisted(() => ({
  getSceneEditor: vi.fn(),
  updateSceneEditor: vi.fn(),
  listRenderExportHistory: vi.fn(),
  duplicateScene: vi.fn(),
}))

vi.mock('./api', () => ({
  cancelRenderExport: vi.fn(),
  createRenderExport: vi.fn(),
  createSceneEditorSnapshot: vi.fn(),
  duplicateScene: mocks.duplicateScene,
  getRenderExport: vi.fn(),
  getSceneEditor: mocks.getSceneEditor,
  listRenderExportHistory: mocks.listRenderExportHistory,
  mediaAssetContentURL: vi.fn(() => ''),
  previewSceneEditorReconcile: vi.fn(),
  reconcileSceneEditor: vi.fn(),
  removeScene: vi.fn(),
  reorderScene: vi.fn(),
  retryRenderExport: vi.fn(),
  updateSceneEditor: mocks.updateSceneEditor,
}))
vi.mock('@/features/scene-plan/api', () => ({ listScenePlans: vi.fn().mockResolvedValue([]) }))
vi.mock('./upstreamBridge', () => ({
  buildReconcileCandidate: vi.fn(),
  forkApprovedScriptForComposition: vi.fn(),
  latestApprovedScenePlanVersion: vi.fn(() => null),
  loadUpstreamBridgeGuidance: vi.fn().mockResolvedValue(null),
}))

import SceneEditorWorkspaceView from './SceneEditorWorkspaceView.vue'

function scene(duration = 2_000): SceneEditorScene {
  return {
    id: 'scene-1', scene_key: 'intro', duration_ms: duration,
    visual_treatment: { fit: 'contain', position_x: 0, position_y: 0, scale: 1, mute_video: true },
    transition_out: { kind: 'cut', duration_ms: 0 },
  }
}

function view(duration = 2_000, revision = 3): SceneEditorView {
  return {
    id: 'composition-1', project_id: 'project-1', revision, scene_plan_version: 2,
    scenes: [scene(duration)], created_at: '2026-09-13T00:00:00Z', updated_at: '2026-09-13T00:00:00Z', state: 'CURRENT',
  }
}
beforeEach(() => {
  vi.clearAllMocks()
  mocks.getSceneEditor.mockResolvedValue(view())
  mocks.updateSceneEditor.mockResolvedValue(view(3_000, 4))
  mocks.listRenderExportHistory.mockResolvedValue({ items: [], next_cursor: null })
  mocks.duplicateScene.mockResolvedValue(view(2_000, 4))
})

async function mountEditor() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/projects/:id/scene-editor', component: SceneEditorWorkspaceView }],
  })
  router.push('/projects/project-1/scene-editor')
  await router.isReady()
  const wrapper = mount(SceneEditorWorkspaceView, { attachTo: document.body, global: { plugins: [router] } })
  await flushPromises()
  return wrapper
}

describe('Scene Editor local draft reset', () => {
  it('requires explicit confirmation, supports keeping edits, and discards only after confirm', async () => {
    const wrapper = await mountEditor()
    const duration = wrapper.find('input[type="number"]')
    await duration.setValue('3000')
    await wrapper.findAll('button').find((button) => button.text() === 'Reload saved revision')!.trigger('click')
    expect((duration.element as HTMLInputElement).value).toBe('3000')
    expect(wrapper.text()).toContain('Discard unsaved local changes')
    await flushPromises()
    expect((document.activeElement as HTMLElement | null)?.id).toBe('confirm-draft-reset')

    await wrapper.findAll('button').find((button) => button.text() === 'Keep editing')!.trigger('click')
    expect((duration.element as HTMLInputElement).value).toBe('3000')
    expect(wrapper.text()).not.toContain('Discard changes')

    await wrapper.findAll('button').find((button) => button.text() === 'Reload saved revision')!.trigger('click')
    await wrapper.find('#confirm-draft-reset').trigger('click')
    expect((wrapper.find('input[type="number"]').element as HTMLInputElement).value).toBe('2000')
    expect(wrapper.text()).toContain('Local draft reset to the authoritative saved revision.')
  })

  it('clears an armed reset when a structural mutation establishes new authoritative state', async () => {
    const wrapper = await mountEditor()
    const duration = wrapper.find('input[type="number"]')
    await duration.setValue('3000')
    await wrapper.findAll('button').find((button) => button.text() === 'Reload saved revision')!.trigger('click')
    expect(wrapper.text()).toContain('Discard changes')
    await duration.setValue('2000')
    await wrapper.findAll('button').find((button) => button.text() === 'Duplicate')!.trigger('click')
    await flushPromises()
    expect(mocks.duplicateScene).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).not.toContain('Discard changes')
  })

  it('clears an armed reset when a save establishes new authoritative state', async () => {
    const wrapper = await mountEditor()
    await wrapper.find('input[type="number"]').setValue('3000')
    await wrapper.findAll('button').find((button) => button.text() === 'Reload saved revision')!.trigger('click')
    expect(wrapper.text()).toContain('Discard changes')
    await wrapper.findAll('button').find((button) => button.text() === 'Save composition')!.trigger('click')
    await flushPromises()
    expect(mocks.updateSceneEditor).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).not.toContain('Discard changes')
  })
})

function flushPromises() {
  return new Promise((resolve) => window.setTimeout(resolve))
}
