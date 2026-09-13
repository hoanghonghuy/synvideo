export type RenderCaptionPresentation = {
  burnedInLabel: string
  burnedInDescription: string
  sidecarLabel: string
  sidecarDescription: string
  sidecarUnavailableReason: string | null
}

/**
 * Keeps the Scene Editor export language explicit: snapshot-bound captions are
 * burned into the MP4 independently from the optional downloadable WebVTT.
 * This is intentionally presentation-only; it must not change render identity.
 */
export function renderCaptionPresentation(
  hasSnapshotBoundCaptions: boolean,
  subtitleMode: 'off' | 'webvtt',
): RenderCaptionPresentation {
  const burnedInLabel = 'In-video captions'
  const sidecarLabel = 'Downloadable WebVTT'

  if (!hasSnapshotBoundCaptions) {
    return {
      burnedInLabel,
      burnedInDescription: 'No snapshot-bound captions are available, so the MP4 will not contain captions.',
      sidecarLabel,
      sidecarDescription: 'No WebVTT file will be created.',
      sidecarUnavailableReason: 'No captions are bound to this immutable snapshot.',
    }
  }

  return {
    burnedInLabel,
    burnedInDescription: 'Snapshot-bound captions are embedded directly in the rendered MP4.',
    sidecarLabel,
    sidecarDescription:
      subtitleMode === 'webvtt'
        ? 'A separate WebVTT download will also be finalized from the same immutable caption snapshot.'
        : 'No separate WebVTT download will be created. This does not disable captions embedded in the MP4.',
    sidecarUnavailableReason: subtitleMode === 'webvtt' ? null : 'WebVTT download is turned off for this render.',
  }
}
