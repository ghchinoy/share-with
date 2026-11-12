// @ts-check
import { defineConfig, fontProviders } from 'astro/config';
import starlight from '@astrojs/starlight';
import starlightLlmsTxt from 'starlight-llms-txt';

const site = process.env.SITE_URL || 'https://ghchinoy.github.io';
const rawBase = process.env.BASE_PATH !== undefined ? process.env.BASE_PATH : '/share-with';
const base = rawBase === '/' ? '' : rawBase.replace(/\/$/, '');

// Static landing + Starlight documentation site for share-with.
// Self-hosts Google Sans Flex and Google Sans Code so local and deployed builds
// make zero runtime requests to Google Fonts hosts.
export default defineConfig({
  site,
  ...(base ? { base } : {}),
  output: 'static',
  integrations: [
    starlight({
      title: 'share-with',
      description:
        'Agent-Aware Go CLI and Agent Plugin for silently granting Google Docs, Sheets, Slides, Colab, and Google Drive permissions with notifications permanently disabled.',
      logo: {
        src: './src/assets/logo.svg',
      },
      favicon: '/favicon.svg',
      social: [
        {
          icon: 'github',
          label: 'GitHub',
          href: 'https://github.com/ghchinoy/share-with',
        },
      ],
      components: {
        Head: './src/components/StarlightHead.astro',
      },
      customCss: ['./src/styles/starlight.css'],
      plugins: [
        starlightLlmsTxt({
          projectName: 'share-with',
          description:
            'Agent-Aware Go CLI and Agent Plugins v1.0.0 package that grants Google Docs, Sheets, Slides, Colab, and Google Drive permissions (reader, commenter, editor) with sendNotificationEmail=false enforced across CLI flags, Drive API v3, and HTTP transport.',
        }),
      ],
      sidebar: [
        {
          label: 'Start Here',
          items: [
            { label: 'Getting Started', slug: 'getting-started' },
            { label: 'Agent Plugin & Skill', slug: 'agent-plugin' },
          ],
        },
        {
          label: 'Architecture & Reference',
          items: [
            { label: 'Silent-Only Guarantee', slug: 'silent-guarantee' },
            { label: 'CLI & JSON Reference', slug: 'cli-reference' },
            { label: 'Authentication & Config', slug: 'authentication' },
            { label: 'Troubleshooting', slug: 'troubleshooting' },
          ],
        },
      ],
    }),
  ],
  fonts: [
    {
      provider: fontProviders.local(),
      name: 'Google Sans Flex',
      cssVariable: '--font-display',
      fallbacks: ['system-ui', 'sans-serif'],
      options: {
        variants: [
          {
            weight: '100 1000',
            style: 'normal',
            display: 'swap',
            src: ['./src/assets/fonts/google-sans-flex.woff2'],
          },
        ],
      },
    },
    {
      provider: fontProviders.local(),
      name: 'Google Sans Code',
      cssVariable: '--font-mono',
      fallbacks: ['ui-monospace', 'SFMono-Regular', 'Menlo', 'monospace'],
      options: {
        variants: [
          {
            weight: '300 800',
            style: 'normal',
            display: 'swap',
            src: ['./src/assets/fonts/google-sans-code.woff2'],
          },
        ],
      },
    },
  ],
  vite: {
    build: {
      cssTarget: 'esnext',
    },
  },
});
