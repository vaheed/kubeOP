import { defineConfig } from 'vitepress';
import { sidebar } from './sidebar';

export default defineConfig({
  lang: 'en-US',
  title: 'kubeOP',
  description: 'Multi-cluster operations without in-cluster controllers',
  base: '/kubeOP/',
  cleanUrls: true,
  markdown: {
    mermaid: true
  },
  themeConfig: {
    nav: [
      { text: 'Quickstart', link: '/QUICKSTART' },
      { text: 'Install', link: '/INSTALL' },
      { text: 'API', link: '/API' },
      { text: 'Operations', link: '/OPERATIONS' },
      { text: 'GitHub', link: 'https://github.com/vaheed/kubeOP' }
    ],
    sidebar,
    socialLinks: [
      { icon: 'github', link: 'https://github.com/vaheed/kubeOP' }
    ],
    footer: {
      message: 'Released under the Apache 2.0 License',
      copyright: 'Copyright © 2024 kubeOP contributors'
    }
  }
});
