# TASK-023 — Media Library + Scene Binding API integration

Status: READY
Milestone: F1 Creative Workflow
Wave: WAVE-F1-J
Branch: `feature/TASK-023-media-library-api`
Base: `develop`
Depends on: TASK-016 accepted; TASK-020 accepted; shared backend composition hotspot released after TASK-021 acceptance.

## Goal
Make accepted S3-compatible Media Asset storage and Scene Media Binding foundations usable through secure streaming creator APIs and real application runtime composition.

## Frozen contract
`docs/contracts/MEDIA_LIBRARY_API_V1.md`.

## Primary ownership
- Media Asset/Scene Media Binding HTTP handlers/tests;
- storage runtime/config wiring;
- minimal `mediaasset.ObjectStorage` range-read extension + accepted S3 adapter implementation/tests;
- friendly asset-in-use integration;
- minimal `httpserver/server.go` + `cmd/api/main.go` composition;
- config/env/docs/local integration required for storage;
- focused Media/Binding service/repository adapters only where frozen contract requires.

## Required capability
- bounded streaming upload;
- safe list/get metadata;
- authenticated streaming content with standard single-byte-range support;
- delete with `MEDIA_ASSET_IN_USE` for active or historical scene references;
- current plan binding list with unbound scenes;
- assign/replace primary visual;
- binding history;
- SeaweedFS/S3-compatible local development wiring with no public internet CI.

## Mandatory isolation
Do not implement:
- frontend Media workspace (TASK-024);
- image/video generation adapters/jobs;
- TTS/audio;
- Scene Editor/render/publish;
- a second object-storage implementation.

## Critical gates
1. File body is never fully buffered just to upload.
2. Client never chooses object key/origin/owner identity.
3. Storage secrets/object key/bucket never public.
4. Video preview supports bounded standard single-range reads.
5. Cross-owner/project object access is non-disclosing.
6. Historical binding references block asset deletion.
7. Storage startup config fails explicit invalid configuration safely.
8. Existing MediaAsset compensation/integrity semantics remain intact.

## Revalidation at activation
Accepted `mediaasset.ObjectStorage` currently exposes `Put`, `Stat`, `Open`, and `Delete`; it has no range-open primitive. TASK-023 may add the minimum provider-neutral range-read method required by `MEDIA_LIBRARY_API_V1` and implement it only in the accepted S3-compatible adapter. Do not redesign object storage or add another implementation.

TASK-021 is accepted and no other active task owns `main.go` / `httpserver` backend composition. TASK-022 is frontend-only, so the write surfaces are independent.

## TDD
Implement all HTTP/runtime/storage tests from `MEDIA_LIBRARY_API_V1`, including oversize streaming rejection, Range 206/416 behavior, credential sentinel safety, local S3-compatible integration and real PostgreSQL binding/delete behavior.

## Worktree / claim
Remote `feature/TASK-023-media-library-api` was absent when PM/TL promoted this task. Atomically claim it from latest `origin/develop`, use a dedicated worktree, and keep the shared/control checkout on `develop`.

Do not self-mark DONE or self-merge.