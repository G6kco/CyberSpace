import { BookOpen, ClipboardCheck, Layers, MonitorCog, ShieldCheck, UserCog, Users } from 'lucide-react'
import type { Role } from '../types/domain'

export const navigation = {
  student: [
    { label: 'Learning Materials', to: '/student/learning', icon: BookOpen },
    { label: 'Assessments', to: '/student/assessments', icon: ClipboardCheck },
  ],
  admin: [
    { label: 'Assessment Monitoring', to: '/admin/monitoring', icon: MonitorCog },
    { label: 'Levels', to: '/admin/levels', icon: Layers },
    { label: 'Question Bank', to: '/admin/assessments', icon: ShieldCheck },
    { label: 'Students', to: '/admin/students', icon: Users },
    { label: 'Administrators', to: '/admin/administrators', icon: UserCog },
  ],
} satisfies Record<Role, Array<{ label: string; to: string; icon: typeof BookOpen }>>

export const roleLanding: Record<Role, string> = { student: '/student/learning', admin: '/admin/monitoring' }
