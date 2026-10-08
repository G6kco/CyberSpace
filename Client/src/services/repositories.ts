import { learningMaterials } from '../mocks/data'
import type { LearningMaterial } from '../types/domain'

// Learning material is still served from in-memory demo data; assessments
// use the real API in services/assessments.ts.
const delay = (ms = 120) => new Promise((resolve) => setTimeout(resolve, ms))
const clone = <T,>(value: T): T => structuredClone(value)

export interface PlatformRepository {
  listLearningMaterials(): Promise<LearningMaterial[]>
  updateLesson(materialId: string, lessonId: string, completed: boolean): Promise<LearningMaterial>
}

class MockPlatformRepository implements PlatformRepository {
  private materials = clone(learningMaterials)

  async listLearningMaterials() { await delay(); return clone(this.materials) }
  async updateLesson(materialId: string, lessonId: string, completed: boolean) {
    await delay()
    const material = this.materials.find((item) => item.id === materialId)
    const lesson = material?.lessons.find((item) => item.id === lessonId)
    if (!material || !lesson) throw new Error('Learning material or lesson not found.')
    lesson.completed = completed
    return clone(material)
  }
}

export const platformRepository: PlatformRepository = new MockPlatformRepository()
