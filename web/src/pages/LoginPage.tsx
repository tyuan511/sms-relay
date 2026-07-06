import { useState } from "react"
import { Link, useNavigate } from "react-router-dom"
import { KeyRound, Loader2 } from "lucide-react"
import { authApi, setToken } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { ThemeToggle } from "@/components/ThemeToggle"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"

export function LoginPage() {
  const navigate = useNavigate()
  const [password, setPassword] = useState("")
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState("")

  async function handleLogin(e: React.FormEvent) {
    e.preventDefault()
    setLoading(true)
    setError("")
    try {
      const res = await authApi.login(password.trim())
      setToken(res.access_token)
      navigate("/")
    } catch (e) {
      setError(e instanceof Error ? e.message : "登录失败")
    } finally {
      setLoading(false)
    }
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
            <KeyRound className="h-5 w-5" />
          </div>
          <CardTitle className="text-xl tracking-tight">登录</CardTitle>
          <CardDescription>输入您的 32 位主密码以访问控制台</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleLogin} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="password">主密码</Label>
              <Input
                id="password"
                type="password"
                placeholder="粘贴您保存的主密码"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                autoComplete="current-password"
                required
              />
            </div>
            {error && (
              <div className="rounded-sm border border-destructive/20 bg-destructive/5 text-destructive text-sm p-3">
                {error}
              </div>
            )}
            <Button className="w-full" size="lg" type="submit" disabled={loading || !password.trim()}>
              {loading && <Loader2 className="h-4 w-4 animate-spin" />}
              {loading ? "验证中..." : "登录"}
            </Button>
            <p className="text-center text-sm text-body">
              还没有账号？{" "}
              <Link to="/register" className="text-accent hover:underline">
                一键注册
              </Link>
            </p>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
