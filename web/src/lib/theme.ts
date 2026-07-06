export type Theme = "light" | "dark"

const STORAGE_KEY = "sms-relay-theme"

export function getStoredTheme(): Theme {
  const stored = localStorage.getItem(STORAGE_KEY)
  return stored === "light" || stored === "dark" ? stored : "light"
}

export function applyTheme(theme: Theme) {
  document.documentElement.classList.toggle("dark", theme === "dark")
}

export function setTheme(theme: Theme) {
  localStorage.setItem(STORAGE_KEY, theme)
  applyTheme(theme)
}
