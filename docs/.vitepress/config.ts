import { defineConfig, type DefaultTheme } from 'vitepress'

const base = process.env.DOCS_BASE ?? '/kubeOP/'

const sidebar: DefaultTheme.Sidebar = {
  '/guide/': [
    { text: 'Overview', link: '/guide/index' },
    { text: 'Install', link: '/guide/install' },
    { text: 'Upgrade', link: '/guide/upgrade' },
    { text: 'Rollback', link: '/guide/rollback' },
    { text: 'Kubeconfig', link: '/guide/kubeconfig' },
    { text: 'Outbox & Offline-first', link: '/guide/outbox-offline-first' },
    { text: 'Drift Detection', link: '/guide/drift' }
  ],
  '/reference/': [
    { text: 'API', link: '/reference/api' },
    { text: 'CRDs', link: '/reference/crds' },
    { text: 'Health/Ready/Version/Metrics', link: '/reference/health' }
  ],
  '/ops/': [
    { text: 'Runbooks', link: '/ops/runbooks' },
    { text: 'Monitoring', link: '/ops/monitoring' },
    { text: 'Alerting', link: '/ops/alerting' }
  ],
  '/security/': [
    { text: 'RBAC', link: '/security/rbac' },
    { text: 'KMS', link: '/security/kms' },
    { text: 'Cert Rotation', link: '/security/cert-rotation' }
  ]
}

export default defineConfig({
  title: 'kubeOP Documentation',
  description: 'Operator-powered multi-tenant application platform for Kubernetes',
  lang: 'en-US',
  lastUpdated: true,
  base,
  head: [
    ['meta', { property: 'og:title', content: 'kubeOP Documentation' }],
    ['meta', { property: 'og:description', content: 'Multi-tenant app platform for Kubernetes' }],
    ['meta', { property: 'og:type', content: 'website' }]
  ],
  sitemap: { hostname: 'https://vaheed.github.io/kubeOP' },
  themeConfig: {
    logo: '/logo.svg',
    nav: [
      { text: 'Guide', link: '/guide/' },
      { text: 'Reference', link: '/reference/api' },
      { text: 'Ops', link: '/ops/runbooks' },
      { text: 'Security', link: '/security/rbac' },
      {
        text: 'v' + (process.env.DOCS_VER || 'dev'),
        items: [
          { text: 'dev (latest)', link: base },
          { text: 'v0.1.x', link: base }
        ]
      }
    ],
    sidebar,
    editLink: {
      pattern: 'https://github.com/vaheed/kubeOP/edit/develop/docs/:path',
      text: 'Edit this page'
    },
    socialLinks: [
      { icon: 'github', link: 'https://github.com/vaheed/kubeOP' }
    ],
    footer: {
      message: 'Released under the MIT License.',
      copyright: '© ' + new Date().getFullYear() + ' kubeOP contributors'
    }
  }
})
