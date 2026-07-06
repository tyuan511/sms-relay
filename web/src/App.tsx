import { Navigate, Route, Routes } from "react-router-dom"
import { isAuthenticated } from "@/lib/api"
import { Layout } from "@/components/layout/Layout"
import { LoginPage } from "@/pages/LoginPage"
import { RegisterPage } from "@/pages/RegisterPage"
import { MessagesPage } from "@/pages/MessagesPage"
import { SettingsPage } from "@/pages/SettingsPage"
import { DevicesPage } from "@/pages/DevicesPage"

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  if (!isAuthenticated()) return <Navigate to="/login" replace />
  return <>{children}</>
}

function PublicRoute({ children }: { children: React.ReactNode }) {
  if (isAuthenticated()) return <Navigate to="/" replace />
  return <>{children}</>
}

export default function App() {
  return (
    <Routes>
      <Route
        path="/login"
        element={
          <PublicRoute>
            <LoginPage />
          </PublicRoute>
        }
      />
      <Route
        path="/register"
        element={
          <PublicRoute>
            <RegisterPage />
          </PublicRoute>
        }
      />
      <Route
        element={
          <ProtectedRoute>
            <Layout />
          </ProtectedRoute>
        }
      >
        <Route index element={<MessagesPage />} />
        <Route path="settings" element={<SettingsPage />} />
        <Route path="devices" element={<DevicesPage />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
