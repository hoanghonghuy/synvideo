import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import StockMediaPanel from './StockMediaPanel.vue'
import { acquireStockMedia, searchStockMedia, type MediaAsset, type StockMediaResult } from './api'

vi.mock('./api', async (importOriginal) => {
  const original = await importOriginal<typeof import('./api')>()
  return {
    ...original,
    searchStockMedia: vi.fn(),
    acquireStockMedia: vi.fn(),
  }
})

const result: StockMediaResult = {
  provider_key: 'pexels',
  provider_result_id: '123',
  kind: 'image',
  preview_url: 'https://images.example/preview.jpg',
  source_page_url: 'https://www.pexels.com/photo/123/',
  creator_name: 'Ada',
  creator_url: 'https://www.pexels.com/@ada',
  license_summary: 'Pexels License',
  license_reference: 'https://www.pexels.com/license/',
  attribution_text: 'Content by Ada on Pexels',
  acquirable: true,
}

const asset: MediaAsset = {
  id: 'asset-1',
  project_id: 'project-1',
  kind: 'image',
  origin: 'stock',
  mime_type: 'image/jpeg',
  byte_size: 5,
  sha256: 'abc',
  original_filename: 'pexels-123.jpg',
  metadata: {},
  created_at: '2026-09-06T00:00:00Z',
  updated_at: '2026-09-06T00:00:00Z',
}

describe('StockMediaPanel', () => {
  beforeEach(() => {
    vi.mocked(searchStockMedia).mockReset()
    vi.mocked(acquireStockMedia).mockReset()
  })

  it('surfaces provider provenance and only acquires after explicit user action', async () => {
    vi.mocked(searchStockMedia).mockResolvedValue({
      results: [result],
      page: 1,
      per_page: 20,
      has_next_page: false,
    })
    vi.mocked(acquireStockMedia).mockResolvedValue({ asset, reused: false })

    const wrapper = mount(StockMediaPanel, { props: { projectId: 'project-1' } })
    await wrapper.get('[data-testid="stock-query"]').setValue('rainy city')
    await wrapper.get('form').trigger('submit')
    await vi.waitFor(() => expect(searchStockMedia).toHaveBeenCalledTimes(1))

    expect(wrapper.text()).toContain('Ada')
    expect(wrapper.text()).toContain('Pexels License')
    expect(wrapper.find('a[href="https://www.pexels.com/photo/123/"]').exists()).toBe(true)
    expect(acquireStockMedia).not.toHaveBeenCalled()

    await wrapper.get('[data-testid="stock-acquire-123"]').trigger('click')
    await vi.waitFor(() => expect(acquireStockMedia).toHaveBeenCalledTimes(1))
    expect(wrapper.emitted('acquired')?.[0]).toEqual([asset])
  })

  it('shows an explicit provider-unavailable state without silent fallback', async () => {
    const error = Object.assign(new Error('unavailable'), {
      name: 'ApiError',
      code: 'STOCK_MEDIA_PROVIDER_UNAVAILABLE',
      status: 503,
      fields: {},
    })
    vi.mocked(searchStockMedia).mockRejectedValue(error)

    const wrapper = mount(StockMediaPanel, { props: { projectId: 'project-1' } })
    await wrapper.get('[data-testid="stock-query"]').setValue('city')
    await wrapper.get('form').trigger('submit')
    await vi.waitFor(() => expect(searchStockMedia).toHaveBeenCalledTimes(1))

    expect(wrapper.get('[data-testid="stock-search-error"]').text()).toContain('Không thể hoàn tất')
  })
})
