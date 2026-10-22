import { apiRequest } from './api'
import type { AdminAccount, AdminInput, ImportResult } from '../types/users'

// Accounts are created only here: sign-in never registers anyone.
export const usersApi = {
  importStudents: (file: File) => {
    const form = new FormData()
    form.append('file', file)
    return apiRequest<ImportResult>('/api/v1/admin/students/import', { method: 'POST', body: form })
  },
  admins: () => apiRequest<{ admins: AdminAccount[] }>('/api/v1/admin/admins').then((r) => r.admins),
  createAdmin: (input: AdminInput) => apiRequest<AdminAccount>('/api/v1/admin/admins', { method: 'POST', body: input }),
}
