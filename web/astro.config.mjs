import { defineConfig } from 'astro/config';

// Static corpus site: reads ../evidence/, renders to dist/, deploys
// to tutti.dunn.dev via Cloudflare Pages.
export default defineConfig({
  site: 'https://tutti.dunn.dev',
  output: 'static',
  trailingSlash: 'always',
  build: {
    format: 'directory',
  },
});
