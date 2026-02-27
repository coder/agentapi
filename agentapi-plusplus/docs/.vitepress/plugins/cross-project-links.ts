import type MarkdownIt from 'markdown-it'
import type { RenderRule } from 'vitepress'

// Map project names to their docs-dist paths
const PROJECT_PATHS: Record<string, string> = {
  'thegent': '/Users/kooshapari/temp-PRODVERCEL/485/kush/thegent/docs-dist/main',
  'jobhunter': '/Users/kooshapari/Dev/job-hunter/docs-dist',
  'heliosShield': '/Users/kooshapari/temp-PRODVERCEL-485/kush/heliosShield/docs-dist',
  'trace': '/Users/kooshapari/kush/trace/docs-dist',
}

export function crossProjectLinks(md: MarkdownIt) {
  const defaultRender: RenderRule = md.renderer.rules.link_open || function(tokens, idx, options, _env, self) {
    return self.renderToken(tokens, idx, options)
  }

  md.renderer.rules.link_open = function(tokens, idx, options, env, self) {
    const href = tokens[idx].attrGet('href')

    // Check for ~project:/path pattern
    if (href && href.startsWith('~')) {
      const match = href.match(/^~([^:]+):(.+)$/)
      if (match) {
        const [, project, path] = match
        const basePath = PROJECT_PATHS[project]

        if (basePath) {
          // Convert markdown path to HTML path
          const htmlPath = path
            .replace(/\.md$/, '.html')
            .replace(/^\/+/, '')

          tokens[idx].attrSet('href', `file://${basePath}/${htmlPath}`)
          tokens[idx].attrSet('target', '_blank')
          tokens[idx].attrSet('class', 'cross-project-link')
        }
      }
    }

    return defaultRender(tokens, idx, options, env, self)
  }
}
