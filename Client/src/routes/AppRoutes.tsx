import { Navigate, Route, Routes, useLocation } from 'react-router-dom'
import { AppShell } from '../components/layout/AppShell'
import { useAuth } from '../features/auth/AuthContext'
import { LoginPage } from '../pages/LoginPage'
import { NotFoundPage, UnauthorizedPage } from '../pages/StatePages'
import { LearningPage } from '../pages/student/LearningPage'
import { MaterialDetailPage } from '../pages/student/MaterialDetailPage'
import { AssessmentsPage } from '../pages/student/AssessmentsPage'
import { AssessmentWorkspacePage } from '../pages/student/AssessmentWorkspacePage'
import { MonitoringPage } from '../pages/admin/MonitoringPage'
import { StudentsPage } from '../pages/admin/StudentsPage'
import { AssessmentManagementPage } from '../pages/admin/AssessmentManagementPage'
import { AssessmentEditorPage } from '../pages/admin/AssessmentEditorPage'
import { roleLanding } from './navigation'
import type { ReactNode } from 'react'
import type { Role } from '../types/domain'

// Until GET /api/v1/me answers, the signed-in state is unknown. Rendering a
// placeholder keeps the guards from redirecting a signed-in user to /login on
// every page load.
function SessionCheck() {
  return <div className="grid min-h-screen place-items-center text-secondary">Checking your session…</div>
}

function RequireAuth({ children }: { children: ReactNode }) {
  const { user, loading } = useAuth()
  const location = useLocation()
  if (loading) return <SessionCheck />
  return user ? children : <Navigate to="/login" state={{ from: location.pathname }} replace />
}

function RequireRole({ role, children }: { role: Role; children: ReactNode }) {
  const { user, loading } = useAuth()
  if (loading) return <SessionCheck />
  if (!user) return <Navigate to="/login" replace />
  return user.role === role ? children : <Navigate to="/unauthorized" replace />
}

function HomeRedirect() {
  const { user, loading } = useAuth()
  if (loading) return <SessionCheck />
  return <Navigate to={user ? roleLanding[user.role] : '/login'} replace />
}

export function AppRoutes() {
  return <Routes>
    <Route path="/login" element={<LoginPage />} />
    <Route path="/unauthorized" element={<UnauthorizedPage />} />
    <Route path="/" element={<HomeRedirect />} />
    <Route element={<RequireAuth><AppShell /></RequireAuth>}>
      <Route path="student" element={<RequireRole role="student"><Navigate to="/student/learning" replace /></RequireRole>} />
      <Route path="student/learning" element={<RequireRole role="student"><LearningPage /></RequireRole>} />
      <Route path="student/learning/:materialId" element={<RequireRole role="student"><MaterialDetailPage /></RequireRole>} />
      <Route path="student/assessments" element={<RequireRole role="student"><AssessmentsPage /></RequireRole>} />
      <Route path="student/assessments/:assessmentId" element={<RequireRole role="student"><AssessmentWorkspacePage /></RequireRole>} />
      <Route path="admin" element={<RequireRole role="admin"><Navigate to="/admin/monitoring" replace /></RequireRole>} />
      <Route path="admin/monitoring" element={<RequireRole role="admin"><MonitoringPage /></RequireRole>} />
      <Route path="admin/assessments" element={<RequireRole role="admin"><AssessmentManagementPage /></RequireRole>} />
      <Route path="admin/assessments/new" element={<RequireRole role="admin"><AssessmentEditorPage /></RequireRole>} />
      <Route path="admin/assessments/:assessmentId/edit" element={<RequireRole role="admin"><AssessmentEditorPage /></RequireRole>} />
      <Route path="admin/students" element={<RequireRole role="admin"><StudentsPage /></RequireRole>} />
      <Route path="admin/students/:studentId" element={<RequireRole role="admin"><StudentsPage /></RequireRole>} />
    </Route>
    <Route path="*" element={<NotFoundPage />} />
  </Routes>
}
