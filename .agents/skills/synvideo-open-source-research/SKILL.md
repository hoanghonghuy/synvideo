---
name: synvideo-open-source-research
description: Research open-source projects and libraries that can accelerate a SynVideo subsystem while controlling licensing, maintenance, security, and lock-in risk.
---

# SynVideo Open-Source Research

## Use when
A task is about to build a substantial editor, rendering, AI-media, TTS, caption, stock-media, publishing or workflow subsystem.

## Workflow
1. Read the relevant section of `docs/research/OPEN_SOURCE_REFERENCES.md`.
2. Search for current candidates if the recorded references are insufficient or stale.
3. For each serious candidate, verify current license from the repository itself.
4. Classify the candidate:
   - `REUSE-CANDIDATE`: code can potentially be integrated after license/fit review.
   - `LIBRARY`: consume as a dependency rather than copying code.
   - `STUDY-ONLY`: architecture/UX reference only.
   - `REJECT`: unsuitable due to license, security, maintenance or mismatch.
5. Compare fit, integration cost, maintenance health, testability and vendor lock-in.
6. Record the selected approach in the task/ADR before substantial code reuse.

## Rule
Never infer that “public on GitHub” means reusable. License verification is mandatory immediately before reuse because repository licensing can change.
