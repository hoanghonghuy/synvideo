# Channel Hub

## Product direction
SynVideo should eventually let creators connect and manage publishing destinations from the same workspace used to create videos.

This does **not** mean implementing a fake universal social API. Each platform exposes different capabilities, permissions, audits, rate limits and publishing rules. The product presents a unified experience backed by capability-aware adapters.

## Core user goals
- connect/disconnect creator channels/accounts securely;
- see which channels a project can publish to;
- configure title/description/tags/thumbnail/privacy/platform-specific fields;
- publish now or schedule where the platform and account allow it;
- track upload/processing/publish failure states;
- view content published from SynVideo and selected channel/content performance where APIs permit;
- manage multiple connected channels without exposing credentials.

## Capability model
A platform adapter should be able to report capabilities such as:
- `can_publish_video`
- `can_upload_draft`
- `can_schedule`
- `can_update_metadata`
- `can_delete_or_unpublish`
- `can_read_content`
- `can_read_channel_stats`
- `can_read_content_stats`
- `supports_thumbnail_upload`
- supported privacy/audience settings
- upload/duration/file constraints

UI must hide/disable/explain unsupported capabilities rather than silently failing.

## Initial platform direction
### YouTube
High priority because SynVideo supports long-form and Shorts. Target connected-channel identity, video upload, metadata/thumbnail where API supports it, scheduling/privacy/status, and useful channel/video metrics under authorized scopes.

### TikTok
High priority for short-form distribution. Integrate only through official posting/authorization workflows and adapt UX to app review/audit and account capability restrictions.

### Instagram / Meta
Desired platform, but implementation must be designed from current official Graph API capabilities and eligible professional account requirements at task time.

## Data/product entities
- ChannelConnection
- ChannelCapabilitySnapshot
- Publication
- PublicationTarget
- PublicationAttempt
- ScheduledPublication
- ExternalContentReference
- ChannelMetricSnapshot / ContentMetricSnapshot (later)

A rendered SynVideo video is not itself a published post. One render can have multiple publication targets/attempts.

## Security
- OAuth/state/PKCE as required by platform.
- tokens stored encrypted and server-side;
- minimal scopes;
- refresh/revocation lifecycle;
- clear disconnect/delete behavior;
- never log access/refresh tokens.

## Product phases
1. Connections + capability discovery.
2. Manual publish from a completed render.
3. Publication status/error recovery.
4. Scheduling where support exists.
5. Content library/channel dashboard.
6. Analytics and optimization recommendations.

Platform documentation and policies are volatile. Every implementation task must re-check official current API requirements instead of assuming this document is an API contract.
