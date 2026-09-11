import { BookOpen, ClipboardCheck, MonitorCog, ShieldCheck, Users } from 'lucide-react'
import type { Role } from '../types/domain'

export const navigation = {
  student: [
    { label: 'Learning Materials', to: '/student/learning', icon: BookOpen },
    { label: 'Assessments', to: '/student/assessments', icon: ClipboardCheck },
  ],
  admin: [
    { label: 'Student Monitoring', to: '/admin/monitoring', icon: MonitorCog },
    { label: 'Assessment Management', to: '/admin/assessments', icon: ShieldCheck },
    { label: 'Student Management', to: '/admin/students', icon: Users },
  ],
} satisfies Record<Role, Array<{ label: string; to: string; icon: typeof BookOpen }>>

export const roleLanding: Record<Role, string> = { student: '/student/learning', admin: '/admin/monitoring' }
