import { describe, expect, it } from 'vitest'

import { latestApprovedScenePlanVersion, narrationDurationMS, sceneEditorNarrationLineageID } from './upstreamBridge'

describe('upstreamBridge', () => {
  it('picks the highest approved scene plan version', () => {
    const version = latestApprovedScenePlanVersion([
      { version: 1, revision: 1, status: 'superseded', source_script_version: 1, source_proposal_version: 1, content_locale: 'en', created_at: '', updated_at: '', approved_at: null },
      { version: 3, revision: 1, status: 'approved', source_script_version: 2, source_proposal_version: 1, content_locale: 'en', created_at: '', updated_at: '', approved_at: null },
      { version: 2, revision: 1, status: 'approved', source_script_version: 2, source_proposal_version: 1, content_locale: 'en', created_at: '', updated_at: '', approved_at: null },
    ])
    expect(version).toBe(3)
  })

  it('derives narration duration from asset metadata', () => {
    expect(narrationDurationMS({ duration_seconds: 1.2346 })).toBe(1235)
    expect(narrationDurationMS({ duration_seconds: 0 })).toBeNull()
    expect(narrationDurationMS(undefined)).toBeNull()
  })

  it('matches the server narration lineage formula', async () => {
    const lineageID = await sceneEditorNarrationLineageID(
      3,
      'intro',
      '11111111-1111-4111-8111-111111111111',
      '22222222-2222-4222-8222-222222222222',
      2_500,
    )
    expect(lineageID).toBe('6e60d6af-6e50-5eda-a0ed-2b01c4de065d')
  })
})
