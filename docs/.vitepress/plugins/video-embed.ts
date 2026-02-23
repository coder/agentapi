/**
 * Video embed plugin for VitePress markdown.
 *
 * Allows embedding recorded Playwright videos in documentation using:
 *   ![Alt text](/path/to/video.webm)
 *   or custom syntax:
 *   <video width="100%" controls>
 *     <source src="/recordings/demo.webm" type="video/webm">
 *   </video>
 *
 * Supports webm, mp4, and other HTML5 video formats.
 */

import type { MarkdownIt } from 'markdown-it'

interface VideoEmbedOptions {
  width?: string
  height?: string
  controls?: boolean
  autoplay?: boolean
  loop?: boolean
  muted?: boolean
}

/**
 * Parse video embed directive syntax.
 * Example: ::: video /path/to/video.webm :::
 */
function parseVideoDirective(
  md: MarkdownIt,
  _options: VideoEmbedOptions
): void {
  const videoBlockRule = (state: any, startLine: number, endLine: number) => {
    const pos = state.bMarks[startLine] + state.tShift[startLine]
    const maximum = state.eMarks[startLine]

    // Check for ::: video syntax
    if (pos + 3 > maximum) return false
    if (state.src.slice(pos, pos + 3) !== ':::') return false

    const markerCount = 3
    const markup = state.src.slice(pos, pos + markerCount)
    const params = state.src.slice(pos + markerCount, maximum).trim()

    if (!params.startsWith('video ')) return false

    const videoSrc = params.slice(6).trim()
    if (!videoSrc) return false

    let nextLine = startLine + 1

    // Find closing marker
    while (nextLine < endLine) {
      if (
        state.bMarks[nextLine] + state.tShift[nextLine] + 3 <=
        state.eMarks[nextLine]
      ) {
        const closePos =
          state.bMarks[nextLine] + state.tShift[nextLine]
        if (
          state.src.slice(closePos, closePos + 3) === ':::'
        ) {
          break
        }
      }
      nextLine++
    }

    const oldParent = state.parentType
    state.parentType = 'paragraph'

    const token = state.push('video_block', 'div', 0)
    token.markup = markup
    token.meta = { src: videoSrc }
    token.map = [startLine, nextLine + 1]

    state.parentType = oldParent
    state.line = nextLine + 1

    return true
  }

  md.block.ruler.before(
    'fence',
    'video_block',
    videoBlockRule
  )

  md.renderer.rules.video_block = (tokens, idx) => {
    const token = tokens[idx]
    const src = token.meta?.src || ''

    return `<video width="100%" controls>
  <source src="${src}" type="video/webm">
  Your browser does not support the video tag.
</video>\n`
  }
}

/**
 * Enhanced image rendering to support video files.
 * Converts ![video](file.webm) to <video> tags.
 */
function enhanceImageRendering(
  md: MarkdownIt,
  options: VideoEmbedOptions
): void {
  const originalImageRule = md.renderer.rules.image

  md.renderer.rules.image = (tokens, idx, _options, env, renderer) => {
    const token = tokens[idx]
    const src = token.attrGet('src') || ''

    // Check if it's a video file
    if (src.match(/\.(webm|mp4|ogg|mov)$/i)) {
      const alt = token.content || 'Video'
      const width = options.width || '100%'
      const controls = options.controls !== false ? 'controls' : ''
      const autoplay = options.autoplay ? 'autoplay' : ''
      const loop = options.loop ? 'loop' : ''
      const muted = options.muted ? 'muted' : ''

      const ext = src.split('.').pop()?.toLowerCase()
      let type = 'video/webm'
      if (ext === 'mp4') type = 'video/mp4'
      else if (ext === 'ogg') type = 'video/ogg'
      else if (ext === 'mov') type = 'video/quicktime'

      return `<video width="${width}" ${controls} ${autoplay} ${loop} ${muted}>
  <source src="${src}" type="${type}">
  ${alt}
</video>`
    }

    // Fall back to default image rendering
    return originalImageRule?.(tokens, idx, _options, env, renderer) || ''
  }
}

/**
 * VitePress plugin for video embedding in markdown.
 *
 * Usage in markdown:
 *   ![My Video](/recordings/demo.webm)
 *   or:
 *   ::: video /recordings/demo.webm :::
 *
 * @param md MarkdownIt instance
 * @param options Video embed options
 */
export function videoEmbedPlugin(
  md: MarkdownIt,
  options: Partial<VideoEmbedOptions> = {}
): void {
  const defaultOptions: VideoEmbedOptions = {
    width: '100%',
    height: 'auto',
    controls: true,
    autoplay: false,
    loop: false,
    muted: false,
    ...options,
  }

  parseVideoDirective(md, defaultOptions)
  enhanceImageRendering(md, defaultOptions)
}

export type { VideoEmbedOptions }
