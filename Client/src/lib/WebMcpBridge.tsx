import { useEffect } from 'react'
import { platformRepository } from '../services/repositories'

function parseInput(input: unknown) {
  if (!input || typeof input !== 'object') throw new Error('Input must be an object.')
  const value = input as Record<string, unknown>
  if (typeof value.materialId !== 'string' || typeof value.lessonId !== 'string' || typeof value.completed !== 'boolean') throw new Error('materialId, lessonId, and completed are required.')
  return { materialId: value.materialId, lessonId: value.lessonId, completed: value.completed }
}

export function WebMcpBridge() {
  useEffect(() => {
    const context = document.modelContext
    if (!context?.registerTool) return
    const lifecycle = new AbortController()
    void Promise.resolve(context.registerTool({
      name: 'update_learning_lesson_completion',
      title: 'Update lesson completion',
      description: 'Mark one learning lesson complete or incomplete in the same mock repository used by the visible student interface.',
      inputSchema: {
        type: 'object',
        properties: {
          materialId: { type: 'string' },
          lessonId: { type: 'string' },
          completed: { type: 'boolean' },
        },
        required: ['materialId', 'lessonId', 'completed'],
        additionalProperties: false,
      },
      annotations: { readOnlyHint: false, untrustedContentHint: false },
      async execute(input) {
        const value = parseInput(input)
        const material = await platformRepository.updateLesson(value.materialId, value.lessonId, value.completed)
        document.dispatchEvent(new CustomEvent('cyberspace:learning-updated', { detail: { materialId: material.id } }))
        return { materialId: material.id, lessonId: value.lessonId, completed: value.completed }
      },
    }, { signal: lifecycle.signal })).catch(() => undefined)
    return () => lifecycle.abort()
  }, [])
  return null
}
