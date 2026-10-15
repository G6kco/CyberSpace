// Types of the level builder API (Server/internal/levels). Expected answers
// are write-only: the server stores them as keyed digests and never sends
// them back.

export type LevelStatus = 'draft' | 'published'

export interface LevelAnswer {
  id: number
  prompt: string
  marks: number
}

export interface LevelFile {
  id: number
  name: string
  size: number
  present: boolean
}

export interface LevelQuestion {
  id: number
  code: string
  prompt: string
  image: string
  marks: number
  answers: LevelAnswer[]
  files: LevelFile[]
}

export interface Level {
  id: number
  sequence: number
  name: string
  title: string
  instructions: string
  durationMinutes: number
  questionsPerAttempt: number
  passPercent: number
  status: LevelStatus
  // False for Level 1, which was imported and is read-only here.
  built: boolean
  questionCount: number
  attempts: number
  questions?: LevelQuestion[]
}

export interface LevelInput {
  name: string
  title: string
  instructions: string
  durationMinutes: number
  questionsPerAttempt: number
  passPercent: number
}

export interface AnswerInput {
  // Set when editing an existing answer; answer may then be left empty to
  // keep the stored one.
  id?: number
  prompt: string
  answer: string
  marks: number
}

export interface QuestionInput {
  prompt: string
  image: string
  answers: AnswerInput[]
}
