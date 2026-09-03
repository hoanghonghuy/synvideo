# STOCK_MEDIA_ACQUISITION_V1

## Purpose
Define the provider-neutral contract for creator-driven stock image/video discovery, truthful license/provenance preview, explicit acquisition into SynVideo-managed storage, and scene assignment.

## Core model
A normalized stock search result contains:
- provider key;
- opaque provider result ID;
- media kind: image or video;
- bounded preview representation;
- creator/display attribution metadata when supplied;
- provider source-page URL when supplied;
- normalized license/usage summary plus provider-specific raw reference needed for audit;
- acquisition capability/state;
- provider capability metadata required for truthful UI behavior.

Provider-specific IDs/URLs are adapter data and never become generic scene or MediaAsset identity.

## Search boundary
Search is explicitly user-driven and bounded. The domain request may include semantic query text, media kind, orientation/aspect preference and page/cursor information only when supported.

Adapters normalize provider pagination/cursor behavior and must impose finite page/result bounds. No V1 path performs catalog mirroring, bulk crawling, scraping, dataset construction or hidden prefetch of an entire provider corpus.

Unsupported filters or capabilities fail/disable truthfully rather than being emulated inaccurately.

## License, attribution and provenance truth
Before acquisition, the creator must be able to inspect the stock source, creator/author when available, license/usage summary and attribution guidance relevant to the selected provider/item.

Acquisition persists provenance sufficient for later audit and downstream attribution decisions, including at minimum:
- provider key;
- opaque source/result ID;
- source-page URL when available;
- creator attribution fields when available;
- license/usage reference captured at acquisition time;
- acquisition timestamp;
- provider metadata needed to reconstruct required/recommended attribution without exposing credentials.

The application must not represent provider content as owned by SynVideo or hide a required attribution obligation.

## Acquisition semantics
Search results are remote candidates, not durable project assets.

Scene assignment succeeds only after an explicit acquisition step has produced or deterministically reused a durable same-project `MediaAsset`. Once acquisition succeeds, the durable asset must remain usable independent of normal expiry of provider preview/download URLs.

Acquisition must use provider-authorized download/content endpoints and respect provider terms. It must not bypass access controls, scrape disallowed endpoints or silently switch to a different source item when the selected item fails.

## Idempotency and duplicate handling
The same logical acquisition request must be idempotent or deterministically deduplicated. Retrying because of browser refresh, worker retry or an uncertain local response must not create uncontrolled duplicate MediaAssets for the same selected stock item.

A safe deduplication identity may include project, provider key, opaque provider result ID and immutable source-version information where available. Deduplication must not accidentally share private project assets across project/principal boundaries.

## Error and recovery semantics
Adapters classify provider failures into truthful states such as:
- rate limited / retry later;
- source item removed/unavailable;
- authorization/configuration failure;
- transient provider/network failure;
- invalid/unsupported request;
- acquisition failed after search result discovery.

The creator can retry a recoverable acquisition without losing the selected result context. A source item that disappeared is surfaced as unavailable; the system must never silently substitute another stock result.

## Project isolation
Search may be provider-global, but acquisition and all durable MediaAsset/binding operations are project/principal scoped. A principal cannot acquire into or assign media to a project they cannot access.

Credentials/API keys never reach browser-visible result payloads, MediaAsset provenance, logs or durable public job payloads.

## Scene binding boundary
TASK-033 reuses existing scene MediaAsset binding/history semantics. Stock origin changes provenance, not the generic scene binding identity model.

Replacing a stock visual preserves existing binding history. Downstream editor/render/export consumers use the selected durable MediaAsset and may inspect provenance/attribution metadata without depending on remote provider URLs.

## UI truthfulness
Creator UI distinguishes:
- searching;
- results;
- empty results;
- provider unavailable/error;
- rate limited;
- acquiring;
- acquired;
- source unavailable;
- assignment success/error.

License/source/creator information must be available before the user commits acquisition. Unsupported filters/actions are disabled or omitted.

## Provider compliance gate
The first live adapter is not selected permanently by this contract. Immediately before implementation READY, PM/TL must revalidate current provider API approval, limits, license, attribution, download/redistribution and anti-mirroring terms.

Provider-specific restrictions belong in the adapter/product capability layer. If current terms do not permit SynVideo's explicit search-and-acquire workflow, that provider is rejected or the product flow is narrowed rather than weakening this contract.

As of 2026-09-04, Pexels is a candidate because it exposes image/video search APIs and permits broad use under the Pexels license, but its API/terms include attribution guidance and restrictions against competing-service/catalog replication and unauthorized large-scale extraction/ML dataset use. READY-time revalidation remains mandatory.

## Observability and privacy
Diagnostics may record provider key, normalized operation/result IDs, rate/error categories, acquisition state and safe timing data. Do not log API keys, authorization headers, private user content or unnecessary provider payload bodies.

## Required regression coverage
- provider result normalization for image/video;
- bounded pagination and unsupported-filter truthfulness;
- source/creator/license/attribution mapping;
- same-project acquisition and cross-project rejection;
- successful durable MediaAsset survives remote URL expiry;
- duplicate logical acquisition is idempotent/deduplicated;
- removed/unavailable item surfaces without silent substitution;
- rate-limit/transient failure recovery;
- assignment/replacement preserves binding history;
- browser refresh recovers persisted acquisition state where applicable;
- no live provider network dependency in ordinary tests.
