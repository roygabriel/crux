// @ts-check

/** @type {import('@docusaurus/plugin-content-docs').SidebarsConfig} */
const sidebars = {
  docsSidebar: [
    {
      type: 'category',
      label: 'Getting Started',
      collapsed: false,
      items: [
        'getting-started/installation',
        'getting-started/first-project',
      ],
    },
    {
      type: 'category',
      label: 'Concepts',
      collapsed: false,
      items: [
        'concepts/architecture',
        'concepts/phase-system',
        'concepts/memory',
        'concepts/security',
      ],
    },
    {
      type: 'category',
      label: 'Guides',
      items: [
        'guides/writing-phases',
        'guides/multi-agent',
        'guides/custom-plugins',
      ],
    },
    {
      type: 'category',
      label: 'Reference',
      items: [
        'reference/configuration',
        'reference/roles',
        'reference/cli',
        'reference/go-docs',
      ],
    },
    'troubleshooting',
    'contributing',
  ],
};

export default sidebars;
