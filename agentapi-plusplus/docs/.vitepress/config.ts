import { defineConfig } from 'vitepress'

// Import shared base config helpers
import { resolveDocsBase, resolveFavicon } from "../../../docs-hub/.vitepress/base.config"

// Supported locales: en, zh-CN, zh-TW, fa, fa-Latn
const locales = {
  root: {
    label: "English",
    lang: "en",
    title: 'agentapi++',
    description: 'Agent API server docs'
  },
  "zh-CN": {
    label: "简体中文",
    lang: "zh-CN",
    title: 'agentapi++',
    description: 'Agent API 服务器文档'
  },
  "zh-TW": {
    label: "繁體中文",
    lang: "zh-TW",
    title: 'agentapi++',
    description: 'Agent API 伺服器文檔'
  },
  fa: {
    label: "فارسی",
    lang: "fa",
    title: 'agentapi++',
    description: 'مستندات سرور API عامل'
  },
  "fa-Latn": {
    label: "Pinglish",
    lang: "fa-Latn",
    title: 'agentapi++',
    description: 'Agent API server docs (Latin)'
  }
};

const docsBase = resolveDocsBase()

export default defineConfig({
  title: 'agentapi++',
  description: 'Agent API server docs',
  base: docsBase,
  locales,
  themeConfig: {
    nav: [
      { text: 'Start Here', link: '/index' },
      { text: 'Tutorials', link: '/tutorials/' },
      { text: 'How-to', link: '/how-to/' },
      { text: 'Explanation', link: '/explanation/' },
      { text: 'Operations', link: '/operations/' },
      { text: 'API', link: '/api/' },
      {
        text: "🌐 Language",
        items: [
          { text: "English", link: "/" },
          { text: "简体中文", link: "/zh-CN/" },
          { text: "繁體中文", link: "/zh-TW/" },
          { text: "فارسی", link: "/fa/" },
          { text: "Pinglish", link: "/fa-Latn/" }
        ]
      }
    ],
    sidebar: [
      {
        text: 'Docs',
        items: [
          { text: 'Start Here', link: '/index' },
          { text: 'Tutorials', link: '/tutorials/' },
          { text: 'How-to', link: '/how-to/' },
          { text: 'Explanation', link: '/explanation/' },
          { text: 'Operations', link: '/operations/' },
          { text: 'API', link: '/api/' }
        ]
      }
    ]
  }
})
