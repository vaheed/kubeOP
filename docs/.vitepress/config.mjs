import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'kubeOP',
  description: 'Operator, manager API, and operations documentation sourced from code',
  srcDir: '.',
  outDir: '.vitepress/dist',
  themeConfig: {
    nav: [
      { text: 'Getting Started', link: '/getting-started' },
      { text: 'Architecture', link: '/architecture' },
      { text: 'CRDs', link: '/crds' },
      { text: 'Controllers', link: '/controllers' },
      { text: 'Config', link: '/config' },
      { text: 'API', link: '/api' },
      { text: 'Operations', link: '/operations' },
      { text: 'Security', link: '/security' },
      { text: 'Troubleshooting', link: '/troubleshooting' },
      { text: 'Contributing', link: '/contributing' }
    ],
    sidebar: {
      '/': [
        {
          text: 'Overview',
          items: [
            { text: 'Getting Started', link: '/getting-started' },
            { text: 'Architecture', link: '/architecture' }
          ]
        },
        {
          text: 'Reference',
          items: [
            { text: 'CRDs', link: '/crds' },
            { text: 'Controllers', link: '/controllers' },
            { text: 'Config', link: '/config' },
            { text: 'API', link: '/api' }
          ]
        },
        {
          text: 'Operations',
          items: [
            { text: 'Operations', link: '/operations' },
            { text: 'Security', link: '/security' },
            { text: 'Troubleshooting', link: '/troubleshooting' },
            { text: 'Contributing', link: '/contributing' }
          ]
        }
      ]
    }
  }
})
