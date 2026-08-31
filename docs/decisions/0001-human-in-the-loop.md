# ADR 0001 — Human-in-the-loop generation

Status: Accepted

## Decision
SynVideo must not treat prompt-to-final-video as the primary workflow. Major creative stages expose explicit review/revision checkpoints.

Baseline flow:
Raw intent/source → Creative Brief → AI Proposal → approval/revision → Script → approval/revision → Scene plan → media/audio/caption generation → editable draft → render/publish.

## Why
Creators should retain editorial control, expensive generation should not run from misunderstood intent, and long-form video requires progressive review to avoid wasting time/cost.

## Consequences
- Stage output must be persisted where practical.
- Regeneration should operate at the smallest useful unit (section/scene/asset), not always restart the project.
- UI and job orchestration need explicit stage state.
