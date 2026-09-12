import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import * as publishingApi from '@/api/publishing'
import ChannelHubView from './ChannelHubView.vue'

const connection: publishingApi.ChannelConnection = {
  id: 'connection-1',
  provider: 'youtube',
  remote_channel_id: 'channel-1',
  display_name: 'SynVideo Channel',
  state: 'connected',
  capabilities: { can_upload: true, can_publish: false, can_schedule: false },
  created_at: '2026-09-12T00:00:00Z',
  updated_at: '2026-09-12T00:00:00Z',
}

const artifact: publishingApi.PublishArtifactSummary = {
  id: 'artifact-1',
  byte_size: 1000,
  duration_ms: 30000,
  width: 1920,
  height: 1080,
  created_at: '2026-09-12T00:00:00Z',
}

function publishAttempt(id: string, state: publishingApi.PublishState, uploadedBytes = 100): publishingApi.PublishAttempt {
  return {
    id,
    project_id: 'project-1',
    connection_id: connection.id,
    render_artifact_id: artifact.id,
    request_id: `request-${id}`,
    provider: 'youtube',
    state,
    uploaded_bytes: uploadedBytes,
    title: `Video ${id}`,
    created_at: '2026-09-12T00:00:00Z',
    updated_at: '2026-09-12T00:00:00Z',
  }
}

describe('ChannelHubView live upload progress', () => {
  let router: ReturnType<typeof createRouter>

  beforeEach(async () => {
    vi.useFakeTimers()
    vi.restoreAllMocks()
    router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/projects/:id', component: { template: '<div />' } },
        { path: '/projects/:id/publishing', component: ChannelHubView },
      ],
    })
    router.push('/projects/project-1/publishing')
    await router.isReady()

    vi.spyOn(publishingApi, 'listPublishingConnections').mockResolvedValue([connection])
    vi.spyOn(publishingApi, 'listPublishArtifacts').mockResolvedValue([artifact])
    vi.spyOn(publishingApi, 'startYouTubeOAuth').mockResolvedValue('https://accounts.example/authorize')
    vi.spyOn(publishingApi, 'createPublishAttempt')
    vi.spyOn(publishingApi, 'retryPublishAttempt')
    vi.spyOn(publishingApi, 'reconcilePublishAttempt')
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('refreshes a selected active upload and stops after it leaves local execution', async () => {
    const uploading = publishAttempt('attempt-1', 'uploading', 250)
    const accepted = publishAttempt('attempt-1', 'upload_accepted', 1000)
    vi.spyOn(publishingApi, 'listPublishAttempts').mockResolvedValue([uploading])
    const getSpy = vi.spyOn(publishingApi, 'getPublishAttempt').mockResolvedValue(accepted)
    const retrySpy = vi.spyOn(publishingApi, 'retryPublishAttempt')
    const reconcileSpy = vi.spyOn(publishingApi, 'reconcilePublishAttempt')

    const wrapper = mount(ChannelHubView, { global: { plugins: [router] } })
    await flushPromises()
    await wrapper.get('[data-attempt-id="attempt-1"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="live-progress-status"]').text()).toContain('refreshes automatically')

    await vi.advanceTimersByTimeAsync(4000)
    await flushPromises()

    expect(getSpy).toHaveBeenCalledTimes(1)
    expect(getSpy).toHaveBeenCalledWith('project-1', 'attempt-1')
    expect(wrapper.text()).toContain('upload accepted')
    expect(wrapper.find('[data-testid="live-progress-status"]').exists()).toBe(false)
    expect(retrySpy).not.toHaveBeenCalled()
    expect(reconcileSpy).not.toHaveBeenCalled()

    await vi.advanceTimersByTimeAsync(12000)
    await flushPromises()
    expect(getSpy).toHaveBeenCalledTimes(1)

    wrapper.unmount()
  })

  it('keeps routine polling copy stable and avoids a nested live-region status', async () => {
    const uploading = publishAttempt('attempt-quiet', 'uploading', 250)
    const progressed = publishAttempt('attempt-quiet', 'uploading', 500)
    vi.spyOn(publishingApi, 'listPublishAttempts').mockResolvedValue([uploading])
    const getSpy = vi.spyOn(publishingApi, 'getPublishAttempt').mockResolvedValue(progressed)

    const wrapper = mount(ChannelHubView, { global: { plugins: [router] } })
    await flushPromises()
    await wrapper.get('[data-attempt-id="attempt-quiet"]').trigger('click')
    await flushPromises()

    const initialStatus = wrapper.get('[data-testid="live-progress-status"]')
    expect(initialStatus.text()).toContain('refreshes automatically')
    expect(initialStatus.attributes('role')).toBeUndefined()

    await vi.advanceTimersByTimeAsync(4000)
    await flushPromises()

    expect(getSpy).toHaveBeenCalledTimes(1)
    const refreshedStatus = wrapper.get('[data-testid="live-progress-status"]')
    expect(refreshedStatus.text()).toBe(initialStatus.text())
    expect(refreshedStatus.text()).not.toContain('Refreshing saved upload progress')
    expect(refreshedStatus.attributes('role')).toBeUndefined()
    expect(wrapper.text()).toContain('500 bytes')

    wrapper.unmount()
  })

  it('cancels scheduled refresh on unmount', async () => {
    vi.spyOn(publishingApi, 'listPublishAttempts').mockResolvedValue([
      publishAttempt('attempt-2', 'queued', 0),
    ])
    const getSpy = vi.spyOn(publishingApi, 'getPublishAttempt').mockResolvedValue(
      publishAttempt('attempt-2', 'uploading', 100),
    )

    const wrapper = mount(ChannelHubView, { global: { plugins: [router] } })
    await flushPromises()
    await wrapper.get('[data-attempt-id="attempt-2"]').trigger('click')
    await flushPromises()
    wrapper.unmount()

    await vi.advanceTimersByTimeAsync(8000)
    await flushPromises()
    expect(getSpy).not.toHaveBeenCalled()
  })

  it('pauses after a transient refresh failure without issuing provider commands', async () => {
    vi.spyOn(publishingApi, 'listPublishAttempts').mockResolvedValue([
      publishAttempt('attempt-3', 'uploading', 400),
    ])
    const getSpy = vi.spyOn(publishingApi, 'getPublishAttempt').mockRejectedValue(new Error('temporary network failure'))
    const retrySpy = vi.spyOn(publishingApi, 'retryPublishAttempt')
    const reconcileSpy = vi.spyOn(publishingApi, 'reconcilePublishAttempt')

    const wrapper = mount(ChannelHubView, { global: { plugins: [router] } })
    await flushPromises()
    await wrapper.get('[data-attempt-id="attempt-3"]').trigger('click')
    await flushPromises()

    await vi.advanceTimersByTimeAsync(4000)
    await flushPromises()

    expect(getSpy).toHaveBeenCalledTimes(1)
    expect(wrapper.get('[data-testid="live-progress-status"]').text()).toContain('Live progress refresh paused')
    expect(wrapper.text()).toContain('400 bytes')
    expect(retrySpy).not.toHaveBeenCalled()
    expect(reconcileSpy).not.toHaveBeenCalled()

    await vi.advanceTimersByTimeAsync(12000)
    await flushPromises()
    expect(getSpy).toHaveBeenCalledTimes(1)

    wrapper.unmount()
  })
})
