import { describe, expect, it } from 'vitest'

import { renderCaptionPresentation } from './renderCaptionPresentation'

describe('renderCaptionPresentation', () => {
  it('makes clear that sidecar off does not disable captions embedded in MP4', () => {
    const copy = renderCaptionPresentation(true, 'off')

    expect(copy.burnedInDescription).toContain('embedded directly')
    expect(copy.sidecarDescription).toContain('does not disable captions embedded in the MP4')
    expect(copy.sidecarUnavailableReason).toContain('turned off')
  })

  it('describes WebVTT as a separate artifact from the same immutable snapshot', () => {
    const copy = renderCaptionPresentation(true, 'webvtt')

    expect(copy.burnedInDescription).toContain('rendered MP4')
    expect(copy.sidecarDescription).toContain('separate WebVTT')
    expect(copy.sidecarDescription).toContain('same immutable caption snapshot')
    expect(copy.sidecarUnavailableReason).toBeNull()
  })

  it('does not promise either caption output when the snapshot has no captions', () => {
    const copy = renderCaptionPresentation(false, 'webvtt')

    expect(copy.burnedInDescription).toContain('will not contain captions')
    expect(copy.sidecarDescription).toBe('No WebVTT file will be created.')
    expect(copy.sidecarUnavailableReason).toContain('No captions')
  })
})
