import type { DefaultTheme } from 'vitepress';

export const sidebar: DefaultTheme.Sidebar = {
  '/': [
    {
      text: 'Get started',
      items: [
        { text: 'Quickstart', link: '/QUICKSTART' },
        { text: 'Install', link: '/INSTALL' },
        { text: 'Environment', link: '/ENVIRONMENT' },
        { text: 'Architecture', link: '/ARCHITECTURE' }
      ]
    },
    {
      text: 'Reference',
      items: [
        { text: 'API', link: '/API' },
        { text: 'CLI', link: '/CLI' },
        { text: 'Operations', link: '/OPERATIONS' },
        { text: 'Security', link: '/SECURITY' },
        { text: 'Troubleshooting', link: '/TROUBLESHOOTING' }
      ]
    },
    {
      text: 'Appendix',
      items: [
        { text: 'FAQ', link: '/FAQ' },
        { text: 'Glossary', link: '/GLOSSARY' },
        { text: 'Roadmap', link: '/ROADMAP' },
        { text: 'Style guide', link: '/STYLEGUIDE' },
        { text: 'Examples', link: '/examples/README' }
      ]
    }
  ]
};
