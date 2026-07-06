import { Moon, Sun } from "lucide-react"
import { Button } from "@/components/ui/button"
import { useTheme } from "@/components/ThemeProvider"

type ThemeToggleProps = {
  className?: string
  showLabel?: boolean
}

export function ThemeToggle({ className, showLabel = true }: ThemeToggleProps) {
  const { theme, toggleTheme } = useTheme()
  const isDark = theme === "dark"

  return (
    <Button
      type="button"
      variant="ghost"
      className={className}
      onClick={toggleTheme}
      aria-label={isDark ? "切换到浅色模式" : "切换到深色模式"}
    >
      {isDark ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
      {showLabel && (isDark ? "浅色模式" : "深色模式")}
    </Button>
  )
}
