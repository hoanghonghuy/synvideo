# TASK-025 — Provider-neutral visual generation foundation

Status: READY
Milestone: F1 Creative Workflow
Wave: WAVE-F1-I early slot
Branch: `feature/TASK-025-visual-provider-foundation`
Base: `develop`
Issue: #43
Depends on: TASK-005 provider capability foundation accepted.

## Goal
Establish production-grade provider-neutral image and asynchronous video generation ports/registry bindings/fakes before live adapters or paid per-scene jobs are introduced.

## Frozen contract
`docs/contracts/VISUAL_GENERATION_PROVIDER_V1.md`.

## Primary ownership
- `apps/api/internal/providers/**` visual interfaces/types/registry extension;
- deterministic visual provider fakes/tests only.

No persistence, jobs, HTTP, runtime composition, provider settings, Media Asset code or frontend.

## Required capability
- ImageGenerator port with bounded provider-neutral prompt/aspect/output/reference semantics;
- VideoGenerator explicit Start/Poll/OpenResult lifecycle with opaque operation ID;
- streaming generated binary abstraction;
- safe errors/context propagation;
- backward-compatible registry bindings/resolvers for text/image/video models;
- deterministic fakes.

## Critical architecture gate
Do not define one universal sync `Generate() -> []byte` API. Video external operation IDs must remain first-class so future durable orchestration can persist and resume paid operations after worker crash instead of blindly resubmitting.

Generated provider output is not yet a durable SynVideo Media Asset in this task. Live adapters, owner runtime, jobs and ingestion are follow-on capabilities.

## TDD
Implement every gate in `VISUAL_GENERATION_PROVIDER_V1`, with special regressions for:
- multi-capability registry models;
- legacy text-generation registry behavior;
- image streaming close/context/error semantics;
- opaque async video Start/Poll/OpenResult lifecycle;
- invalid metadata/capability registration;
- deep-copy/immutability and deterministic list order;
- no provider-specific SDK/schema leakage.

## Isolation / parallel safety
TASK-020 is merged, releasing one implementation slot. TASK-018 remains on backend job/httpserver/runtime surfaces and TASK-019 remains on frontend Script/router/locale surfaces. Neither owns `apps/api/internal/providers/**` core visual capability work.

Do not modify `main.go`, `httpserver/**`, `jobs/**`, provider settings/BYOK persistence, media assets/storage, Scene Plan/Script feature packages, frontend, live provider adapters or migrations.

## Worktree / claim
Before work, confirm remote `feature/TASK-025-visual-provider-foundation` is still absent. Atomically create that remote ref from latest `origin/develop`, then use a dedicated TASK-025 worktree. Shared/control checkout remains on `develop`.

Do not self-mark DONE or self-merge.