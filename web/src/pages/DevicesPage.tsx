import { useCallback, useEffect, useState } from "react"
import { RefreshCw, Smartphone, Circle } from "lucide-react"
import { devicesApi, type Device } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardTitle } from "@/components/ui/card"
import { CardListSkeleton } from "@/components/ui/loading-state"
import { cn } from "@/lib/utils"

function formatTime(iso: string | null) {
  if (!iso) return "从未上线"
  const d = new Date(iso)
  const diff = Date.now() - d.getTime()
  if (diff < 5 * 60 * 1000) return "刚刚在线"
  return d.toLocaleString("zh-CN")
}

export function DevicesPage() {
  const [devices, setDevices] = useState<Device[]>([])
  const [loading, setLoading] = useState(true)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      setDevices(await devicesApi.list())
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between gap-4">
        <div>
          <p className="font-mono text-xs text-muted-foreground mb-1">Devices</p>
          <h1 className="text-2xl font-semibold tracking-tight">设备</h1>
          <p className="text-body text-sm mt-1">
            通过 Android 应用连接后自动注册，使用主密码鉴权
          </p>
        </div>
        <Button variant="outline" size="sm" onClick={load} disabled={loading}>
          <RefreshCw className={cn("h-3.5 w-3.5", loading && "animate-spin")} />
          刷新
        </Button>
      </div>

      {loading && devices.length === 0 ? (
        <CardListSkeleton count={2} />
      ) : devices.length === 0 ? (
        <Card className="border-dashed shadow-none">
          <CardContent className="flex flex-col items-center py-12 text-center">
            <Smartphone className="h-10 w-10 text-muted-foreground/40 mb-4" />
            <CardTitle className="text-lg mb-2">暂无设备</CardTitle>
            <CardDescription className="max-w-sm">
              在 Android 应用中填写服务器地址和主密码，首次上报短信时将自动注册设备。
            </CardDescription>
          </CardContent>
        </Card>
      ) : (
        <div className="rounded-md border border-border bg-card shadow-elevated divide-y divide-border overflow-hidden">
          {devices.map((device) => (
            <article
              key={device.id}
              className="flex items-center justify-between gap-3 px-3 py-2.5 hover:bg-muted/40 transition-colors"
            >
              <div className="flex items-center gap-2 min-w-0">
                <Smartphone className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
                <div className="min-w-0">
                  <p className="text-sm font-medium truncate">{device.name}</p>
                  <p className="text-xs text-body truncate">
                    最后活跃 · {formatTime(device.last_seen_at)}
                  </p>
                </div>
              </div>
              <span
                className={cn(
                  "inline-flex items-center gap-1 shrink-0 text-[11px] font-mono px-2 py-0.5 rounded-full border",
                  device.online
                    ? "border-border bg-muted text-foreground"
                    : "border-transparent text-muted-foreground",
                )}
              >
                <Circle className={cn("h-1.5 w-1.5 fill-current", device.online && "text-accent")} />
                {device.online ? "在线" : "离线"}
              </span>
            </article>
          ))}
        </div>
      )}
    </div>
  )
}
