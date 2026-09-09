import { describe, expect, it } from 'vitest'

import type { RenderExportJob } from './api'
import {
  isRenderExportCancellable,
  isRenderExportRetryable,
  isRenderExportTerminal,
  persistRenderJobID,
  renderExportStorageKey,
  restoreRenderJobID,
} from './renderExportState'

function job(state: RenderExportJob['state']): RenderExportJob {
  return {
    id: 'job-1',
    state,
    attempt: 1,
    max_attempts: 2,
    snapshot_digest: 'digest',
    profile_id: 'local_software_mp4_v1',
    created_at: '2026-09-08T00:00:00Z',
    updated_at: '2026-09-08T00:00:00Z',
  }
}

class MemoryStorage {
  private readonly values = new Map<string, string>()

  getItem(key: string): string | null {
    return this.values.get(key) ?? null
  }

  setItem(key: string, value: string): void {
    this.values.set(key, value)
  }

  removeItem(key: string): void {
    this.values.delete(key)
  }
}

describe('Scene Editor render export UI state', () => {
  it('persists one project-scoped job identity across refresh', () => {
    const storage = new MemoryStorage()
    persistRenderJobID(storage, 'project-1', 'job-1')
    expect(storage.getItem(renderExportStorageKey('project-1'))).toBe('job-1')
    expect(restoreRenderJobID(storage, 'project-1')).toBe('job-1')
    expect(restoreRenderJobID(storage, 'project-2')).toBeNull()
  })

  it('clears persisted identity explicitly', () => {
    const storage = new MemoryStorage()
    persistRenderJobID(storage, 'project-1', 'job-1')
    persistRenderJobID(storage, 'project-1', null)
    expect(restoreRenderJobID(storage, 'project-1')).toBeNull()
  })

  it('polls only non-terminal durable states', () => {
    expect(isRenderExportTerminal(job('queued'))).toBe(false)
    expect(isRenderExportTerminal(job('running'))).toBe(false)
    expect(isRenderExportTerminal(job('succeeded'))).toBe(true)
    expect(isRenderExportTerminal(job('failed'))).toBe(true)
    expect(isRenderExportTerminal(job('cancelled'))).toBe(true)
  })

  it('enables cancel only for queued or running jobs without pending cancellation', () => {
    expect(isRenderExportCancellable(job('queued'))).toBe(true)
    expect(isRenderExportCancellable(job('running'))).toBe(true)
    expect(isRenderExportCancellable({ ...job('running'), cancellation_pending: true })).toBe(false)
    expect(isRenderExportCancellable(job('failed'))).toBe(false)
    expect(isRenderExportCancellable(job('cancelled'))).toBe(false)
  })

  it('enables retry for every terminal render job state', () => {
    expect(isRenderExportRetryable(job('succeeded'))).toBe(true)
    expect(isRenderExportRetryable(job('failed'))).toBe(true)
    expect(isRenderExportRetryable(job('cancelled'))).toBe(true)
    expect(isRenderExportRetryable(job('queued'))).toBe(false)
    expect(isRenderExportRetryable(job('running'))).toBe(false)
  })
})
