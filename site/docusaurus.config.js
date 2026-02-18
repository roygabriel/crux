// @ts-check
import { themes as prismThemes } from 'prism-react-renderer';

/** @type {import('@docusaurus/types').Config} */
const config = {
  title: 'Crux',
  tagline: 'Orchestrate AI coding agents with persistent memory',
  favicon: 'img/favicon.svg',

  future: {
    v4: true,
  },

  url: 'https://roygabriel.github.io',
  baseUrl: '/crux/',

  organizationName: 'roygabriel',
  projectName: 'crux',

  onBrokenLinks: 'throw',

  markdown: {
    hooks: {
      onBrokenMarkdownLinks: 'warn',
    },
  },

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

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
      }),
    ],
  ],

  themeConfig:
    /** @type {import('@docusaurus/preset-classic').ThemeConfig} */
    ({
      image: 'img/crux-social-card.png',
      colorMode: {
        defaultMode: 'dark',
        respectPrefersColorScheme: true,
      },
      navbar: {
        title: 'Crux',
        logo: {
          alt: 'Crux Logo',
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
