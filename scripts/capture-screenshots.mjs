import { chromium } from "playwright"
import path from "path"
import { fileURLToPath } from "url"

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const OUT = process.env.OUT_DIR || path.join(__dirname, "../docs/screenshots")
const BASE = process.env.BASE_URL || "http://localhost:5173"
const token = process.env.TOKEN

if (!token) {
  console.error("TOKEN env var required")
  process.exit(1)
}

const browser = await chromium.launch()
const context = await browser.newContext({ viewport: { width: 1920, height: 1080 } })
const page = await context.newPage()

await page.goto(`${BASE}/register`, { waitUntil: "networkidle" })
await page.screenshot({ path: path.join(OUT, "web-register.png") })

await page.getByRole("button", { name: "一键注册" }).click()
await page.getByRole("heading", { name: "请立即保存主密码" }).waitFor()
await page.screenshot({ path: path.join(OUT, "web-password.png") })

await page.evaluate((t) => localStorage.setItem("access_token", t), token)

for (const [route, file] of [
  ["/", "web-messages.png"],
  ["/settings", "web-settings.png"],
  ["/devices", "web-devices.png"],
]) {
  await page.goto(`${BASE}${route}`, { waitUntil: "networkidle" })
  await page.screenshot({ path: path.join(OUT, file) })
}

await browser.close()
console.log("Screenshots saved to", OUT)
