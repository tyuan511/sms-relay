import { MessageSquare, Settings, Smartphone, LogOut } from "lucide-react"
import { NavLink, Outlet, useNavigate } from "react-router-dom"
import { clearToken } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { ThemeToggle } from "@/components/ThemeToggle"
import { cn } from "@/lib/utils"

const nav = [
  { to: "/", icon: MessageSquare, label: "短信" },
  { to: "/settings", icon: Settings, label: "通知渠道" },
  { to: "/devices", icon: Smartphone, label: "设备" },
]

export function Layout() {
  const navigate = useNavigate()

  return (
    <div className="min-h-screen flex bg-background">
      <aside className="w-60 shrink-0 border-r border-border bg-card flex flex-col">
        <div className="h-14 flex items-center px-4 border-b border-border">
          <div className="font-semibold text-sm tracking-tight">SMS Relay</div>
        </div>

        <nav className="flex flex-col gap-0.5 p-2 flex-1">
          {nav.map(({ to, icon: Icon, label }) => (
            <NavLink
              key={to}
              to={to}
              end={to === "/"}
              className={({ isActive }) =>
                cn(
                  "relative flex items-center gap-2.5 rounded-sm px-3 py-2 text-sm transition-colors",
                  isActive
                    ? "bg-muted text-foreground font-medium"
                    : "text-body hover:bg-muted hover:text-foreground",
                )
              }
            >
              {({ isActive }) => (
                <>
                  {isActive && (
                    <span className="absolute left-0 top-1/2 -translate-y-1/2 h-4 w-0.5 rounded-full bg-foreground" />
                  )}
                  <Icon className="h-4 w-4 shrink-0" />
                  {label}
                </>
              )}
            </NavLink>
          ))}
        </nav>

        <div className="p-2 border-t border-border space-y-0.5">
          <ThemeToggle className="w-full justify-start text-body h-9" />
          <Button
            variant="ghost"
            className="w-full justify-start text-body h-9"
            onClick={() => {
              clearToken()
              navigate("/login")
            }}
          >
            <LogOut className="h-4 w-4" />
            退出登录
          </Button>
        </div>
      </aside>

      <main className="flex-1 overflow-auto">
        <div className="mx-auto max-w-4xl p-6 lg:p-8">
          <Outlet />
        </div>
      </main>
    </div>
  )
}
