# Backend Contract Draft

Status: **frontend-driven discussion draft** — no backend support is claimed.

Last reviewed: 2026-09-10

## Contract conventions

- JSON over HTTPS under a versioned base such as `/api/v1`.
- ISO 8601 timestamps with offsets or UTC `Z`; the frontend formats them in the user's locale.
- Stable opaque string IDs.
- Cursor pagination for large collections; page/limit can be used for an initial implementation.
- Structured error bodies: `{ code, message, fieldErrors?, requestId? }`.
- Sensitive commands accept an idempotency key and return the authoritative updated resource.
- Authorization is enforced on the server for every operation.
- Mutable administrative resources should expose a version or ETag to detect conflicting edits.

## Authentication and current user

### Entity

```ts
type CurrentUser = {
  id: string
  name: string
  email: string
  initials: string
  role: 'student' | 'admin'
  permissions: string[]
}
```

### Draft operations

- `POST /auth/session` — establish a session from the college identity flow.
- `DELETE /auth/session` — sign out.
- `GET /me` — return the current user and permissions.
- Password recovery and identity-provider details remain a separate authentication decision.

## Learning materials

### Entities

```ts
type LearningMaterialSummary = {
  id: string
  title: string
  description: string
  category: string
  difficulty: 'Beginner' | 'Intermediate' | 'Advanced'
  estimatedMinutes: number
  lessonCount: number
  completedLessonCount: number
  completionPercent: number
}

type LearningMaterialDetail = LearningMaterialSummary & {
  prerequisites: string[]
  learningOutcomes: string[]
  lessons: Array<{ id: string; title: string; estimatedMinutes: number; completed: boolean }>
}
```

### Draft operations

- `GET /learning-materials?q=&category=&difficulty=&completion=&cursor=`
- `GET /learning-materials/{materialId}`
- `PUT /learning-materials/{materialId}/lessons/{lessonId}/completion` with `{ completed: boolean }`

Completion writes should be idempotent and return updated material progress.

## Assessment definitions

### Entity

```ts
type AssessmentDefinition = {
  id: string
  slug: string
  name: string
  description: string
  level: string
  difficulty: 'Beginner' | 'Intermediate' | 'Advanced'
  durationMinutes: number
  prerequisites: string[]
  learningOutcomes: string[]
  status: 'draft' | 'published' | 'archived'
  questions: Question[]
  evaluation: { passingPercent: number }
  labConfiguration: { yaml: string; validation?: YamlValidationResult }
  createdAt: string
  updatedAt: string
  version: number
}
```

### Draft operations

- `GET /admin/assessments?q=&status=&difficulty=&cursor=`
- `POST /admin/assessments`
- `GET /admin/assessments/{assessmentId}`
- `PUT /admin/assessments/{assessmentId}`
- `POST /admin/assessments/{assessmentId}/duplicate`
- `POST /admin/assessments/{assessmentId}/publish`
- `POST /admin/assessments/{assessmentId}/unpublish`
- `POST /admin/assessments/{assessmentId}/archive`
- `DELETE /admin/assessments/{assessmentId}` — server rejects non-draft definitions.

Publishing should fail with structured validation errors if questions, evaluation rules, or lab configuration are invalid.

## Questions and submissions

### Entities

```ts
type Question = {
  id: string
  order: number
  prompt: string
  type: 'flag' | 'short-text' | 'multiple-choice' | string
  points: number
  choices?: Array<{ id: string; label: string }>
  expectedAnswer?: string // admin payload only
  hint?: string
  subquestions: Question[]
}

type Submission = {
  questionId: string
  answer: string
  status: 'pending' | 'accepted' | 'rejected'
  pointsAwarded?: number
  submittedAt: string
  attemptsUsed: number
}
```

### Draft operations

- Question writes may be part of the full assessment `PUT`, or use nested CRUD endpoints if concurrent editing is required.
- `PUT /admin/assessments/{id}/questions/order` with `{ questionIds: string[] }` if reorder is separate.
- `POST /assessment-attempts/{attemptId}/submissions` with `{ questionId, answer }`.
- `GET /assessment-attempts/{attemptId}/submissions`.

Student payloads must never expose expected answers. The backend owns normalization, attempt limits, grading, and partial-credit rules.

## Student assessment attempts

### Entity

```ts
type AssessmentAttempt = {
  id: string
  assessment: { id: string; name: string; level: string }
  booking: { externalId?: string; scheduledAt?: string; state: string; slotLabel?: string }
  status: 'not_booked' | 'booked' | 'awaiting_approval' | 'ready' | 'active' | 'completed' | 'revoked' | 'expired'
  attemptNumber: number
  durationMinutes: number
  authoritativeEndsAt?: string
  completionPercent: number
  eligibility: { eligible: boolean; reason: string }
  environment?: LabProvisioningStatus
}
```

### Draft operations

- `GET /me/assessment-attempts?status=&cursor=`
- `GET /assessment-attempts/{attemptId}`
- `POST /assessment-attempts/{attemptId}/start` — succeeds only inside an approved window.
- `POST /assessment-attempts/{attemptId}/finish`.

The backend calculates remaining time from authoritative timestamps and rejects submissions after completion, revocation, or expiry.

## Student sessions and administrative controls

### Entity

```ts
type StudentSession = {
  id: string
  student: { id: string; name: string; registerNumber: string; email: string }
  assessment: { id: string; name: string; level: string }
  booking: { state: string; scheduledAt: string }
  attemptStatus: AssessmentAttempt['status']
  environment: LabProvisioningStatus
  startedAt?: string
  endedAt?: string
  completionPercent: number
  submissionSummary: { submitted: number; total: number }
  actions: AdministrativeAction[]
}

type AdministrativeAction = {
  id: string
  action: 'approved' | 'started' | 'ended' | 'revoked'
  actor: { id: string; name: string }
  reason?: string
  createdAt: string
}
```

### Draft operations

- `GET /admin/student-sessions?q=&assessmentId=&level=&bookingState=&attemptState=&from=&to=&cursor=`
- `GET /admin/student-sessions/{sessionId}`
- `POST /admin/student-sessions/{sessionId}/approve-and-start`
- `POST /admin/student-sessions/{sessionId}/end` with `{ reason?: string }`
- `POST /admin/student-sessions/{sessionId}/revoke` with `{ reason?: string }`

Commands must be idempotent, permission-checked, audited, and return the updated session. Conflicting terminal states should return a typed `409` response.

## Students and eligibility

### Entity

```ts
type Student = {
  id: string
  name: string
  registerNumber: string
  email: string
  department: string
  academicYear: number
  accountStatus: 'active' | 'inactive'
  eligibility: { eligible: boolean; reason?: string; updatedAt?: string; updatedBy?: string }
  completionPercent: number
  lastAssessment?: { id: string; name: string; status: string; completedAt?: string }
  attemptCount: number
  administrativeNotes?: string
}
```

### Draft operations

- `GET /admin/students?q=&department=&year=&accountStatus=&eligible=&sort=&cursor=`
- `GET /admin/students/{studentId}`
- `POST /admin/students/{studentId}/activate` with `{ reason?: string }`
- `POST /admin/students/{studentId}/deactivate` with `{ reason?: string }`
- `PUT /admin/students/{studentId}/eligibility` with `{ eligible: boolean, reason?: string }`
- `GET /admin/students/{studentId}/attempts?cursor=`
- `PUT /admin/students/{studentId}/notes` — only if policy permits storing notes.

Eligibility may need to become assessment-specific; this is an open product decision.

## YAML configuration validation

### Entity

```ts
type YamlValidationResult = {
  valid: boolean
  normalizedSummary?: {
    image: string
    services: Array<{ name: string; internalPort: number; protocol: string }>
    networkMode: 'isolated'
    cpuLimit: string
    memoryLimit: string
    healthCheck?: string
    startupTimeoutSeconds: number
  }
  errors: Array<{ path?: string; line?: number; code: string; message: string }>
  warnings: Array<{ path?: string; line?: number; code: string; message: string }>
}
```

### Draft operations

- `POST /admin/lab-configurations/validate` with `{ yaml: string }`.

Validation should parse with safe limits, reject unknown/disallowed fields where policy requires, enforce isolated networking, allow-list images/services/environment variables, cap CPU/memory/timeouts, and never provision during validation.

## Lab provisioning status

### Entity

```ts
type LabProvisioningStatus = {
  state: 'not_provisioned' | 'queued' | 'starting' | 'running' | 'stopping' | 'stopped' | 'failed' | 'revoked'
  targetIp?: string
  services?: Array<{ name: string; protocol: string; port?: number }>
  requestedAt?: string
  readyAt?: string
  stoppedAt?: string
  failure?: { code: string; message: string }
  version: number
}
```

### Draft operations

- `POST /assessment-attempts/{attemptId}/environment/start`
- `POST /assessment-attempts/{attemptId}/environment/stop`
- `POST /assessment-attempts/{attemptId}/environment/reset`
- `GET /assessment-attempts/{attemptId}/environment`

Provisioning is asynchronous. The initial command may return `202 Accepted`; the frontend can poll a status URL initially and move to server-sent events or another event channel only when real infrastructure exists. Target addressing must never expose infrastructure outside the authorized isolated network.
