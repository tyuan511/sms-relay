import { createContext, useContext, useEffect, useState, type ReactNode } from "react"
import { applyTheme, getStoredTheme, setTheme, type Theme } from "@/lib/theme"

type ThemeContextValue = {
  theme: Theme
  setTheme: (theme: Theme) => void
  toggleTheme: () => void
}

const ThemeContext = createContext<ThemeContextValue | null>(null)

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<Theme>(() => getStoredTheme())

  useEffect(() => {
    applyTheme(theme)
  }, [theme])

  function updateTheme(next: Theme) {
    setTheme(next)
    setThemeState(next)
  }

  return (
    <ThemeContext.Provider
      value={{
        theme,
        setTheme: updateTheme,
        toggleTheme: () => updateTheme(theme === "dark" ? "light" : "dark"),
      }}
    >
      {children}
    </ThemeContext.Provider>
  )
}

export function useTheme() {
  const ctx = useContext(ThemeContext)
  if (!ctx) throw new Error("useTheme must be used within ThemeProvider")
  return ctx
}
