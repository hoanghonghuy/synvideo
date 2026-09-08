import { expect, test } from '@playwright/test'
import { execFile } from 'node:child_process'
import { mkdtemp, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { promisify } from 'node:util'

const execFileAsync = promisify(execFile)
const PNG_2X2 = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAIAAAACCAYAAABytg0kAAAAEUlEQVR4nGP4z8DwH4QZYAwAR8oH+WdZbrcAAAAASUVORK5CYII=',
  'base64',
)

test('creator can snapshot, render a real durable MP4, refresh, and download it', async ({ page, request }) => {
  test.setTimeout(90_000)
  const runId = process.env.SYNVIDEO_E2E_RUN_ID ?? `${Date.now()}`
  const title = `Render acceptance ${runId}`

  await page.goto('/projects/new')
  await page.locator('input[name="title"]').fill(title)
  await page.locator('textarea[name="description"]').fill('Fresh-local real FFmpeg render acceptance.')
  await page.locator('form.project-form button[type="submit"]').click()
  await expect(page).toHaveURL(/\/projects\/[0-9a-f-]{36}$/i)
  const projectID = page.url().match(/\/projects\/([0-9a-f-]{36})$/i)?.[1]
  expect(projectID).toBeTruthy()

  await execFileAsync('go', ['run', './cmd/e2e-render-fixture', projectID!], {
    cwd: join(process.cwd(), 'apps/api'),
    env: process.env,
  })

  const upload = await request.post(`/api/v1/projects/${projectID}/media-assets`, {
    multipart: {
      file: { name: 'acceptance.png', mimeType: 'image/png', buffer: PNG_2X2 },
    },
  })
  expect(upload.status(), await upload.text()).toBe(201)
  const asset = (await upload.json()) as { id: string }

  const bindingResponse = await request.put(
    `/api/v1/projects/${projectID}/scene-plans/1/scenes/intro/primary-visual`,
    { data: { asset_id: asset.id } },
  )
  expect(bindingResponse.status(), await bindingResponse.text()).toBe(200)
  const binding = (await bindingResponse.json()) as { binding: { id: string } }

  const compositionResponse = await request.post(`/api/v1/projects/${projectID}/scene-editor`, {
    data: {
      scene_plan_version: 1,
      scenes: [
        {
          id: crypto.randomUUID(),
          scene_key: 'intro',
          visual: { asset_id: asset.id, binding_id: binding.binding.id },
          duration_ms: 2000,
          visual_treatment: {
            fit: 'contain',
            position_x: 0,
            position_y: 0,
            scale: 1,
            mute_video: true,
          },
          transition_out: { kind: 'cut', duration_ms: 0 },
        },
      ],
    },
  })
  expect(compositionResponse.status(), await compositionResponse.text()).toBe(201)

  await page.goto(`/projects/${projectID}/scene-editor`)
  await expect(page.getByRole('heading', { level: 1, name: 'Scene Editor' })).toBeVisible()
  await expect(page.getByText('CURRENT', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Snapshot & render MP4' }).click()

  await expect(page.getByText(/Render succeeded/)).toBeVisible({ timeout: 60_000 })
  const renderedSummary = page.getByText(/MP4 ready/)
  await expect(renderedSummary).toContainText(/\d+×\d+/)

  await page.reload({ waitUntil: 'networkidle' })
  await expect(page.getByText(/Render succeeded/)).toBeVisible({ timeout: 10_000 })

  const downloadLink = page.getByRole('link', { name: 'Download rendered MP4' })
  await expect(downloadLink).toBeVisible()
  const response = await request.get(await downloadLink.getAttribute('href') as string)
  expect(response.status()).toBe(200)
  expect(response.headers()['content-type']).toContain('video/mp4')
  const mp4 = await response.body()
  expect(mp4.byteLength).toBeGreaterThan(0)

  const workDir = await mkdtemp(join(tmpdir(), 'synvideo-render-e2e-'))
  try {
    const outputPath = join(workDir, 'render.mp4')
    await writeFile(outputPath, mp4)
    const { stdout } = await execFileAsync('ffprobe', [
      '-v', 'error',
      '-select_streams', 'v:0',
      '-show_entries', 'stream=width,height:format=duration',
      '-of', 'json',
      outputPath,
    ])
    const probe = JSON.parse(stdout) as {
      streams?: Array<{ width?: number; height?: number }>
      format?: { duration?: string }
    }
    expect(probe.streams?.[0]?.width).toBeGreaterThan(0)
    expect(probe.streams?.[0]?.height).toBeGreaterThan(0)
    const duration = Number(probe.format?.duration ?? 0)
    expect(duration).toBeGreaterThanOrEqual(1.8)
    expect(duration).toBeLessThanOrEqual(2.2)
  } finally {
    await rm(workDir, { recursive: true, force: true })
  }
})
