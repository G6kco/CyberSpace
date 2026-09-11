import { chromium } from 'playwright-core'
import { mkdir, writeFile } from 'node:fs/promises'
import path from 'node:path'

const executablePath = 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe'
const baseURL = 'http://127.0.0.1:5173'
const users = {
  student: { id: 'usr-stu-01', name: 'Maya Iyer', email: 'maya.iyer@demo.cyberspace.edu', role: 'student', initials: 'MI' },
  admin: { id: 'usr-adm-01', name: 'Dr. Arjun Sen', email: 'arjun.sen@demo.cyberspace.edu', role: 'admin', initials: 'AS' },
}
const cases = [
  { name: 'login-desktop', path: '/login', viewport: { width: 1440, height: 1000 } },
  { name: 'login-mobile', path: '/login', viewport: { width: 375, height: 812 } },
  { name: 'student-learning-desktop', path: '/student/learning', role: 'student', viewport: { width: 1440, height: 1000 } },
  { name: 'student-learning-mobile', path: '/student/learning', role: 'student', viewport: { width: 375, height: 812 } },
  { name: 'student-assessments-tablet', path: '/student/assessments', role: 'student', viewport: { width: 768, height: 1024 } },
  { name: 'student-workspace-laptop', path: '/student/assessments/stu-assess-7', role: 'student', viewport: { width: 1280, height: 900 } },
  { name: 'admin-monitoring-wide', path: '/admin/monitoring', role: 'admin', viewport: { width: 1440, height: 1000 } },
  { name: 'admin-monitoring-mobile', path: '/admin/monitoring', role: 'admin', viewport: { width: 375, height: 812 } },
  { name: 'admin-assessment-editor', path: '/admin/assessments/assess-web-1/edit', role: 'admin', viewport: { width: 1440, height: 1000 } },
  { name: 'admin-students-tablet', path: '/admin/students', role: 'admin', viewport: { width: 768, height: 1024 } },
]

await mkdir('qa-artifacts', { recursive: true })
const browser = await chromium.launch({ executablePath, headless: true })
const results = []

for (const item of cases) {
  const context = await browser.newContext({ viewport: item.viewport, deviceScaleFactor: 1 })
  if (item.role) {
    await context.addInitScript(({ user }) => localStorage.setItem('cyberspace-demo-session', JSON.stringify(user)), { user: users[item.role] })
  }
  const page = await context.newPage()
  const errors = []
  page.on('console', (message) => { if (message.type() === 'error') errors.push(`console: ${message.text()}`) })
  page.on('pageerror', (error) => errors.push(`page: ${error.message}`))
  const response = await page.goto(`${baseURL}${item.path}`, { waitUntil: 'networkidle' })
  await page.screenshot({ path: path.join('qa-artifacts', `${item.name}.png`), fullPage: true })
  const layout = await page.evaluate(() => ({
    title: document.title,
    width: document.documentElement.clientWidth,
    scrollWidth: document.documentElement.scrollWidth,
    heading: document.querySelector('h1, h2')?.textContent?.trim() ?? '',
  }))
  const interactions = []
  if (item.name === 'student-learning-mobile') {
    await page.getByRole('button', { name: 'Open navigation' }).click()
    await page.screenshot({ path: path.join('qa-artifacts', 'student-mobile-drawer.png') })
    await page.locator('aside').getByRole('link', { name: 'Assessments' }).click()
    await page.getByRole('heading', { name: 'Your assessments' }).waitFor()
    interactions.push('mobile drawer opened and closed after route navigation')
  }
  if (item.name === 'admin-monitoring-wide') {
    const row = page.getByRole('row').filter({ hasText: 'Nila Joseph' })
    await row.getByRole('button', { name: 'Open actions' }).click()
    await page.getByRole('menuitem', { name: 'Approve and start' }).click()
    await page.getByRole('alertdialog', { name: 'Approve and start assessment?' }).waitFor()
    await page.screenshot({ path: path.join('qa-artifacts', 'admin-start-confirmation.png') })
    await page.keyboard.press('Escape')
    interactions.push('start confirmation opened and dismissed by keyboard')
  }
  if (item.name === 'student-workspace-laptop') {
    await page.getByRole('button', { name: 'Submit answer' }).click()
    await page.getByText('Enter an answer before submitting.').waitFor()
    interactions.push('empty answer validation displayed')
  }
  if (item.name === 'admin-students-tablet') {
    const firstActions = page.getByRole('button', { name: 'Open actions' }).first()
    await firstActions.click()
    await page.getByRole('menuitem', { name: 'Change eligibility' }).click()
    await page.getByRole('alertdialog', { name: 'Change assessment eligibility?' }).waitFor()
    await page.screenshot({ path: path.join('qa-artifacts', 'student-eligibility-confirmation.png') })
    interactions.push('eligibility confirmation opened')
  }
  results.push({ ...item, status: response?.status(), layout, interactions, errors })
  await context.close()
}

await browser.close()
await writeFile('qa-artifacts/results.json', JSON.stringify(results, null, 2))
console.log(JSON.stringify(results, null, 2))

if (results.some((item) => item.status !== 200 || item.errors.length || item.layout.scrollWidth > item.layout.width)) process.exitCode = 1
