import { useState } from "react"
import { Link, useNavigate } from "react-router-dom"
import { AlertTriangle, Copy, Check, Loader2, Sparkles } from "lucide-react"
import { authApi, setToken } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { ThemeToggle } from "@/components/ThemeToggle"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"

export function RegisterPage() {
  const navigate = useNavigate()
  const [loading, setLoading] = useState(false)
  const [password, setPassword] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState("")

  async function handleRegister() {
    setLoading(true)
    setError("")
    try {
      const res = await authApi.register()
      setToken(res.access_token)
      setPassword(res.master_password || "")
    } catch (e) {
      setError(e instanceof Error ? e.message : "注册失败")
    } finally {
      setLoading(false)
    }
  }

  async function copyPassword() {
    if (!password) return
    await navigator.clipboard.writeText(password)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  if (password) {
    return (
      <div className="min-h-screen flex items-center justify-center p-6 relative bg-background">
        <ThemeToggle
          className="absolute top-4 right-4 text-body"
          showLabel={false}
        />
        <Card className="w-full max-w-lg bg-muted border-border shadow-elevated">
          <CardHeader className="text-center pb-2">
            <div className="mx-auto mb-3 h-10 w-10 rounded-sm border border-destructive/30 bg-destructive/5 flex items-center justify-center">
              <AlertTriangle className="h-5 w-5 text-destructive" />
            </div>
            <CardTitle className="text-xl tracking-tight">请立即保存主密码</CardTitle>
            <CardDescription>
              这是您唯一的登录凭证，系统不会再次显示。丢失后将无法找回账号。
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="rounded-sm border border-border bg-card p-4">
              <div className="font-mono text-xs text-muted-foreground mb-2">
                您的主密码（32 位）
              </div>
              <div className="font-mono text-sm break-all leading-relaxed select-all text-foreground">
                {password}
              </div>
            </div>

            <Button variant="outline" className="w-full" onClick={copyPassword}>
              {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
              {copied ? "已复制" : "复制密码"}
            </Button>

            <label className="flex items-start gap-3 cursor-pointer rounded-sm border border-border bg-card p-4 hover:bg-muted transition-colors">
              <input
                type="checkbox"
                checked={saved}
                onChange={(e) => setSaved(e.target.checked)}
                className="mt-0.5 accent-foreground"
              />
              <span className="text-sm text-body">
                我已将主密码安全保存（密码管理器、离线备份等），理解丢失密码将无法登录。
              </span>
            </label>

            <Button
              className="w-full"
              size="lg"
              disabled={!saved}
              onClick={() => navigate("/")}
            >
              进入控制台
            </Button>
          </CardContent>
        </Card>
      </div>
    )
  }

  return (
    <div className="min-h-screen flex items-center justify-center p-6 relative bg-background">
      <ThemeToggle
        className="absolute top-4 right-4 text-body"
        showLabel={false}
      />
      <Card className="w-full max-w-md bg-muted border-border shadow-elevated">
        <CardHeader className="text-center pb-2">
          <div className="mx-auto mb-3 h-10 w-10 rounded-sm border border-border bg-card flex items-center justify-center">
            <Sparkles className="h-5 w-5" />
          </div>
          <CardTitle className="text-xl tracking-tight">创建账号</CardTitle>
          <CardDescription>
            无需填写任何信息，系统将自动生成 32 位主密码作为您的唯一凭证。
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {error && (
            <div className="rounded-sm border border-destructive/20 bg-destructive/5 text-destructive text-sm p-3">
              {error}
            </div>
          )}
          <Button className="w-full" size="lg" onClick={handleRegister} disabled={loading}>
            {loading && <Loader2 className="h-4 w-4 animate-spin" />}
            {loading ? "生成中..." : "一键注册"}
          </Button>
          <p className="text-center text-sm text-body">
            已有账号？{" "}
            <Link to="/login" className="text-accent hover:underline">
              使用主密码登录
            </Link>
          </p>
        </CardContent>
      </Card>
    </div>
  )
}
