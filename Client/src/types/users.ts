// A row of an uploaded student list that was not imported. Row 0 means the
// whole sheet was skipped.
export interface ImportIssue {
  sheet: string
  row: number
  registerNumber: string
  email: string
  reason: string
}

export interface ImportResult {
  created: number
  updated: number
  unchanged: number
  skipped: ImportIssue[]
}

export interface AdminAccount {
  id: number
  publicId: string
  name: string
  email: string
  status: string
  lastLoginAt: string | null
  createdAt: string
}

export interface AdminInput {
  name: string
  email: string
}
