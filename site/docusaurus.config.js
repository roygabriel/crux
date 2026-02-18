// @ts-check
import { themes as prismThemes } from 'prism-react-renderer';

/** @type {import('@docusaurus/types').Config} */
const config = {
  title: 'Crux - Multi-Agent AI Orchestrator',
  tagline: 'Coordinate AI coding agents in tmux sessions with phase-based workflows, persistent memory, and verification gates.',
  favicon: 'img/favicon.svg',

  url: 'https://runcrux.dev',
  baseUrl: '/',

  organizationName: 'roygabriel',
  projectName: 'crux',

  onBrokenLinks: 'throw',

  markdown: {
    mermaid: true,
    hooks: {
      onBrokenMarkdownLinks: 'warn',
    },
  },

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  themes: ['@docusaurus/theme-mermaid'],

  headTags: [
    {
      tagName: 'meta',
      attributes: {
        name: 'keywords',
        content: 'AI agent orchestration, multi-agent, Claude Code, Codex CLI, Gemini CLI, tmux, Go, CLI tool, phase-based workflows, persistent memory, coding agents',
      },
    },
    {
      tagName: 'meta',
      attributes: {
        name: 'author',
        content: 'Roy Gabriel',
      },
    },
    {
      tagName: 'script',
      attributes: {
        type: 'application/ld+json',
      },
      innerHTML: JSON.stringify({
        '@context': 'https://schema.org',
        '@type': 'SoftwareApplication',
        name: 'Crux',
        description: 'Open-source Go CLI that orchestrates multiple AI coding agents running in tmux sessions with phase-based execution, persistent memory, and verification gates.',
        applicationCategory: 'DeveloperApplication',
        operatingSystem: 'Linux, macOS',
        programmingLanguage: 'Go',
        license: 'https://opensource.org/licenses/MIT',
        url: 'https://runcrux.dev/',
        codeRepository: 'https://github.com/roygabriel/crux',
        author: {
          '@type': 'Person',
          name: 'Roy Gabriel',
          url: 'https://github.com/roygabriel',
        },
        offers: {
          '@type': 'Offer',
          price: '0',
          priceCurrency: 'USD',
        },
      }),
    },
  ],

  presets: [
    [
      'classic',
      /** @type {import('@docusaurus/preset-classic').Options} */
      ({
        docs: {
          sidebarPath: './sidebars.js',
          editUrl: 'https://github.com/roygabriel/crux/tree/main/site/',
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
        sitemap: {
          changefreq: 'weekly',
          priority: 0.5,
          filename: 'sitemap.xml',
        },
      }),
    ],
  ],

  themeConfig:
    /** @type {import('@docusaurus/preset-classic').ThemeConfig} */
    ({
      image: 'img/crux-social-card.svg',
      metadata: [
        { name: 'twitter:card', content: 'summary_large_image' },
        { name: 'twitter:title', content: 'Crux - Multi-Agent AI Orchestrator' },
        { name: 'twitter:description', content: 'Coordinate AI coding agents in tmux sessions with phase-based workflows, persistent memory, and verification gates.' },
        { property: 'og:type', content: 'website' },
        { property: 'og:site_name', content: 'Crux' },
      ],
      colorMode: {
        defaultMode: 'dark',
        respectPrefersColorScheme: true,
      },
      navbar: {
        title: 'Crux',
        logo: {
          alt: 'Crux - Multi-Agent AI Orchestrator Logo',
          src: 'img/logo.svg',
        },
        items: [
          {
            type: 'docSidebar',
            sidebarId: 'docsSidebar',
            position: 'left',
            label: 'Docs',
          },
          {
            to: '/docs/getting-started/installation',
            label: 'Getting Started',
            position: 'left',
          },
          {
            to: '/docs/guides/writing-phases',
            label: 'Guides',
            position: 'left',
          },
          {
            to: '/docs/reference/configuration',
            label: 'Reference',
            position: 'left',
          },
          {
            href: 'https://github.com/users/roygabriel/projects/2',
            label: 'Roadmap',
            position: 'left',
          },
          {
            href: 'https://github.com/roygabriel/crux',
            label: 'GitHub',
            position: 'right',
          },
        ],
      },
      footer: {
        style: 'dark',
        links: [
          {
            title: 'Documentation',
            items: [
              {
                label: 'Getting Started',
                to: '/docs/getting-started/installation',
              },
              {
                label: 'Architecture',
                to: '/docs/concepts/architecture',
              },
              {
                label: 'Configuration',
                to: '/docs/reference/configuration',
              },
            ],
          },
          {
            title: 'Guides',
            items: [
              {
                label: 'Writing Phases',
                to: '/docs/guides/writing-phases',
              },
              {
                label: 'Multi-Agent Workflows',
                to: '/docs/guides/multi-agent',
              },
              {
                label: 'Custom Plugins',
                to: '/docs/guides/custom-plugins',
              },
            ],
          },
          {
            title: 'More',
            items: [
              {
                label: 'GitHub',
                href: 'https://github.com/roygabriel/crux',
              },
              {
                label: 'Contributing',
                to: '/docs/contributing',
              },
              {
                label: 'Go Docs',
                href: 'https://pkg.go.dev/github.com/roygabriel/crux',
              },
            ],
          },
        ],
        copyright: `Copyright © ${new Date().getFullYear()} Roy Gabriel. MIT Licensed.<br/><br/><small>Claude is a trademark of Anthropic, PBC. Codex and OpenAI are trademarks of OpenAI, Inc. Gemini is a trademark of Google LLC. This project is independent and is not affiliated with, sponsored by, or endorsed by Anthropic, OpenAI, or Google. <a href="https://github.com/tmux/tmux" style="color:inherit;text-decoration:underline;">tmux</a> is a separate open-source project and is not part of or related to Crux.</small>`,
      },
      prism: {
        theme: prismThemes.github,
        darkTheme: prismThemes.dracula,
        additionalLanguages: ['bash', 'go', 'yaml', 'json'],
      },
    }),
};

export default config;
