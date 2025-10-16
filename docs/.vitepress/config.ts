import { defineConfig } from 'vitepress'

const hostname = process.env.DOCS_SITEMAP_HOST ?? 'https://kubeop.example.com'

export default defineConfig({
  title: 'kubeOP',
  description: 'Out-of-cluster control plane for multi-tenant Kubernetes',
  appearance: true,
  lastUpdated: true,
  cleanUrls: true,
  base: '/kubeOP/',
  sitemap: {
    hostname
  },
  rss: {
    hostname,
    title: 'kubeOP Documentation'
  },
  themeConfig: {
    nav: [
      { text: 'Overview', link: '/' },
      { text: 'Quickstart', link: '/getting-started' },
      { text: 'Architecture', link: '/architecture' },
      {
        text: 'Guides',
        activeMatch: '^/guides/',
        items: [
          { text: 'Tenants & apps', link: '/guides/tenants-projects-apps' },
          { text: 'Kubeconfig & RBAC', link: '/guides/kubeconfig-and-rbac' },
          { text: 'Helm & OCI', link: '/guides/helm-oci-deployments' },
          { text: 'Quotas & limits', link: '/guides/quotas-and-limits' },
          { text: 'Watcher sync', link: '/guides/watcher-sync' }
        ]
      },
      { text: 'API', link: '/api/' },
      { text: 'Operations', link: '/operations' }
    ],
    sidebar: [
      {
        text: 'Overview',
        items: [
          { text: 'Home', link: '/' },
          { text: 'Getting started', link: '/getting-started' }
        ]
      },
      {
        text: 'Architecture',
        items: [
          { text: 'System architecture', link: '/architecture' }
        ]
      },
      {
        text: 'Configuration',
        items: [
          { text: 'Environment variables', link: '/configuration' }
        ]
      },
      {
        text: 'Operations',
        items: [
          { text: 'Operations runbook', link: '/operations' }
        ]
      },
      {
        text: 'Guides',
        items: [
          { text: 'Tenants, projects, and apps', link: '/guides/tenants-projects-apps' },
          { text: 'Kubeconfig and RBAC', link: '/guides/kubeconfig-and-rbac' },
          { text: 'Helm and OCI deployments', link: '/guides/helm-oci-deployments' },
          { text: 'Quotas and limits', link: '/guides/quotas-and-limits' },
          { text: 'Watcher sync', link: '/guides/watcher-sync' }
        ]
      },
      {
        text: 'API reference',
        items: [
          { text: 'Overview', link: '/api/' },
          { text: 'Health & metadata', link: '/api/health' },
          { text: 'Clusters', link: '/api/clusters' },
          { text: 'Users', link: '/api/users' },
          { text: 'Projects & apps', link: '/api/projects' },
          { text: 'Kubeconfigs', link: '/api/kubeconfigs' },
          { text: 'Templates', link: '/api/templates' },
          { text: 'Webhooks', link: '/api/webhooks' },
          { text: 'Watcher ingest (planned)', link: '/api/watcher-ingest' }
        ]
      },
      {
        text: 'Reference',
        items: [
          { text: 'Troubleshooting', link: '/troubleshooting' },
          { text: 'FAQ', link: '/faq' },
          { text: 'Glossary', link: '/glossary' },
          { text: 'Architecture decisions', link: '/adr' },
          { text: 'Changelog', link: '/changelog' },
          { text: 'Contributing', link: '/contributing' },
          { text: 'Code of conduct', link: '/code-of-conduct' }
        ]
      }
    ],
    docFooter: {
      prev: 'Previous',
      next: 'Next'
    },
    search: {
      provider: 'local'
    },
    socialLinks: [
      { icon: 'github', link: 'https://github.com/vaheed/kubeOP' }
    ],
    editLink: {
      pattern: 'https://github.com/vaheed/kubeOP/edit/main/docs/:path',
      text: 'Edit this page'
    },
    footer: {
      message: 'Released under the MIT License.',
      copyright: `© ${new Date().getFullYear()} kubeOP`
    }
  }
})
