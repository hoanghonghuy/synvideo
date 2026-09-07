<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink, useRoute } from 'vue-router'

import { ApiError } from '@/api/projects'
import {
  createSceneEditorSnapshot,
  duplicateScene,
  getSceneEditor,
  removeScene,
  reorderScene,
  type SceneEditorScene,
  type SceneEditorView,
} from './api'

const route = useRoute()
const projectID = computed(() => String(route.params.id ?? ''))
const composition = ref<SceneEditorView | null>(null)
const loading = ref(true)
const acting = ref(false)
const error = ref('')
const notice = ref('')

const snapshotBlocked = computed(() => composition.value?.state !== 'CURRENT')

onMounted(() => {
  void load()
})

async function load() {
  if (!projectID.value) return
  loading.value = true
  error.value = ''
  try {
    composition.value = await getSceneEditor(projectID.value)
  } catch (cause) {
    if (cause instanceof ApiError && cause.status === 404) {
      composition.value = null
      notice.value = 'Scene composition has not been initialized from authoritative project sources yet.'
    } else {
      error.value = messageFor(cause)
    }
  } finally {
    loading.value = false
  }
}

async function move(scene: SceneEditorScene, delta: -1 | 1) {
  if (!composition.value || acting.value) return
  const from = composition.value.scenes.findIndex((item) => item.id === scene.id)
  const to = from + delta
  if (from < 0 || to < 0 || to >= composition.value.scenes.length) return
  await act(() => reorderScene(projectID.value, scene.id, composition.value!.revision, to), 'Scene order saved.')
}

async function duplicate(scene: SceneEditorScene) {
  if (!composition.value || acting.value) return
  await act(() => duplicateScene(projectID.value, scene.id, composition.value!.revision), 'Scene duplicated.')
}

async function remove(scene: SceneEditorScene) {
  if (!composition.value || acting.value || composition.value.scenes.length <= 1) return
  await act(() => removeScene(projectID.value, scene.id, composition.value!.revision), 'Scene removed.')
}

async function createSnapshot() {
  if (!composition.value || snapshotBlocked.value || acting.value) return
  acting.value = true
  error.value = ''
  notice.value = ''
  try {
    const snapshot = await createSceneEditorSnapshot(projectID.value, composition.value.revision)
    notice.value = `Immutable render snapshot ready: ${snapshot.digest.slice(0, 12)}… (revision ${snapshot.revision}).`
  } catch (cause) {
    error.value = messageFor(cause)
  } finally {
    acting.value = false
  }
}

async function act(operation: () => Promise<SceneEditorView>, success: string) {
  acting.value = true
  error.value = ''
  notice.value = ''
  try {
    composition.value = await operation()
    notice.value = success
  } catch (cause) {
    error.value = messageFor(cause)
    if (cause instanceof ApiError && cause.status === 409) {
      await load()
    }
  } finally {
    acting.value = false
  }
}

function messageFor(cause: unknown): string {
  if (cause instanceof ApiError) return `${cause.code}: ${cause.message}`
  return cause instanceof Error ? cause.message : 'Scene editor request failed.'
}

function seconds(ms: number): string {
  return `${(ms / 1000).toFixed(ms % 1000 === 0 ? 0 : 1)}s`
}
</script>

<template>
  <main class="scene-editor-page">
    <header class="page-header">
      <div>
        <p class="eyebrow">Creator composition</p>
        <h1>Scene Editor</h1>
        <p>Review deterministic scene order, source lineage and render readiness without hiding stale dependencies.</p>
      </div>
      <RouterLink :to="`/projects/${projectID}`">Back to project</RouterLink>
    </header>

    <p v-if="error" class="notice error" role="alert">{{ error }}</p>
    <p v-if="notice" class="notice" role="status">{{ notice }}</p>
    <p v-if="loading" role="status">Loading scene composition…</p>

    <template v-else-if="composition">
      <section class="status-panel" :data-state="composition.state" aria-live="polite">
        <div>
          <strong>{{ composition.state }}</strong>
          <span>Revision {{ composition.revision }} · Scene plan v{{ composition.scene_plan_version }}</span>
        </div>
        <p v-if="composition.state === 'STALE'">Upstream source lineage changed. Reconcile before creating a render snapshot.</p>
        <p v-else-if="composition.state === 'BROKEN'">One or more exact upstream dependencies are unavailable. Rendering is blocked.</p>
        <p v-else>All tracked dependencies match this composition revision.</p>
      </section>

      <section aria-labelledby="scene-list-heading">
        <div class="section-heading">
          <div>
            <h2 id="scene-list-heading">Scenes</h2>
            <p>Keyboard-accessible controls preserve stable local scene identity while changing order.</p>
          </div>
          <button type="button" :disabled="snapshotBlocked || acting" @click="createSnapshot">
            Create render snapshot
          </button>
        </div>

        <ol class="scene-list">
          <li v-for="(scene, index) in composition.scenes" :key="scene.id" class="scene-card">
            <header>
              <div>
                <span class="scene-index">Scene {{ index + 1 }}</span>
                <h3>{{ scene.scene_key }}</h3>
              </div>
              <span>{{ seconds(scene.duration_ms) }}</span>
            </header>

            <dl class="lineage-grid">
              <div>
                <dt>Visual</dt>
                <dd>{{ scene.visual ? scene.visual.asset_id : 'No visual bound' }}</dd>
              </div>
              <div>
                <dt>Narration</dt>
                <dd>{{ scene.narration ? scene.narration.lineage_id : 'No narration bound' }}</dd>
              </div>
              <div>
                <dt>Captions</dt>
                <dd>{{ scene.caption ? `${scene.caption.document_id} r${scene.caption.revision}` : 'No captions bound' }}</dd>
              </div>
              <div>
                <dt>Transition</dt>
                <dd>{{ scene.transition_out.kind }} · {{ scene.transition_out.duration_ms }}ms</dd>
              </div>
            </dl>

            <div class="scene-actions" :aria-label="`Actions for ${scene.scene_key}`">
              <button type="button" :disabled="acting || index === 0" @click="move(scene, -1)">Move up</button>
              <button type="button" :disabled="acting || index === composition.scenes.length - 1" @click="move(scene, 1)">Move down</button>
              <button type="button" :disabled="acting" @click="duplicate(scene)">Duplicate</button>
              <button type="button" :disabled="acting || composition.scenes.length <= 1" @click="remove(scene)">Remove</button>
            </div>
          </li>
        </ol>
      </section>
    </template>
  </main>
</template>

<style scoped>
.scene-editor-page { display: grid; gap: 1.5rem; max-width: 1100px; margin: 0 auto; padding: 2rem 1rem 4rem; }
.page-header, .section-heading, .scene-card header, .status-panel > div { display: flex; gap: 1rem; justify-content: space-between; align-items: flex-start; }
.eyebrow, .scene-index { text-transform: uppercase; letter-spacing: .08em; font-size: .75rem; font-weight: 700; }
.notice, .status-panel, .scene-card { border: 1px solid currentColor; border-radius: .75rem; padding: 1rem; }
.notice.error, .status-panel[data-state='BROKEN'] { border-width: 2px; }
.status-panel { display: grid; gap: .5rem; }
.status-panel strong { margin-right: .75rem; }
.scene-list { display: grid; gap: 1rem; padding: 0; list-style: none; }
.scene-card { display: grid; gap: 1rem; }
.scene-card h3 { margin: .2rem 0 0; }
.lineage-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(190px, 1fr)); gap: .75rem; margin: 0; }
.lineage-grid div { min-width: 0; }
.lineage-grid dt { font-weight: 700; }
.lineage-grid dd { margin: .25rem 0 0; overflow-wrap: anywhere; }
.scene-actions { display: flex; gap: .5rem; flex-wrap: wrap; }
button { min-height: 2.75rem; padding: .55rem .8rem; }
button:focus-visible, a:focus-visible { outline: 3px solid currentColor; outline-offset: 3px; }
@media (max-width: 640px) { .page-header, .section-heading, .scene-card header { flex-direction: column; } }
</style>
