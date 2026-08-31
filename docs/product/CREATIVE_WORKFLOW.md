# Creative Workflow

## Goal
Turn imperfect human intent/source material into an editable video plan before expensive media generation occurs.

## Supported source directions
A project can begin from one or more of:
- plain-language topic/idea/goal;
- a detailed creative description;
- existing script/text/article/document;
- uploaded image(s);
- uploaded video/audio;
- existing creator media;
- reference material/URLs where legally/technically supported.

## Stage 1 — Source intake
Capture source material plus explicit creator intent:
- target audience;
- objective;
- desired video type/style;
- platform(s);
- target duration/range;
- tone/language;
- aspect ratio or auto-recommendation;
- call to action;
- must-include/must-avoid facts.

Do not require all fields. AI can propose missing creative choices, but must label them as proposals rather than creator facts.

## Stage 2 — Creative Brief
Normalize the input into durable structured intent, including provenance between user-provided facts and AI-inferred recommendations.

## Stage 3 — AI Proposal
AI creates an editorial proposal, not final media. Typical output:
- title/concept options;
- intended audience/objective summary;
- recommended format and estimated duration;
- hook(s);
- narrative angle;
- structure/sections;
- visual direction;
- narration/persona/voice direction;
- caption/music direction;
- CTA;
- potential factual/research gaps.

Creator can edit fields, request targeted revision or regenerate alternatives.

## Stage 4 — Proposal approval
No large-scale media generation from an unapproved proposal unless creator explicitly chooses an accelerated/autopilot workflow added in a future product decision.

Persist approved proposal version.

## Stage 5 — Script
Generate or transform the script from approved intent/proposal. Support manual editing and section-level AI revision. Preserve versions/history enough to avoid losing accepted edits.

Long-form scripts must not be squeezed into short-video structures.

## Stage 6 — Script approval
Approval produces/updates scene-plan input. If creator later changes upstream script content, affected downstream scenes should be marked stale rather than silently remaining “approved”.

## Stage 7 — Scene plan
Break content into ordered scenes/segments with:
- purpose/section relationship;
- narration/text;
- visual instruction;
- planned source type (stock/upload/generated image/generated video/etc.);
- expected timing/duration;
- caption intent;
- transition notes;
- provenance/dependencies.

## Stage 8 — Media/audio generation & acquisition
Assets are generated/found/uploaded at scene level where possible. Individual failures can retry/replace without discarding other accepted scenes.

## Stage 9 — Scene editor
Creator can review and manipulate each scene at minimum:
- text/narration;
- selected/generated visual;
- timing/duration;
- captions;
- voice/audio choice and relevant parameters;
- background music relationship;
- transition/basic visual treatment;
- reorder/duplicate/delete;
- regenerate/replace individual components;
- preview.

## Stage 10 — Render/export
Render is an explicit version/job. Editing state survives render failures. A rendered artifact records the project/version that produced it.

## Stage 11 — Publish
A successful render can target one or more connected channels through platform capability-aware publishing adapters.
