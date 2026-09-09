import type { ScenePlanSummary } from '@/features/scene-plan/api'
import type { ScriptSummary } from '@/features/script/api'

import type { SceneEditorState } from './api'

export type UpstreamBridgePhase =
  | 'aligned'
  | 'script_draft_pending'
  | 'script_approved_needs_scene_plan'
  | 'scene_plan_draft_pending_approval'
  | 'downstream_rebuild_required'
  | 'ready_to_reconcile'

export interface UpstreamBridgeGuidance {
  phase: UpstreamBridgePhase
  compositionScenePlanVersion: number
  compositionScriptVersion: number
  latestApprovedScriptVersion: number | null
  latestApprovedScenePlanVersion: number | null
  pendingScriptDraftVersion: number | null
  pendingScenePlanDraftVersion: number | null
  rebuildNarration: boolean
  rebuildCaptions: boolean
  rebuildAudioMix: boolean
  message: string
}

export interface UpstreamBridgeContext {
  compositionScenePlanVersion: number
  compositionScriptVersion: number
  compositionState: SceneEditorState
  scripts: ScriptSummary[]
  scenePlans: ScenePlanSummary[]
}

export function latestApprovedScriptVersion(scripts: ScriptSummary[]): number | null {
  const approved = scripts.filter((item) => item.status === 'approved')
  if (approved.length === 0) return null
  return approved.reduce((highest, item) => Math.max(highest, item.version), 0)
}

export function latestApprovedScenePlanVersion(scenePlans: ScenePlanSummary[]): number | null {
  const approved = scenePlans.filter((item) => item.status === 'approved')
  if (approved.length === 0) return null
  return approved.reduce((highest, item) => Math.max(highest, item.version), 0)
}

function highestDraftVersion<T extends { version: number; status: string }>(items: T[]): number | null {
  const drafts = items.filter((item) => item.status === 'draft')
  if (drafts.length === 0) return null
  return drafts.reduce((highest, item) => Math.max(highest, item.version), 0)
}

export function evaluateUpstreamBridge(context: UpstreamBridgeContext): UpstreamBridgeGuidance {
  const latestScript = latestApprovedScriptVersion(context.scripts)
  const latestPlan = latestApprovedScenePlanVersion(context.scenePlans)
  const pendingScript = highestDraftVersion(context.scripts)
  const pendingPlan = highestDraftVersion(context.scenePlans)

  const base: UpstreamBridgeGuidance = {
    phase: 'aligned',
    compositionScenePlanVersion: context.compositionScenePlanVersion,
    compositionScriptVersion: context.compositionScriptVersion,
    latestApprovedScriptVersion: latestScript,
    latestApprovedScenePlanVersion: latestPlan,
    pendingScriptDraftVersion: pendingScript,
    pendingScenePlanDraftVersion: pendingPlan,
    rebuildNarration: false,
    rebuildCaptions: false,
    rebuildAudioMix: false,
    message: 'Composition matches the current approved upstream lineage.',
  }

  if (pendingScript !== null && (latestScript === null || pendingScript > latestScript)) {
    return {
      ...base,
      phase: 'script_draft_pending',
      message:
        'An authoritative script draft is still pending approval. Composition is unchanged until you approve the script and complete downstream regeneration.',
    }
  }

  if (latestScript !== null && latestScript > context.compositionScriptVersion) {
    const latestApprovedPlanForScript = context.scenePlans
      .filter((item) => item.status === 'approved' && item.source_script_version === latestScript)
      .reduce<ScenePlanSummary | null>((match, item) => (match === null || item.version > match.version ? item : match), null)

    if (pendingPlan !== null) {
      const pendingPlanSummary = context.scenePlans.find((item) => item.version === pendingPlan)
      if (pendingPlanSummary && pendingPlanSummary.source_script_version === latestScript) {
        return {
          ...base,
          phase: 'scene_plan_draft_pending_approval',
          message:
            'A new Scene Plan draft exists for the approved script. Approve it before rebuilding narration, captions or audio mix.',
        }
      }
    }

    if (latestApprovedPlanForScript === null || latestApprovedPlanForScript.version <= context.compositionScenePlanVersion) {
      return {
        ...base,
        phase: 'script_approved_needs_scene_plan',
        message:
          'The approved script advanced beyond this composition. Regenerate and approve a Scene Plan from the Script/Scene Plan workspaces before reconciling.',
      }
    }
  }

  if (latestPlan !== null && latestPlan > context.compositionScenePlanVersion) {
    const rebuild = {
      rebuildNarration: true,
      rebuildCaptions: true,
      rebuildAudioMix: true,
    }
    if (context.compositionState === 'STALE') {
      return {
        ...base,
        phase: 'ready_to_reconcile',
        ...rebuild,
        message:
          'A newer approved Scene Plan exists. Rebuild narration, captions and audio mix for the new plan version, then reconcile to preserve local presentation edits.',
      }
    }
    return {
      ...base,
      phase: 'downstream_rebuild_required',
      ...rebuild,
      message:
        'Upstream Scene Plan advanced. Rebuild narration, captions and audio mix against the new approved plan before rendering or reconciling.',
    }
  }

  if (context.compositionState === 'STALE') {
    return {
      ...base,
      phase: 'ready_to_reconcile',
      rebuildNarration: true,
      rebuildCaptions: true,
      rebuildAudioMix: true,
      message:
        'Upstream dependencies changed while local presentation edits were preserved. Rebuild affected narration/caption/audio assumptions, then reconcile explicitly.',
    }
  }

  return base
}
