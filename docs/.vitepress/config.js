const fs = require('fs');
const path = require('path');

const docsRoot = path.resolve(__dirname, '..');

function titleFromFilename(filename) {
  return filename
    .replace(/\.md$/, '')
    .replace(/_/g, ' ')
    .replace(/\b\w/g, (c) => c.toUpperCase());
}

function buildSidebar() {
  const entries = fs
    .readdirSync(docsRoot)
    .filter((file) => file.endsWith('.md') && file !== 'index.md')
    .sort((a, b) => a.localeCompare(b));

  const items = entries.map((file) => ({
    text: titleFromFilename(file),
    link: `/${file.replace(/\.md$/, '')}`,
  }));

  return [
    {
      text: 'Documentation',
      items,
    },
  ];
}

module.exports = {
  title: 'kubeOP',
  description: 'Multi-cluster control plane for tenants, apps, and automation',
  lang: 'en-US',
  appearance: true,
  lastUpdated: true,
  themeConfig: {
    logo: '/logo.svg',
    siteTitle: 'kubeOP',
    nav: [
      { text: 'Overview', link: '/README' },
      { text: 'Architecture', link: '/ARCHITECTURE' },
      { text: 'API', link: '/API' },
      { text: 'Deploy', link: '/DEPLOY' },
    ],
    sidebar: {
      '/': buildSidebar(),
    },
    search: {
      provider: 'local',
    },
    editLink: {
      pattern: 'https://github.com/vaheed/kubeOP/edit/main/docs/:path',
      text: 'Edit this page on GitHub',
    },
    socialLinks: [
      { icon: 'github', link: 'https://github.com/vaheed/kubeOP' },
    ],
    footer: {
      message: 'Released under the Apache 2.0 License.',
      copyright: '© ' + new Date().getFullYear() + ' kubeOP',
    },
  },
  head: [
    ['link', { rel: 'icon', href: '/favicon.svg' }],
  ],
};
