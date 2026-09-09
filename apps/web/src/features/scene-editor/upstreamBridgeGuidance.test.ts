import { describe, expect, it } from 'vitest'

import { evaluateUpstreamBridge } from './upstreamBridgeGuidance'

const script = (version: number, status: 'draft' | 'approved' | 'superseded') => ({
  version,
  revision: 1,
  status,
  source_proposal_version: 1,
  content_locale: 'en',
  created_at: '',
  updated_at: '',
  approved_at: status === 'approved' ? '2026-01-01T00:00:00Z' : null,
})

const plan = (version: number, status: 'draft' | 'approved' | 'superseded', sourceScriptVersion: number) => ({
  version,
  revision: 1,
  status,
  source_script_version: sourceScriptVersion,
  source_proposal_version: 1,
  content_locale: 'en',
  created_at: '',
  updated_at: '',
  approved_at: status === 'approved' ? '2026-01-01T00:00:00Z' : null,
})

describe('evaluateUpstreamBridge', () => {
  it('detects pending script draft before composition mutates', () => {
    const guidance = evaluateUpstreamBridge({
      compositionScenePlanVersion: 2,
      compositionScriptVersion: 2,
      compositionState: 'CURRENT',
      scripts: [script(2, 'approved'), script(3, 'draft')],
      scenePlans: [plan(2, 'approved', 2)],
    })
    expect(guidance.phase).toBe('script_draft_pending')
  })

  it('detects approved script without matching scene plan', () => {
    const guidance = evaluateUpstreamBridge({
      compositionScenePlanVersion: 2,
      compositionScriptVersion: 2,
      compositionState: 'CURRENT',
      scripts: [script(2, 'approved'), script(3, 'approved')],
      scenePlans: [plan(2, 'approved', 2)],
    })
    expect(guidance.phase).toBe('script_approved_needs_scene_plan')
  })

  it('detects pending scene plan approval', () => {
    const guidance = evaluateUpstreamBridge({
      compositionScenePlanVersion: 2,
      compositionScriptVersion: 2,
      compositionState: 'CURRENT',
      scripts: [script(2, 'approved'), script(3, 'approved')],
      scenePlans: [plan(2, 'approved', 2), plan(3, 'draft', 3)],
    })
    expect(guidance.phase).toBe('scene_plan_draft_pending_approval')
  })

  it('marks stale composition ready to reconcile with rebuild guidance', () => {
    const guidance = evaluateUpstreamBridge({
      compositionScenePlanVersion: 2,
      compositionScriptVersion: 3,
      compositionState: 'STALE',
      scripts: [script(3, 'approved')],
      scenePlans: [plan(2, 'approved', 2), plan(3, 'approved', 3)],
    })
    expect(guidance.phase).toBe('ready_to_reconcile')
    expect(guidance.rebuildNarration).toBe(true)
    expect(guidance.rebuildCaptions).toBe(true)
    expect(guidance.rebuildAudioMix).toBe(true)
  })
})
