import { apiBase, apiRequest } from './api'
import type { Level, LevelInput, QuestionInput } from '../types/levels'

const base = '/api/v1/admin/levels'

// Every change answers with the level as it now stands, so the editor
// redraws from one response.
export const levelsApi = {
  list: () => apiRequest<{ levels: Level[] }>(base).then((r) => r.levels),
  get: (levelId: number) => apiRequest<Level>(`${base}/${levelId}`),
  create: (input: LevelInput) => apiRequest<Level>(base, { method: 'POST', body: input }),
  update: (levelId: number, input: LevelInput) => apiRequest<Level>(`${base}/${levelId}`, { method: 'PUT', body: input }),
  remove: (levelId: number) => apiRequest<void>(`${base}/${levelId}`, { method: 'DELETE' }),
  publish: (levelId: number) => apiRequest<Level>(`${base}/${levelId}/publish`, { method: 'POST' }),
  unpublish: (levelId: number) => apiRequest<Level>(`${base}/${levelId}/unpublish`, { method: 'POST' }),
  createQuestion: (levelId: number, input: QuestionInput) =>
    apiRequest<Level>(`${base}/${levelId}/questions`, { method: 'POST', body: input }),
  updateQuestion: (levelId: number, questionId: number, input: QuestionInput) =>
    apiRequest<Level>(`${base}/${levelId}/questions/${questionId}`, { method: 'PUT', body: input }),
  removeQuestion: (levelId: number, questionId: number) =>
    apiRequest<Level>(`${base}/${levelId}/questions/${questionId}`, { method: 'DELETE' }),
  uploadFile: (levelId: number, questionId: number, file: File) => {
    const form = new FormData()
    form.append('file', file)
    return apiRequest<Level>(`${base}/${levelId}/questions/${questionId}/files`, { method: 'POST', body: form })
  },
  removeFile: (levelId: number, questionId: number, fileId: number) =>
    apiRequest<Level>(`${base}/${levelId}/questions/${questionId}/files/${fileId}`, { method: 'DELETE' }),
  // A plain link: the browser downloads the file with the session cookie.
  fileUrl: (levelId: number, questionId: number, fileId: number) =>
    `${apiBase}${base}/${levelId}/questions/${questionId}/files/${fileId}`,
}
