import { getAudioMix } from '@/features/audio-mix/api'
import { getCaptions } from '@/features/captions/api'
import { listSceneMediaBindings } from '@/features/media/api'
import { listSceneNarrations } from '@/features/scene-narration/api'
import { getScenePlan, listScenePlans } from '@/features/scene-plan/api'
import { forkScript, listScripts } from '@/features/script/api'

import { evaluateUpstreamBridge, latestApprovedScenePlanVersion, type UpstreamBridgeGuidance } from './upstreamBridgeGuidance'

import type {
  SceneEditorAudioMixRef,
  SceneEditorCandidate,
  SceneEditorCaptionRef,
  SceneEditorNarrationRef,
  SceneEditorState,
  SceneEditorVisualRef,
} from './api'

const OID_NAMESPACE = '6ba7b812-9dad-11d1-80b4-00c04fd430c8'

export async function resolveSourceScriptVersion(projectID: string, scenePlanVersion: number): Promise<number> {
  const plan = await getScenePlan(projectID, scenePlanVersion)
  return plan.source_script_version
}

export { latestApprovedScenePlanVersion }

export async function loadUpstreamBridgeGuidance(
  projectID: string,
  composition: { scene_plan_version: number; state: SceneEditorState },
): Promise<UpstreamBridgeGuidance> {
  const [scripts, scenePlans, plan] = await Promise.all([
    listScripts(projectID),
    listScenePlans(projectID),
    getScenePlan(projectID, composition.scene_plan_version),
  ])
  return evaluateUpstreamBridge({
    compositionScenePlanVersion: composition.scene_plan_version,
    compositionScriptVersion: plan.source_script_version,
    compositionState: composition.state,
    scripts,
    scenePlans,
  })
}

export async function forkApprovedScriptForComposition(projectID: string, scenePlanVersion: number) {
  const sourceVersion = await resolveSourceScriptVersion(projectID, scenePlanVersion)
  return forkScript(projectID, sourceVersion)
}

export function scenePlanSceneKeys(plan: { scenes: Array<{ key: string }> }): string[] {
  return plan.scenes.map((scene) => scene.key)
}

export async function buildReconcileCandidate(
  projectID: string,
  scenePlanVersion: number,
): Promise<SceneEditorCandidate> {
  const plan = await getScenePlan(projectID, scenePlanVersion)
  const sceneKeys = scenePlanSceneKeys(plan)
  const [mediaEntries, narrationEntries, audioMix] = await Promise.all([
    listSceneMediaBindings(projectID, scenePlanVersion),
    listSceneNarrations(projectID, scenePlanVersion),
    getAudioMix(projectID).catch(() => null),
  ])

  const scenes = await Promise.all(sceneKeys.map(async (sceneKey) => {
    const visual = toVisualRef(mediaEntries.find((entry) => entry.scene_key === sceneKey))
    const narration = await toNarrationRef(projectID, scenePlanVersion, sceneKey, narrationEntries)
    const caption = await toCaptionRef(projectID, scenePlanVersion, sceneKey, narration)
    return { scene_key: sceneKey, visual, narration, caption }
  }))

  const candidate: SceneEditorCandidate = {
    scene_plan_version: scenePlanVersion,
    scenes,
  }

  if (audioMix && audioMix.scene_plan_version === scenePlanVersion) {
    candidate.audio_mix = {
      document_id: audioMix.id,
      revision: audioMix.revision,
      music_asset_id: audioMix.music_asset_id,
      narration_lineage_id: audioMix.narration_lineage_id,
    } satisfies SceneEditorAudioMixRef
  }

  return candidate
}

function toVisualRef(entry?: { binding?: { id?: string; asset_id?: string } }): SceneEditorVisualRef | undefined {
  const bindingID = entry?.binding?.id
  const assetID = entry?.binding?.asset_id
  if (!bindingID || !assetID) return undefined
  return { binding_id: bindingID, asset_id: assetID }
}

async function toNarrationRef(
  _projectID: string,
  scenePlanVersion: number,
  sceneKey: string,
  entries: Awaited<ReturnType<typeof listSceneNarrations>>,
): Promise<SceneEditorNarrationRef | undefined> {
  const entry = entries.find((item) => item.scene_key === sceneKey)
  const bindingID = entry?.binding?.id
  const assetID = entry?.binding?.asset_id ?? entry?.asset?.id
  if (!bindingID || !assetID) return undefined

  const durationMS = narrationDurationMS(entry?.asset?.metadata)
  if (durationMS === null) return undefined

  return {
    asset_id: assetID,
    binding_id: bindingID,
    duration_ms: durationMS,
    lineage_id: await sceneEditorNarrationLineageID(scenePlanVersion, sceneKey, bindingID, assetID, durationMS),
  }
}

async function toCaptionRef(
  projectID: string,
  scenePlanVersion: number,
  sceneKey: string,
  narration?: SceneEditorNarrationRef,
): Promise<SceneEditorCaptionRef | undefined> {
  if (!narration) return undefined

  try {
    const caption = await getCaptions(projectID, scenePlanVersion, sceneKey)
    const lastEndMS = caption.segments.reduce((max, segment) => Math.max(max, segment.end_ms), 0)
    if (lastEndMS <= 0) return undefined
    return {
      document_id: caption.id,
      revision: caption.revision,
      lineage_id: narration.lineage_id,
      last_end_ms: lastEndMS,
    }
  } catch {
    return undefined
  }
}

export function narrationDurationMS(metadata: Record<string, unknown> | undefined): number | null {
  const raw = metadata?.duration_seconds
  if (typeof raw !== 'number' || !Number.isFinite(raw) || raw <= 0) return null
  const durationMS = Math.round(raw * 1000)
  return durationMS > 0 ? durationMS : null
}

export async function sceneEditorNarrationLineageID(
  planVersion: number,
  sceneKey: string,
  bindingID: string,
  assetID: string,
  durationMS: number,
): Promise<string> {
  const value = `scene-editor-narration-v1|plan:${planVersion}|scene:${sceneKey}|binding:${bindingID}|asset:${assetID}|duration_ms:${durationMS}`
  return sha1UUID(OID_NAMESPACE, value)
}

async function sha1UUID(namespace: string, value: string): Promise<string> {
  const namespaceBytes = uuidToBytes(namespace)
  const data = new Uint8Array(namespaceBytes.length + value.length)
  data.set(namespaceBytes)
  data.set(new TextEncoder().encode(value), namespaceBytes.length)

  const digest = await crypto.subtle.digest('SHA-1', data)
  const bytes = new Uint8Array(digest).slice(0, 16)
  bytes[6] = (bytes[6]! & 0x0f) | 0x50
  bytes[8] = (bytes[8]! & 0x3f) | 0x80
  return formatUUID(bytes)
}

function uuidToBytes(uuid: string): Uint8Array {
  const hex = uuid.replace(/-/g, '')
  const bytes = new Uint8Array(16)
  for (let index = 0; index < 16; index += 1) {
    bytes[index] = Number.parseInt(hex.slice(index * 2, index * 2 + 2), 16)
  }
  return bytes
}

function formatUUID(bytes: Uint8Array): string {
  const hex = Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('')
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
}
