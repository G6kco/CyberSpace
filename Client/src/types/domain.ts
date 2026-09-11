export type Role = 'student' | 'admin'
export type Difficulty = 'Beginner' | 'Intermediate' | 'Advanced'
export type MaterialCategory =
  | 'Networking Fundamentals'
  | 'Linux Fundamentals'
  | 'Web Security'
  | 'Reconnaissance'
  | 'Vulnerability Assessment'
  | 'Digital Forensics'

export interface User {
  id: string
  name: string
  email: string
  role: Role
  initials: string
}

export interface Lesson {
  id: string
  title: string
  duration: number
  completed: boolean
}

export interface LearningMaterial {
  id: string
  title: string
  description: string
  category: MaterialCategory
  difficulty: Difficulty
  duration: number
  prerequisites: string[]
  outcomes: string[]
  lessons: Lesson[]
}

export type AssessmentLifecycle =
  | 'Not booked'
  | 'Booked'
  | 'Awaiting approval'
  | 'Ready'
  | 'Active'
  | 'Completed'
  | 'Revoked'
  | 'Expired'

export type AssessmentStatus = 'draft' | 'published' | 'archived'
export type AnswerType = 'flag' | 'short-text' | 'multiple-choice'

export interface Question {
  id: string
  prompt: string
  type: AnswerType
  points: number
  expectedAnswer: string
  options?: string[]
  hint?: string
  subquestions?: Question[]
}

export interface AssessmentDefinition {
  id: string
  name: string
  slug: string
  description: string
  level: string
  difficulty: Difficulty
  duration: number
  prerequisites: string[]
  outcomes: string[]
  status: AssessmentStatus
  questions: Question[]
  passingScore: number
  yaml: string
  createdAt: string
  updatedAt: string
}

export interface StudentAssessment {
  id: string
  definitionId: string
  name: string
  level: string
  scheduledAt?: string
  duration: number
  status: AssessmentLifecycle
  attempt: number
  progress: number
  eligibility: string
  targetIp?: string
  environmentState?: 'Stopped' | 'Starting' | 'Running' | 'Stopping'
  answers?: Record<string, string>
}

export interface StudentRecord {
  id: string
  name: string
  initials: string
  registerNumber: string
  email: string
  department: string
  year: number
  active: boolean
  eligible: boolean
  lastAssessment: string
  completionRate: number
  notes: string
  attempts: number
}

export interface MonitoringSession {
  id: string
  studentId: string
  studentName: string
  registerNumber: string
  email: string
  assessment: string
  level: string
  scheduledAt: string
  bookingState: 'Booked' | 'Confirmed' | 'Waitlisted'
  environmentState: 'Not provisioned' | 'Stopped' | 'Starting' | 'Running' | 'Stopped by admin'
  assessmentState: AssessmentLifecycle
  timeRemaining: number
  progress: number
  startedAt?: string
  endedAt?: string
  submittedAnswers: number
  totalAnswers: number
  actionHistory: string[]
}
