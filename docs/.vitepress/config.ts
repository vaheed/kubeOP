import { defineConfig } from 'vitepress'

const base = process.env.DOCS_BASE ?? '/kubeOP/'

export default defineConfig({
  title: 'kubeOP Documentation',
  description: 'Operator-powered multi-tenant application platform for Kubernetes',
  lang: 'en-US',
  lastUpdated: true,
  base,
  themeConfig: {
    logo: '/logo.svg',
    nav: [
      { text: 'Getting Started', link: '/getting-started' },
      { text: 'Bootstrap Guide', link: '/bootstrap-guide' },
      { text: 'Operations', link: '/operations' },
      { text: 'Delivery', link: '/delivery' },
      { text: 'API Reference', link: '/api-reference' },
      { text: 'Production Hardening', link: '/production-hardening' }
    ],
     footer: {
      message: 'Released under the MIT License.',
      copyright: '© ' + new Date().getFullYear() + ' kubeOP contributors'
    }
  }
})
