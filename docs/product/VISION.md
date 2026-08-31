# SynVideo Product Vision

SynVideo is an **AI-assisted video production workspace** for individual creators. It helps users turn human intent and source material into polished short-form or long-form video while keeping the creator in control through explicit review and editing checkpoints.

It targets TikTok, Reels and Shorts **and** long-form YouTube/general video. It is not a short-video-only generator and not a prompt-to-MP4 demo.

## Primary users
Individual creators and small creator workflows that want AI assistance without needing to master prompt engineering or professional editing software.

## Core promise
A creator can provide a rough idea, description, text, images, existing media or reference material. SynVideo helps refine that raw input into a stronger creative proposal, lets the creator approve/edit it, then progressively builds script, scenes, media, voice, captions, music and final video.

## Product principles
1. **Human intent first.** The product accepts natural descriptions, not only technical prompts.
2. **Human-in-the-loop.** AI proposes; the creator can approve, revise or regenerate at meaningful checkpoints.
3. **Complete workflows, not demos.** A capability marked complete includes validation, persistence, error/empty/loading states, recovery and appropriate tests.
4. **Short and long video are first-class.** Duration, pacing, aspect ratio and editing behavior must not assume only 30–60 second content.
5. **Scene is the initial creative unit.** Scene-first editing must be comprehensive enough for real use and designed to evolve toward richer timeline editing.
6. **Replaceable providers.** AI vendors and media providers are adapters, not product-domain concepts.
7. **BYOK first.** Users can configure supported provider credentials; future managed credits/billing must not require redesigning domain semantics.
8. **Creator workflow extends to distribution.** Rendering is not the end state; connected-channel publishing and channel management are product goals.
9. **i18n from day one.** Initial UI may prioritize Vietnamese, but user-facing copy must live behind localization infrastructure so English can be added without retrofit.
10. **Safe iteration.** Expensive generation is granular, retryable and versioned where practical; one failed scene must not destroy good work.

## North-star workflow
Raw intent/source → Creative Brief → AI Proposal → Human approval → Script → Human approval → Scene plan → Assets/audio/captions → Editable draft → Review/regenerate → Render → Publish/manage.

## Non-goals for early milestones
- Rebuilding every professional NLE feature before the AI-assisted workflow is useful.
- Supporting every model/provider through provider-specific domain code.
- Calling placeholder UI or mocked workflows “complete”.
