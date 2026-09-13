import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import type { RenderExportJob, SceneEditorScene, SceneEditorView } from './api'
import { renderExportStorageKey } from './renderExportState'

const mocks = vi.hoisted(() => ({
  cancelRenderExport: vi.fn(),
  getRenderExport: vi.fn(),
  getSceneEditor: vi.fn(),
  listRenderExportHistory: vi.fn(),
}))

vi.mock('./api', () => ({
  cancelRenderExport: mocks.cancelRenderExport,
  createRenderExport: vi.fn(),
  createSceneEditorSnapshot: vi.fn(),
  duplicateScene: vi.fn(),
  getRenderExport: mocks.getRenderExport,
  getSceneEditor: mocks.getSceneEditor,
  listRenderExportHistory: mocks.listRenderExportHistory,
  mediaAssetContentURL: vi.fn(() => ''),
  previewSceneEditorReconcile: vi.fn(),
  reconcileSceneEditor: vi.fn(),
  removeScene: vi.fn(),
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

function scene(): SceneEditorScene {
  return {
    id: 'scene-1',
    scene_key: 'intro',
    duration_ms: 2_000,
    visual_treatment: { fit: 'contain', position_x: 0, position_y: 0, scale: 1, mute_video: true },
    transition_out: { kind: 'cut', duration_ms: 0 },
  }
}

function view(): SceneEditorView {
  return {
    id: 'composition-1', project_id: 'project-1', revision: 3, scene_plan_version: 2,
    scenes: [scene()], created_at: '2026-09-13T00:00:00Z', updated_at: '2026-09-13T00:00:00Z', state: 'CURRENT',
  }
}

function renderJob(id: string, state: RenderExportJob['state'] = 'running'): RenderExportJob {
  return {
    id,
    state,
    attempt: 1,
    max_attempts: 2,
    snapshot_digest: 'a'.repeat(64),
    profile_id: 'mp4-h264-720p',
    subtitle_mode: 'off',
    created_at: '2026-09-13T00:00:00Z',
    updated_at: '2026-09-13T00:00:00Z',
  }
}

async function mountView() {
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/projects/:id/scene-editor', component: SceneEditorWorkspaceView }] })
  router.push('/projects/project-1/scene-editor')
  await router.isReady()
  const wrapper = mount(SceneEditorWorkspaceView, { attachTo: document.body, global: { plugins: [router] } })
  await flushPromises()
  return wrapper
}

beforeEach(() => {
  vi.clearAllMocks()
  window.localStorage.clear()
  window.localStorage.setItem(renderExportStorageKey('project-1'), 'render-active')
  mocks.getSceneEditor.mockResolvedValue(view())
  mocks.getRenderExport.mockResolvedValue(renderJob('render-active'))
  mocks.listRenderExportHistory.mockResolvedValue({ items: [], next_cursor: null })
  mocks.cancelRenderExport.mockResolvedValue({ ...renderJob('render-active'), cancellation_pending: true })
})

describe('Scene Editor active render cancellation', () => {
  it('requires explicit confirmation, supports keeping the render, and cancels only after confirm', async () => {
    const wrapper = await mountView()

    await wrapper.findAll('button').find((button) => button.text() === 'Cancel render')!.trigger('click')
    expect(mocks.cancelRenderExport).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('Work already completed for this attempt may be lost.')
    await flushPromises()
    expect((document.activeElement as HTMLElement | null)?.textContent).toContain('Confirm cancel render')

    await wrapper.findAll('button').find((button) => button.text() === 'Keep rendering')!.trigger('click')
    expect(mocks.cancelRenderExport).not.toHaveBeenCalled()
    expect(wrapper.text()).not.toContain('Confirm cancel render')

    await wrapper.findAll('button').find((button) => button.text() === 'Cancel render')!.trigger('click')
    await wrapper.findAll('button').find((button) => button.text() === 'Confirm cancel render')!.trigger('click')
    await flushPromises()

    expect(mocks.cancelRenderExport).toHaveBeenCalledTimes(1)
    expect(mocks.cancelRenderExport).toHaveBeenCalledWith('project-1', 'render-active')
    expect(wrapper.text()).not.toContain('Confirm cancel render')
  })

  it('preserves an armed cancellation across no-op polls and clears it when authoritative state changes', async () => {
    const wrapper = await mountView()

    await wrapper.findAll('button').find((button) => button.text() === 'Cancel render')!.trigger('click')
    expect(wrapper.text()).toContain('Confirm cancel render')

    mocks.getRenderExport.mockResolvedValueOnce(renderJob('render-active', 'running'))
    await wrapper.findAll('button').find((button) => button.text() === 'Refresh render status')!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('Confirm cancel render')

    mocks.getRenderExport.mockResolvedValueOnce(renderJob('render-active', 'succeeded'))
    await wrapper.findAll('button').find((button) => button.text() === 'Refresh render status')!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).not.toContain('Confirm cancel render')
    expect(mocks.cancelRenderExport).not.toHaveBeenCalled()
  })

  it('clears an armed cancellation when the user selects another authoritative render', async () => {
    mocks.listRenderExportHistory.mockResolvedValue({ items: [renderJob('render-old', 'succeeded')], next_cursor: null })
    const wrapper = await mountView()

    await wrapper.findAll('button').find((button) => button.text() === 'Cancel render')!.trigger('click')
    expect(wrapper.text()).toContain('Confirm cancel render')

    await wrapper.find('.history-item').trigger('click')
    expect(wrapper.text()).not.toContain('Confirm cancel render')
    expect(mocks.cancelRenderExport).not.toHaveBeenCalled()
  })
})

function flushPromises() {
  return new Promise((resolve) => window.setTimeout(resolve))
}
