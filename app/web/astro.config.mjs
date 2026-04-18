import { defineConfig } from 'astro/config';
import sitemap from '@astrojs/sitemap';
import cloudflare from '@astrojs/cloudflare';
import remarkMath from 'remark-math';
import rehypeKatex from 'rehype-katex';

export default defineConfig({
  site: 'https://puri.ac',
  output: 'static',
  adapter: cloudflare({ imageService: 'passthrough' }),
  integrations: [sitemap({
    filter: (page) => !page.match(/\/community\/(notice|general|qna|tips)\/\d/)
  })],
  markdown: {
    remarkPlugins: [remarkMath],
    rehypePlugins: [rehypeKatex]
  },
  server: { port: 4321 },
  vite: {
    resolve: {
      preserveSymlinks: true
    },
    server: {
      proxy: {
        '/puri.': 'http://localhost:8080'
      }
    }
  }
});
