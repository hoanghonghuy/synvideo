import { expect, test } from '@playwright/test'

test('creator can persist a project and recover it after a full reload', async ({ page }) => {
  const runId = process.env.SYNVIDEO_E2E_RUN_ID ?? `${Date.now()}`
  const title = `E2E project ${runId}`

  await page.goto('/projects/new')
  await page.locator('input[name="title"]').fill(title)
  await page.locator('textarea[name="description"]').fill('Created by the isolated SynVideo E2E acceptance harness.')
  await page.locator('form.project-form button[type="submit"]').click()

  await expect(page).toHaveURL(/\/projects\/[0-9a-f-]{36}$/i)
  await expect(page.getByRole('heading', { level: 1, name: title })).toBeVisible()
  await expect(page.locator('input[name="title"]')).toHaveValue(title)

  await page.reload({ waitUntil: 'networkidle' })

  await expect(page.getByRole('heading', { level: 1, name: title })).toBeVisible()
  await expect(page.locator('input[name="title"]')).toHaveValue(title)
})
