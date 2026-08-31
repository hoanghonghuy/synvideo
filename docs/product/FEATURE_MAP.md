# Feature Map

This is a product capability map, not an implementation backlog. Actual tasks are created after codebase audit and milestone planning.

## A. Identity & workspace
- Sign up / sign in / sign out / session management.
- Profile and locale preferences.
- Workspace/project ownership and authorization.
- Provider credential/settings management (BYOK) with secure secret storage.

## B. Projects
- Create, rename, duplicate, archive/delete, search/filter projects.
- Project aspect ratio/platform/duration goals.
- Autosave/durable draft state and last-updated indicators.
- Project state/version awareness so generated media maps to the creative version that produced it.

## C. Source intake / Creative Brief
- Natural-language idea/description.
- Existing text/script/document.
- Upload image/video/audio/reference media.
- Mixed input sources.
- AI extraction/analysis of source media where useful.
- Separate creator-provided facts/constraints from AI recommendations.

## D. AI Proposal
- Generate structured creative direction from the brief.
- Multiple title/hook/angle options.
- Audience/objective/style/duration recommendations.
- Structure/outline.
- Visual/voice/music/caption directions.
- Edit, targeted revise, regenerate, approve.
- Persist approval and proposal versions.

## E. Script
- Script generation from approved brief/proposal.
- Script import and AI rewrite.
- Section-aware editing.
- Regenerate selected section without destroying manual changes elsewhere.
- Word/duration estimation.
- Approval/version checkpoints.

## F. Scene planning
- Convert approved script/content into ordered scenes.
- Scene purpose, narration, visual instruction, planned duration, source strategy.
- User can add/delete/reorder/split/merge scenes where appropriate.
- Long-form projects support many scenes/sections without artificial short-video limits.

## G. Asset library
- Upload user images/video/audio.
- Reuse assets across project scenes where intended.
- Track provenance: uploaded, stock source, generated provider/model, derived asset.
- Search/filter/preview/replace/remove.
- Generated asset history so regeneration does not immediately destroy the prior choice.

## H. AI images / video
- Text-to-image.
- Image editing where provider supports it.
- Text-to-video and image-to-video.
- Provider/model capability-aware parameters.
- Generation cost/usage estimate/display where obtainable.
- Per-scene regeneration and alternatives.

## I. Stock media
- Search/recommend stock images/videos using provider adapters.
- License/source attribution metadata as required.
- Avoid baking provider-specific semantics into scene/domain model.

## J. Voice, audio and captions
- TTS provider abstraction.
- Voice preview/selection/language and supported parameters.
- Optional user-provided voice/audio where allowed.
- Music/background audio selection/generation/import.
- Caption generation/transcription, style and correction.
- Audio/caption timing survives normal scene editing or is clearly invalidated/rebuilt.

## K. Scene editor — baseline comprehensive editor
Each scene should eventually support at least:
- narration/script text;
- visual/media choice and crop/fit/basic transform where relevant;
- duration/timing;
- captions and caption style;
- voice/audio configuration;
- background music relationship;
- transition;
- reorder, duplicate, delete;
- regenerate/replace individual components;
- preview.

Project editor also needs global settings such as aspect ratio, base style/brand direction, voice/music/caption defaults and project preview.

The data model must leave a credible path to multi-track timeline/keyframes later without making the first editor a crippled demo.

## L. Render/export
- Render jobs with queued/running/succeeded/failed/cancelled states.
- Progress and actionable errors.
- Retry without losing edits.
- Common platform-oriented output presets plus manual quality settings.
- MP4 baseline; subtitle export where supported.
- Keep rendered-version provenance.

## M. Channel Hub
- Connect multiple creator channels/accounts with OAuth as required.
- Capability discovery by platform/account.
- Publish rendered videos.
- Platform-specific metadata/settings.
- Schedule where platform APIs and account state allow.
- Publish/upload/processing status and retry/recovery.
- Content library for posts managed through SynVideo.
- Channel/content analytics where APIs permit.
- Never pretend unsupported platform capability exists.

See `CHANNEL_HUB.md`.

## N. Internationalization
- i18n routing/resources from initial frontend architecture.
- Vietnamese is the first required locale.
- English resources added progressively, with no hard-coded-language architecture debt.

## O. Operational completeness
A feature is not “done” solely because the happy-path button works. Relevant completion includes:
- validation and permissions;
- loading/empty/error/disabled/retry states;
- persistence and refresh behavior;
- concurrency/idempotency where relevant;
- observability for asynchronous jobs;
- automated tests appropriate to risk;
- responsive/accessibility baseline;
- security/privacy/secret handling;
- deployable configuration.
