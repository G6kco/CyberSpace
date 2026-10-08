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
