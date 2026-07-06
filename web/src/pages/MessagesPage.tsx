import { useCallback, useEffect, useState } from "react"
import { ChevronLeft, ChevronRight, Inbox, RefreshCw, Wifi } from "lucide-react"
import { messagesApi, subscribeMessages, type Message } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardTitle } from "@/components/ui/card"
import { CardListSkeleton } from "@/components/ui/loading-state"
import { cn } from "@/lib/utils"

const PAGE_SIZE = 20

function formatTime(iso: string) {
  return new Date(iso).toLocaleString("zh-CN", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  })
}

export function MessagesPage() {
  const [messages, setMessages] = useState<Message[]>([])
  const [page, setPage] = useState(1)
  const [hasMore, setHasMore] = useState(false)
  const [loading, setLoading] = useState(true)
  const [connected, setConnected] = useState(false)

  const load = useCallback(async (targetPage: number) => {
    setLoading(true)
    try {
      const offset = (targetPage - 1) * PAGE_SIZE
      const data = await messagesApi.list(PAGE_SIZE, offset)
      setMessages(data)
      setPage(targetPage)
      setHasMore(data.length === PAGE_SIZE)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load(1)
    const unsub = subscribeMessages(() => {
      setConnected(true)
      void load(1)
    })
    setConnected(true)
    return unsub
  }, [load])

  const canPrev = page > 1
  const canNext = hasMore

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between gap-4">
        <div>
          <p className="font-mono text-xs text-muted-foreground mb-1">Messages</p>
          <h1 className="text-2xl font-semibold tracking-tight">短信列表</h1>
          <p className="text-body text-sm mt-1">实时同步来自 Android 设备的转发短信</p>
        </div>
        <div className="flex items-center gap-2">
          <span
            className={cn(
              "inline-flex items-center gap-1.5 text-xs px-2.5 py-1 rounded-full border",
              connected
                ? "border-border bg-muted text-foreground"
                : "border-border text-muted-foreground",
            )}
          >
            <Wifi className="h-3 w-3" />
            {connected ? "实时连接" : "连接中"}
          </span>
          <Button variant="outline" size="sm" onClick={() => load(page)} disabled={loading}>
            <RefreshCw className={cn("h-3.5 w-3.5", loading && "animate-spin")} />
            刷新
          </Button>
        </div>
      </div>

      {loading && messages.length === 0 ? (
        <CardListSkeleton count={4} />
      ) : messages.length === 0 ? (
        <Card className="border-dashed shadow-none">
          <CardContent className="flex flex-col items-center justify-center py-16 text-center">
            <Inbox className="h-10 w-10 text-muted-foreground/40 mb-4" />
            <CardTitle className="text-lg mb-2">暂无短信</CardTitle>
            <CardDescription className="max-w-sm">
              在 Android 应用中配置服务器地址和主密码后，收到的短信将自动出现在这里。
            </CardDescription>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-3">
          <div
            className={cn(
              "rounded-md border border-border bg-card shadow-elevated divide-y divide-border overflow-hidden",
              loading && "opacity-60",
            )}
          >
            {messages.map((msg) => (
              <article key={msg.id} className="px-3 py-2.5 hover:bg-muted/40 transition-colors">
                <div className="flex items-baseline justify-between gap-3 mb-1">
                  <span className="font-mono text-xs font-medium truncate">{msg.sender}</span>
                  <time className="font-mono text-[11px] text-muted-foreground shrink-0">
                    {formatTime(msg.received_at)}
                  </time>
                </div>
                <p className="text-sm leading-snug whitespace-pre-wrap break-words text-body line-clamp-4">
                  {msg.body}
                </p>
              </article>
            ))}
          </div>

          <div className="flex items-center justify-between gap-4">
            <p className="text-xs text-muted-foreground">
              第 {page} 页 · 每页 {PAGE_SIZE} 条
            </p>
            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                disabled={!canPrev || loading}
                onClick={() => load(page - 1)}
              >
                <ChevronLeft className="h-3.5 w-3.5" />
                上一页
              </Button>
              <Button
                variant="outline"
                size="sm"
                disabled={!canNext || loading}
                onClick={() => load(page + 1)}
              >
                下一页
                <ChevronRight className="h-3.5 w-3.5" />
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
