const API_BASE = import.meta.env.VITE_API_URL || "/api/v1"

function getToken(): string | null {
  return localStorage.getItem("access_token")
}

export function setToken(token: string) {
  localStorage.setItem("access_token", token)
}

export function clearToken() {
  localStorage.removeItem("access_token")
}

export function isAuthenticated() {
  return !!getToken()
}

async function api<T>(path: string, options: RequestInit = {}): Promise<T> {
  const token = getToken()
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...options.headers,
    },
  })

  if (res.status === 401) {
    clearToken()
    window.location.href = "/login"
    throw new Error("Unauthorized")
  }

  if (!res.ok) {
    const err = await res.json().catch(() => ({ detail: "Request failed" }))
    throw new Error(err.detail || "Request failed")
  }

  if (res.status === 204) return undefined as T
  return res.json()
}

export interface AuthResponse {
  access_token: string
  token_type: string
  master_password?: string
}

export interface Message {
  id: string
  sender: string
  body: string
  received_at: string
  created_at: string
}

export interface Destination {
  id: string
  name: string
  platform: string
  enabled: boolean
  created_at: string
  bot_username?: string
  chat_id?: string
}

export interface TelegramLinkSession {
  link_id: string
  bot_username: string
  start_url: string
  expires_at: string
}

export interface TelegramLinkStatus {
  status: "pending" | "linked" | "expired" | "failed"
  bot_username?: string
  start_url?: string
  destination?: Destination
  error?: string
}

export interface Device {
  id: string
  name: string
  last_seen_at: string | null
  online: boolean
  created_at: string
}

export const authApi = {
  register: () =>
    api<AuthResponse>("/auth/register", { method: "POST", body: "{}" }),
  login: (master_password: string) =>
    api<AuthResponse>("/auth/login", {
      method: "POST",
      body: JSON.stringify({ master_password }),
    }),
}

export const messagesApi = {
  list: (limit = 50, offset = 0) =>
    api<Message[]>(`/messages?limit=${limit}&offset=${offset}`),
}

export const destinationsApi = {
  list: () => api<Destination[]>("/destinations"),
  create: (data: {
    name: string
    platform: string
    config: { bot_token: string; chat_id: string }
  }) =>
    api<Destination>("/destinations", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  startTelegramLink: (data: { name: string; bot_token: string }) =>
    api<TelegramLinkSession>("/destinations/telegram/link", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  getTelegramLinkStatus: (linkId: string) =>
    api<TelegramLinkStatus>(`/destinations/telegram/link/${linkId}`),
  update: (
    id: string,
    data: Partial<{
      name: string
      enabled: boolean
      config: { bot_token: string; chat_id: string }
    }>,
  ) =>
    api<Destination>(`/destinations/${id}`, {
      method: "PATCH",
      body: JSON.stringify(data),
    }),
  delete: (id: string) =>
    api<void>(`/destinations/${id}`, { method: "DELETE" }),
}

export async function fetchDestinationAvatar(id: string): Promise<string | null> {
  const token = getToken()
  if (!token) return null
  const res = await fetch(`${API_BASE}/destinations/${id}/avatar`, {
    headers: { Authorization: `Bearer ${token}` },
  })
  if (!res.ok) return null
  const blob = await res.blob()
  return URL.createObjectURL(blob)
}

export const devicesApi = {
  list: () => api<Device[]>("/devices"),
}

export function subscribeMessages(onEvent: (data: { type: string; id?: string }) => void) {
  const token = getToken()
  if (!token) return () => {}

  const url = `${API_BASE}/messages/stream`
  const controller = new AbortController()

  fetch(url, {
    headers: { Authorization: `Bearer ${token}` },
    signal: controller.signal,
  }).then(async (res) => {
    if (!res.ok || !res.body) return
    const reader = res.body.getReader()
    const decoder = new TextDecoder()
    let buffer = ""

    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })
      const parts = buffer.split("\n\n")
      buffer = parts.pop() || ""
      for (const part of parts) {
        if (part.startsWith("event: message")) {
          const dataLine = part.split("\n").find((l) => l.startsWith("data: "))
          if (dataLine) {
            try {
              onEvent(JSON.parse(dataLine.slice(6)))
            } catch {
              /* ignore */
            }
          }
        }
      }
    }
  })

  return () => controller.abort()
}
