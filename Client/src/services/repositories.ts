import { assessmentDefinitions, learningMaterials, monitoringSessions, studentAssessments, students } from '../mocks/data'
import type { AssessmentDefinition, LearningMaterial, MonitoringSession, StudentAssessment, StudentRecord } from '../types/domain'

const delay = (ms = 120) => new Promise((resolve) => setTimeout(resolve, ms))
const clone = <T,>(value: T): T => structuredClone(value)

export interface PlatformRepository {
  listLearningMaterials(): Promise<LearningMaterial[]>
  updateLesson(materialId: string, lessonId: string, completed: boolean): Promise<LearningMaterial>
  listStudentAssessments(): Promise<StudentAssessment[]>
  updateStudentAssessment(id: string, patch: Partial<StudentAssessment>): Promise<StudentAssessment>
  listAssessmentDefinitions(): Promise<AssessmentDefinition[]>
  saveAssessment(assessment: AssessmentDefinition): Promise<AssessmentDefinition>
  deleteAssessment(id: string): Promise<void>
  listStudents(): Promise<StudentRecord[]>
  updateStudent(id: string, patch: Partial<StudentRecord>): Promise<StudentRecord>
  listMonitoringSessions(): Promise<MonitoringSession[]>
  updateMonitoringSession(id: string, patch: Partial<MonitoringSession>): Promise<MonitoringSession>
}

class MockPlatformRepository implements PlatformRepository {
  private materials = clone(learningMaterials)
  private studentAssessmentRecords = clone(studentAssessments)
  private definitions = clone(assessmentDefinitions)
  private studentRecords = clone(students)
  private sessions = clone(monitoringSessions)

  async listLearningMaterials() { await delay(); return clone(this.materials) }
  async updateLesson(materialId: string, lessonId: string, completed: boolean) {
    await delay()
    const material = this.materials.find((item) => item.id === materialId)
    const lesson = material?.lessons.find((item) => item.id === lessonId)
    if (!material || !lesson) throw new Error('Learning material or lesson not found.')
    lesson.completed = completed
    return clone(material)
  }
  async listStudentAssessments() { await delay(); return clone(this.studentAssessmentRecords) }
  async updateStudentAssessment(id: string, patch: Partial<StudentAssessment>) {
    await delay()
    const record = this.studentAssessmentRecords.find((item) => item.id === id)
    if (!record) throw new Error('Assessment attempt not found.')
    Object.assign(record, patch)
    return clone(record)
  }
  async listAssessmentDefinitions() { await delay(); return clone(this.definitions) }
  async saveAssessment(assessment: AssessmentDefinition) {
    await delay()
    const index = this.definitions.findIndex((item) => item.id === assessment.id)
    if (index >= 0) this.definitions[index] = clone(assessment)
    else this.definitions.unshift(clone(assessment))
    return clone(assessment)
  }
  async deleteAssessment(id: string) { await delay(); this.definitions = this.definitions.filter((item) => item.id !== id) }
  async listStudents() { await delay(); return clone(this.studentRecords) }
  async updateStudent(id: string, patch: Partial<StudentRecord>) {
    await delay()
    const record = this.studentRecords.find((item) => item.id === id)
    if (!record) throw new Error('Student not found.')
    Object.assign(record, patch)
    return clone(record)
  }
  async listMonitoringSessions() { await delay(); return clone(this.sessions) }
  async updateMonitoringSession(id: string, patch: Partial<MonitoringSession>) {
    await delay()
    const record = this.sessions.find((item) => item.id === id)
    if (!record) throw new Error('Session not found.')
    Object.assign(record, patch)
    return clone(record)
  }
}

export const platformRepository: PlatformRepository = new MockPlatformRepository()
