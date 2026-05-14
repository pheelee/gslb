import { test, expect } from '@playwright/test'

test.describe('Dashboard', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/')
  })

  test('should display dashboard with title', async ({ page }) => {
    await expect(page.locator('h1')).toContainText('GSLB Manager')
  })

  test('should show stats cards', async ({ page }) => {
    await expect(page.locator('.stat-card')).toHaveCount(4)
    await expect(page.locator('.stat-label').nth(0)).toContainText('Configurations')
    await expect(page.locator('.stat-label').nth(1)).toContainText('Total Backends')
    await expect(page.locator('.stat-label').nth(2)).toContainText('Healthy Backends')
    await expect(page.locator('.stat-label').nth(3)).toContainText('Unhealthy Backends')
  })

  test('should navigate to config list', async ({ page }) => {
    await page.click('text=Configurations')
    await expect(page).toHaveURL('/configs')
    await expect(page.locator('h1')).toContainText('Configurations')
  })

  test('should navigate to new config form', async ({ page }) => {
    await page.click('text=New Configuration')
    await expect(page).toHaveURL('/configs/new')
    await expect(page.locator('h1')).toContainText('New Configuration')
  })
})

test.describe('Configuration List', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/configs')
  })

  test('should display empty state when no configs', async ({ page }) => {
    await expect(page.locator('.empty')).toContainText('No configurations yet')
  })

  test('should have create button', async ({ page }) => {
    await expect(page.locator('text=New Configuration')).toBeVisible()
  })

  test('should navigate to create form', async ({ page }) => {
    await page.click('text=New Configuration')
    await expect(page).toHaveURL('/configs/new')
  })
})

test.describe('Create Configuration', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/configs/new')
  })

  test('should display form fields', async ({ page }) => {
    await expect(page.locator('input#name')).toBeVisible()
    await expect(page.locator('input#dns_name')).toBeVisible()
    await expect(page.locator('input#dns_ttl')).toBeVisible()
    await expect(page.locator('select#lb_method')).toBeVisible()
  })

  test('should create new configuration', async ({ page }) => {
    const testName = `Test Config ${Date.now()}`
    
    await page.fill('input#name', testName)
    await page.fill('input#dns_name', 'test.example.com')
    await page.fill('input#dns_ttl', '60')
    await page.selectOption('select#lb_method', 'round_robin')
    
    // Configure health check
    await page.selectOption('select#hc_type', 'tcp')
    await page.fill('input#hc_interval', '10')
    await page.fill('input#hc_timeout', '5')
    
    // Submit form
    await page.click('button[type="submit"]')
    
    // Should redirect to config detail
    await expect(page).toHaveURL(/\/configs\/.+/)
    await expect(page.locator('h1')).toContainText(testName)
  })

  test('should validate required fields', async ({ page }) => {
    // Try to submit without filling required fields
    await page.click('button[type="submit"]')
    
    // Should stay on the same page
    await expect(page).toHaveURL('/configs/new')
  })
})

test.describe('Configuration Detail', () => {
  test('should display config details', async ({ page }) => {
    // First create a config
    await page.goto('/configs/new')
    const testName = `Detail Test ${Date.now()}`
    
    await page.fill('input#name', testName)
    await page.fill('input#dns_name', 'detail.example.com')
    await page.fill('input#dns_ttl', '30')
    await page.selectOption('select#lb_method', 'weighted')
    await page.click('button[type="submit"]')
    
    // Should be on detail page
    await expect(page.locator('h1')).toContainText(testName)
    
    // Check details
    await expect(page.locator('.detail-card')).toContainText('detail.example.com')
    await expect(page.locator('.detail-card')).toContainText('weighted')
  })

  test('should add backend', async ({ page }) => {
    // Create config first
    await page.goto('/configs/new')
    await page.fill('input#name', `Backend Test ${Date.now()}`)
    await page.fill('input#dns_name', 'backend.example.com')
    await page.click('button[type="submit"]')
    
    // Add backend
    await page.click('text=Add Backend')
    await page.fill('input[placeholder="192.168.1.1"]', '192.168.1.100')
    await page.fill('input[type="number"]', '8080')
    await page.click('button:has-text("Add Backend")')
    
    // Backend should appear in list
    await expect(page.locator('.backend-address')).toContainText('192.168.1.100:8080')
  })
})

test.describe('Navigation', () => {
  test('should navigate between pages', async ({ page }) => {
    // Start at dashboard
    await page.goto('/')
    
    // Go to configs
    await page.click('text=Configurations')
    await expect(page).toHaveURL('/configs')
    
    // Go to new config
    await page.click('text=New Configuration')
    await expect(page).toHaveURL('/configs/new')
    
    // Go back
    await page.click('text=Back to List')
    await expect(page).toHaveURL('/configs')
    
    // Back to dashboard
    await page.goto('/')
    await expect(page).toHaveURL('/')
  })
})

test.describe('API Integration', () => {
  test('should load configs from API', async ({ page }) => {
    await page.goto('/')
    
    // Wait for data to load
    await page.waitForSelector('.stat-card')
    
    // Stats should be visible (even if 0)
    const configsCount = await page.locator('.stat-value').nth(0).textContent()
    expect(configsCount).toMatch(/^\d+$/)
  })
})
