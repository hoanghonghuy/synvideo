# Scene Narration & Voice V1 Contract

Status: FROZEN for planned `TASK-031`.

## Product outcome
For each scene in an approved Scene Plan, a creator can select an enabled TTS model/voice, synthesize the exact narration, recover from refresh/failure without duplicate paid work, obtain a durable audio MediaAsset with measured timing, preview it, regenerate alternatives and explicitly make one narration asset current while preserving history.

This is one end-to-end product capability. Do not split chunking, persistence, scene audio binding, API and creator UI into unrelated micro-tasks.

## Source snapshot
A narration generation request snapshots owner/project identity, approved Scene Plan version, exact scene key, exact narration text, content locale, selected internal TTS model/voice IDs, accepted provider-neutral output options and client request ID. Execution must not silently read a newer Scene Plan after queueing.

## Exact narration invariant
Every Unicode character of accepted source narration must be represented in synthesis input in original order. No layer may silently trim, summarize, rewrite or truncate narration.

## Chunking
If one provider call cannot accept the full narration, orchestration owns deterministic chunking and synthesizes every chunk. Preferred boundaries: paragraph/line, sentence, safe whitespace, then rune-safe hard boundary. Re-concatenated chunk text must equal the original narration exactly.

## Durable lifecycle
Use existing durable jobs with explicit recovery boundaries around request creation, chunk synthesis, durable chunk outcome, ordered stitch/finalization, duration measurement, final MediaAsset ingestion and optional scene narration binding. Retry after later-stage crash must not automatically re-synthesize already-durable paid chunk results.

## Final asset
Successful generation produces one creator-visible `MediaAsset` kind `audio`, origin `generated_audio`, canonical MIME, safe job/internal model/voice lineage and measured duration. No credentials/base URLs/external IDs/raw upstream/temporary URLs. Intermediate chunks remain internal.

## Scene narration binding/history
Introduce scene-audio/narration binding distinct from primary visual binding. For approved plan version + scene key: at most one current narration audio; owner/project/audio constraints; same-asset assignment idempotent; replacement atomic; append-only history; referenced audio deletion remains safe.

## Timing
Final duration is measured from produced audio and tied to the exact narration asset/version. Caption work may consume this later but is non-scope here.

## Creator flow
Choose enabled model/voice, optionally preview where truly supported, submit with stable request ID, observe durable status, refresh/resume same logical generation, preview final audio/duration, keep as alternative or assign current, regenerate with new request ID while preserving history.

## Failure truthfulness
Distinguish provider/model/voice unavailable, stale source, invalid narration, synthesis failure, stitch/measurement/storage failure, asset success but binding failure, and transient status/load failure. Polling failure must not create another synthesis job.

## Non-scope
Voice cloning, transcription, captions, music/mixing, project master audio, generated video, waveform editor and billing ledger.

## Required TDD gates
1. exact source snapshot;
2. deterministic chunk recomposition;
3. no truncation/all chunks synthesized;
4. preflight before paid execution;
5. request-id replay/conflict;
6. duplicate worker delivery;
7. durable chunk recovery;
8. ordered stitch/duration;
9. final MediaAsset/safe provenance;
10. scene narration binding constraints;
11. replacement/history;
12. asset-success/binding-failure recovery;
13. refresh/same-job UI recovery;
14. real PostgreSQL/object-storage/audio integration;
15. full CI.
