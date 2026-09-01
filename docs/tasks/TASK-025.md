# TASK-025 — Provider-neutral visual generation foundation

Status: BACKLOG
Milestone: F1 Creative Workflow
Planned wave: WAVE-F1-I candidate
Branch when activated: `feature/TASK-025-visual-provider-foundation`
Base: `develop`
Depends on: TASK-005 provider capability foundation accepted.

## Goal
Establish production-grade provider-neutral image and asynchronous video generation ports/registry bindings/fakes before live adapters or paid per-scene jobs are introduced.

## Frozen contract
`docs/contracts/VISUAL_GENERATION_PROVIDER_V1.md`.

## Primary ownership when activated
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

## TDD
Implement every gate in `VISUAL_GENERATION_PROVIDER_V1`, with special regressions for multi-capability registry models, streaming close/cancel, opaque video lifecycle and legacy text behavior.

## Activation gate
This task has no unresolved product dependency and uses an isolated primary write surface, but stays BACKLOG while WAVE-F1-H already fills all three configured implementation slots.

As soon as a worktree slot is released, PM/TL may promote TASK-025 to READY even if other H tasks are still under review, provided no active task has started modifying core `providers/**`.

Do not self-mark READY/DONE or self-merge.