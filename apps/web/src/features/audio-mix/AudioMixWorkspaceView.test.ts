import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { i18n } from '@/locales'
import AudioMixWorkspaceView from './AudioMixWorkspaceView.vue'

const mocks = vi.hoisted(() => ({
  listMediaAssets: vi.fn(),
  mediaAssetContentURL: vi.fn(() => '/media/audio-1'),
  uploadMediaAsset: vi.fn(),
  createAudioMix: vi.fn(),
  getAudioMix: vi.fn(),
  getAudioMixSnapshot: vi.fn(),
  listAudioMixHistory: vi.fn(),
  rebindAudioMixNarration: vi.fn(),
  updateAudioMix: vi.fn(),
}))

vi.mock('@/features/media/api', () => ({
  listMediaAssets: mocks.listMediaAssets,
  mediaAssetContentURL: mocks.mediaAssetContentURL,
  uploadMediaAsset: mocks.uploadMediaAsset,
}))

vi.mock('./api', () => ({
  createAudioMix: mocks.createAudioMix,
  getAudioMix: mocks.getAudioMix,
  getAudioMixSnapshot: mocks.getAudioMixSnapshot,
  listAudioMixHistory: mocks.listAudioMixHistory,
  rebindAudioMixNarration: mocks.rebindAudioMixNarration,
  updateAudioMix: mocks.updateAudioMix,
}))

const asset = {
  id: 'audio-1',
  project_id: 'project-1',
  kind: 'audio',
  original_filename: 'music.mp3',
}

const mix = {
  id: 'mix-1',
  project_id: 'project-1',
  revision: 1,
  scene_plan_version: 2,
  music_asset_id: 'audio-1',
  narration_duration_ms: 8000,
  state: 'CURRENT',
  config: {
    music_trim_start_ms: 0,
    start_offset_ms: 0,
    loop_policy: 'LOOP_TO_TARGET',
    music_gain_db: -12,
    narration_gain_db: 0,
    ducking: { enabled: true, reduction_db: 9, attack_ms: 120, release_ms: 350 },
  },
  created_at: '2026-09-15T00:00:00Z',
  updated_at: '2026-09-15T00:00:00Z',
}

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => { resolve = done })
  return { promise, resolve }
}

async function mountView() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/projects/:id', component: { template: '<div />' } },
      { path: '/projects/:id/media', component: { template: '<div />' } },
      { path: '/projects/:id/audio-mix', component: AudioMixWorkspaceView },
    ],
  })
  router.push('/projects/project-1/audio-mix')
  await router.isReady()
  const wrapper = mount(AudioMixWorkspaceView, { global: { plugins: [router, i18n] } })
  await flushPromises()
  return wrapper
}

describe('AudioMixWorkspaceView progress semantics', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    i18n.global.locale.value = 'vi'
    mocks.listMediaAssets.mockResolvedValue({ assets: [asset] })
    mocks.getAudioMix.mockResolvedValue(mix)
    mocks.listAudioMixHistory.mockResolvedValue([mix])
  })

  it('announces save progress only while the revision request is pending', async () => {
    const pending = deferred<typeof mix>()
    mocks.updateAudioMix.mockReturnValueOnce(pending.promise)
    const wrapper = await mountView()

    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(wrapper.get('.actions').attributes('aria-busy')).toBe('true')
    const status = wrapper.get('[data-testid="audio-save-status"]')
    expect(status.attributes('role')).toBe('status')
    expect(status.attributes('aria-live')).toBe('polite')
    expect(status.attributes('aria-atomic')).toBe('true')
    expect(status.text()).toContain('Đang lưu')

    pending.resolve({ ...mix, revision: 2 })
    await flushPromises()

    expect(wrapper.find('[data-testid="audio-save-status"]').exists()).toBe(false)
    expect(wrapper.get('.actions').attributes('aria-busy')).toBeUndefined()
    expect(wrapper.text()).toContain('Đã lưu audio mix revision 2')
  })

  it('announces upload progress only while the upload request is pending', async () => {
    const pending = deferred<typeof asset>()
    mocks.uploadMediaAsset.mockReturnValueOnce(pending.promise)
    const wrapper = await mountView()
    const input = wrapper.get('input[type="file"]')
    Object.defineProperty(input.element, 'files', {
      configurable: true,
      value: [new File(['audio'], 'new-track.mp3', { type: 'audio/mpeg' })],
    })

    await input.trigger('change')
    await flushPromises()

    expect(wrapper.get('.panel').attributes('aria-busy')).toBe('true')
    const status = wrapper.get('[data-testid="audio-upload-status"]')
    expect(status.attributes('role')).toBe('status')
    expect(status.attributes('aria-live')).toBe('polite')
    expect(status.attributes('aria-atomic')).toBe('true')
    expect(status.text()).toContain('Đang upload')

    pending.resolve({ ...asset, id: 'audio-2' })
    mocks.listMediaAssets.mockResolvedValueOnce({ assets: [asset, { ...asset, id: 'audio-2' }] })
    await flushPromises()

    expect(wrapper.find('[data-testid="audio-upload-status"]').exists()).toBe(false)
    expect(wrapper.get('.panel').attributes('aria-busy')).toBeUndefined()
    expect(wrapper.text()).toContain('Đã upload nhạc vào Media Library')
  })
})
