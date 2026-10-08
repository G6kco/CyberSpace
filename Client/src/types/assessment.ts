// Types of the assessment API (Server/internal/assessments). Expected
// answers never appear here: the server never sends them.

export type StudentStatus = 'eligible' | 'cooldown' | 'in_progress' | 'completed' | 'unavailable'
export type AttemptStatus = 'ready' | 'starting' | 'in_progress' | 'submitted' | 'expired' | 'revoked' | 'failed' | 'evaluated'
export type AttemptResult = 'pending' | 'passed' | 'failed'
export type Pool = 'wireshark' | 'nmap'
export type LabStatus = 'queued' | 'provisioning' | 'running' | 'stopping' | 'stopped' | 'failed'

export interface AssessmentInfo {
  level: string
  levelNumber: number
  title: string
  durationSeconds: number
  passScore: number
  totalPoints: number
  instructions: string
}

export interface AttemptSummary {
  id: number
  level: string
  title: string
  status: AttemptStatus
  result: AttemptResult
  score: number
  maxScore: number
  passScore: number
  startedAt: string | null
  deadlineAt: string | null
  endedAt: string | null
  reason?: string
}

export interface StudentOverview {
  status: StudentStatus
  assessment: AssessmentInfo | null
  cooldownUntil: string | null
  attendanceMarked: boolean
  inLobby: boolean
  activeAttemptId: number | null
  history: AttemptSummary[]
  serverTime: string
}

export interface FlagSlot {
  id: number
  prompt: string
  points: number
  solved: boolean
  submissions: number
}

export interface DealtQuestion {
  id: number
  code: string
  pool: Pool
  difficulty: 'easy' | 'hard'
  prompt: string
  points: number
  score: number
  hasPcap: boolean
  flags: FlagSlot[]
}

export interface LabView {
  status: LabStatus
  ip?: string
}

export interface AttemptView extends AttemptSummary {
  questions: DealtQuestion[] | null
  lab: LabView | null
  serverTime: string
}

export interface SubmitResult {
  correct: boolean
  points: number
  score: number
}

export interface StudentRef {
  id: string
  name: string
  email: string
  registerNumber: string
  status: string
}

export interface LobbyEntry extends StudentRef {
  level: string
  online: boolean
  lastSeen: string | null
  attendanceMarked: boolean
  activeAttemptId: number | null
  eligibility: StudentStatus
}

export interface ActionResult {
  studentId: string
  ok: boolean
  attemptId?: number
  error?: string
  warning?: string
}

export interface AdminAttempt extends AttemptSummary {
  student: StudentRef
  lab: LabView | null
  questions: DealtQuestion[]
}

export interface BankQuestion {
  id: number
  code: string
  pool: Pool
  difficulty: 'easy' | 'hard'
  prompt: string
  points: number
  flagPrompts: string[]
  pcapFile?: string
  pcapPresent: boolean
  image?: string
  timesDealt: number
}

export interface BankAssessment extends AssessmentInfo {
  questions: BankQuestion[]
}

export interface StudentRecord extends StudentRef {
  levelsPassed: number
  lastAttempt: AttemptSummary | null
  cooldownUntil: string | null
}
