import { buildVercelDeploymentConfig } from './scripts/vercel-config.mjs'

export const config = buildVercelDeploymentConfig(process.env.VITE_API_BASE_URL, {
  requireApiBaseUrl: true,
})
