import { useEffect, useRef, useState } from "react"
import { Bot, Plus, Trash2, ExternalLink, Loader2 } from "lucide-react"
import { destinationsApi, fetchDestinationAvatar, type Destination } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { DestinationListSkeleton } from "@/components/ui/loading-state"

type FormStep = "idle" | "linking" | "manual"

function DestinationAvatar({ id, username }: { id: string; username?: string }) {
  const [src, setSrc] = useState<string | null>(null)

  useEffect(() => {
    let objectUrl: string | null = null
    fetchDestinationAvatar(id).then((url) => {
      objectUrl = url
      setSrc(url)
    })
    return () => {
      if (objectUrl) URL.revokeObjectURL(objectUrl)
    }
  }, [id])

  if (src) {
    return (
      <img
        src={src}
        alt={username ? `@${username}` : "Telegram bot"}
        className="h-9 w-9 rounded-sm border border-border object-cover"
      />
    )
  }

  return (
    <div className="h-9 w-9 rounded-sm border border-border bg-muted flex items-center justify-center">
      <Bot className="h-4 w-4 text-accent" />
    </div>
  )
}

export function SettingsPage() {
  const [destinations, setDestinations] = useState<Destination[]>([])
  const [showForm, setShowForm] = useState(false)
  const [formStep, setFormStep] = useState<FormStep>("idle")
  const [name, setName] = useState("我的 Telegram")
  const [botToken, setBotToken] = useState("")
  const [chatId, setChatId] = useState("")
  const [startURL, setStartURL] = useState("")
  const [botUsername, setBotUsername] = useState("")
  const [loading, setLoading] = useState(false)
  const [initialLoading, setInitialLoading] = useState(true)
  const [error, setError] = useState("")
  const pollRef = useRef<number | null>(null)

  async function load() {
    try {
      setDestinations(await destinationsApi.list())
    } finally {
      setInitialLoading(false)
    }
  }

  useEffect(() => {
    load()
    return () => {
      if (pollRef.current) window.clearInterval(pollRef.current)
    }
  }, [])

  function resetForm() {
    if (pollRef.current) {
      window.clearInterval(pollRef.current)
      pollRef.current = null
    }
    setShowForm(false)
    setFormStep("idle")
    setBotToken("")
    setChatId("")
    setStartURL("")
    setBotUsername("")
    setError("")
  }

  async function handleStartLink(e: React.FormEvent) {
    e.preventDefault()
    setLoading(true)
    setError("")
    try {
      const session = await destinationsApi.startTelegramLink({ name, bot_token: botToken })
      setStartURL(session.start_url)
      setBotUsername(session.bot_username)
      setFormStep("linking")
      pollRef.current = window.setInterval(() => {
        void pollLinkStatus(session.link_id)
      }, 2000)
      void pollLinkStatus(session.link_id)
    } catch (e) {
      setError(e instanceof Error ? e.message : "连接失败")
    } finally {
      setLoading(false)
    }
  }

  async function pollLinkStatus(linkId: string) {
    try {
      const status = await destinationsApi.getTelegramLinkStatus(linkId)
      if (status.error && status.status === "pending") {
        setError(status.error)
      }
      if (status.status === "linked") {
        if (pollRef.current) {
          window.clearInterval(pollRef.current)
          pollRef.current = null
        }
        resetForm()
        await load()
        return
      }
      if (status.status === "expired" || status.status === "failed") {
        if (pollRef.current) {
          window.clearInterval(pollRef.current)
          pollRef.current = null
        }
        setError(status.error || (status.status === "expired" ? "连接超时，请重试" : "连接失败"))
        setFormStep("idle")
      }
    } catch (e) {
      if (pollRef.current) {
        window.clearInterval(pollRef.current)
        pollRef.current = null
      }
      setError(e instanceof Error ? e.message : "连接失败")
      setFormStep("idle")
    }
  }

  async function handleManualCreate(e: React.FormEvent) {
    e.preventDefault()
    setLoading(true)
    setError("")
    try {
      await destinationsApi.create({
        name,
        platform: "telegram",
        config: { bot_token: botToken, chat_id: chatId },
      })
      resetForm()
      await load()
    } catch (e) {
      setError(e instanceof Error ? e.message : "创建失败")
    } finally {
      setLoading(false)
    }
  }

  async function toggleEnabled(dest: Destination) {
    await destinationsApi.update(dest.id, { enabled: !dest.enabled })
    await load()
  }

  async function handleDelete(id: string) {
    if (!confirm("确定删除此通知渠道？")) return
    await destinationsApi.delete(id)
    await load()
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <p className="font-mono text-xs text-muted-foreground mb-1">Destinations</p>
          <h1 className="text-2xl font-semibold tracking-tight">通知渠道</h1>
          <p className="text-body text-sm mt-1">配置 Telegram 机器人接收转发的短信</p>
        </div>
        {!showForm && (
          <Button onClick={() => setShowForm(true)}>
            <Plus className="h-4 w-4" />
            添加 Telegram
          </Button>
        )}
      </div>

      {showForm && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Bot className="h-4 w-4" />
              新建 Telegram 通知
            </CardTitle>
            <CardDescription>
              {formStep === "linking"
                ? "在 Telegram 中打开下方链接完成绑定"
                : "填写 Bot Token，点击连接后在 Telegram 中确认即可"}
            </CardDescription>
          </CardHeader>
          <CardContent>
            {formStep === "linking" ? (
              <div className="space-y-4">
                <div className="rounded-md border border-border bg-muted p-4 space-y-3">
                  <p className="text-sm">
                    1. 点击下方按钮打开 Telegram，向{" "}
                    <code className="font-mono text-xs text-accent">@{botUsername}</code> 发送绑定请求
                  </p>
                  <p className="text-sm text-muted-foreground">
                    2. 在 Telegram 中点击「Start / 开始」（直接发 /start 也可以）
                  </p>
                  <Button asChild className="w-full">
                    <a href={startURL} target="_blank" rel="noreferrer">
                      <ExternalLink className="h-4 w-4" />
                      打开 Telegram 绑定
                    </a>
                  </Button>
                </div>
                <div className="flex items-center justify-center gap-2 text-sm text-muted-foreground">
                  <Loader2 className="h-4 w-4 animate-spin" />
                  等待 Telegram 确认...
                </div>
                {error && (
                  <div className="rounded-sm border border-destructive/20 bg-destructive/5 text-destructive text-sm p-3">{error}</div>
                )}
                <Button type="button" variant="outline" onClick={resetForm}>
                  取消
                </Button>
              </div>
            ) : formStep === "manual" ? (
              <form onSubmit={handleManualCreate} className="space-y-4">
                <div className="space-y-2">
                  <Label htmlFor="name">名称</Label>
                  <Input id="name" value={name} onChange={(e) => setName(e.target.value)} required />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="token">Bot Token</Label>
                  <Input
                    id="token"
                    placeholder="123456789:ABCdefGHI..."
                    value={botToken}
                    onChange={(e) => setBotToken(e.target.value)}
                    required
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="chatId">Chat ID</Label>
                  <Input
                    id="chatId"
                    placeholder="-1001234567890"
                    value={chatId}
                    onChange={(e) => setChatId(e.target.value)}
                    required
                  />
                </div>
                {error && (
                  <div className="rounded-sm border border-destructive/20 bg-destructive/5 text-destructive text-sm p-3">{error}</div>
                )}
                <div className="flex gap-3">
                  <Button type="button" variant="outline" onClick={() => setFormStep("idle")}>
                    返回
                  </Button>
                  <Button type="submit" disabled={loading}>
                    {loading && <Loader2 className="h-4 w-4 animate-spin" />}
                    {loading ? "保存中..." : "保存"}
                  </Button>
                </div>
              </form>
            ) : (
              <form onSubmit={handleStartLink} className="space-y-4">
                <div className="space-y-2">
                  <Label htmlFor="name">名称</Label>
                  <Input id="name" value={name} onChange={(e) => setName(e.target.value)} required />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="token">Bot Token</Label>
                  <Input
                    id="token"
                    placeholder="123456789:ABCdefGHI..."
                    value={botToken}
                    onChange={(e) => setBotToken(e.target.value)}
                    required
                  />
                </div>

                <details className="rounded-md border border-border p-4 text-sm text-body">
                  <summary className="cursor-pointer font-medium text-foreground flex items-center gap-2">
                    <ExternalLink className="h-4 w-4" />
                    如何获取 Bot Token？
                  </summary>
                  <ol className="mt-3 space-y-2 list-decimal list-inside">
                    <li>
                      在 Telegram 搜索 <code className="font-mono text-xs text-accent">@BotFather</code>，发送{" "}
                      <code className="font-mono text-xs">/newbot</code> 创建机器人
                    </li>
                    <li>复制 BotFather 返回的 Token，粘贴到上方</li>
                    <li>点击「连接 Telegram」，在弹出的对话中点击开始即可自动完成绑定</li>
                  </ol>
                </details>

                {error && (
                  <div className="rounded-sm border border-destructive/20 bg-destructive/5 text-destructive text-sm p-3">{error}</div>
                )}

                <div className="flex flex-wrap gap-3">
                  <Button type="button" variant="outline" onClick={resetForm}>
                    取消
                  </Button>
                  <Button type="submit" disabled={loading}>
                    {loading && <Loader2 className="h-4 w-4 animate-spin" />}
                    {loading ? "验证中..." : "连接 Telegram"}
                  </Button>
                  <Button type="button" variant="ghost" onClick={() => setFormStep("manual")}>
                    手动填写 Chat ID
                  </Button>
                </div>
              </form>
            )}
          </CardContent>
        </Card>
      )}

      <div className="space-y-3">
        {initialLoading ? (
          <DestinationListSkeleton count={2} />
        ) : destinations.length === 0 && !showForm ? (
          <Card className="border-dashed shadow-none">
            <CardContent className="py-12 text-center text-body">
              尚未配置通知渠道，点击上方按钮添加 Telegram 机器人
            </CardContent>
          </Card>
        ) : null}
        {!initialLoading &&
          destinations.map((dest) => (
          <Card key={dest.id}>
            <CardContent className="flex items-center justify-between py-4">
              <div className="flex items-center gap-3">
                <DestinationAvatar id={dest.id} username={dest.bot_username} />
                <div>
                  <div className="font-medium">{dest.name}</div>
                  <div className="text-xs text-muted-foreground">
                    {dest.bot_username ? `@${dest.bot_username}` : "Telegram"}
                    {dest.chat_id ? ` · chat_id: ${dest.chat_id}` : ""}
                  </div>
                </div>
              </div>
              <div className="flex items-center gap-4">
                <div className="flex items-center gap-2">
                  <span className="text-xs text-muted-foreground">
                    {dest.enabled ? "已启用" : "已禁用"}
                  </span>
                  <Switch checked={dest.enabled} onCheckedChange={() => toggleEnabled(dest)} />
                </div>
                <Button variant="ghost" size="icon" onClick={() => handleDelete(dest.id)}>
                  <Trash2 className="h-4 w-4 text-muted-foreground hover:text-destructive" />
                </Button>
              </div>
            </CardContent>
          </Card>
          ))}
      </div>
    </div>
  )
}
