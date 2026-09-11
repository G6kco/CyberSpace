# Route Reference

Last reviewed: 2026-09-10

All protected routes use `PlatformRepository` through the current `MockPlatformRepository`. “Future API” entries describe expected integration, not implemented backend behavior.

| Route | Role | Purpose | Main actions | Data requirements | Current mock service | Expected future API |
| --- | --- | --- | --- | --- | --- | --- |
| `/login` | Public | Establish a demo role session | Show password, remember selection, choose student/admin, sign in | Demo identities and session state | `AuthProvider`, `demoUsers` | `POST /auth/session`, `GET /me`, password-recovery link |
| `/unauthorized` | Public/session-aware | Explain a role mismatch | Return to the current role workspace | Current user role | `AuthProvider` | Current authenticated principal |
| `/` | Public/session-aware | Choose the correct landing page | Automatic redirect | Current user role | `AuthProvider`, `roleLanding` | Current authenticated principal |
| `/student` | Student | Student landing alias | Redirect to learning | Current user role | Route configuration | None beyond current user |
| `/student/learning` | Student | Search and filter the learning library | Search, filter, continue/open material | Material summaries and lesson completion | `listLearningMaterials` | `GET /learning-materials`, query filters, progress summary |
| `/student/learning/:materialId` | Student | Review one material and its lessons | Mark lesson complete/incomplete | Material details, prerequisites, outcomes, lessons | `listLearningMaterials`, `updateLesson` | `GET /learning-materials/{id}`, `PUT /learning-materials/{id}/lessons/{lessonId}/completion` |
| `/student/assessments` | Student | Understand imported bookings and attempt lifecycle | Search/filter, inspect, confirm start, resume | Student attempts, booking facts, eligibility, state | `listStudentAssessments`, `updateStudentAssessment` | `GET /me/assessment-attempts`, `POST /assessment-attempts/{id}/start` |
| `/student/assessments/:assessmentId` | Student | View attempt details or use an active workspace | Start/stop/reset simulated lab, navigate questions, submit answer, confirm finish | Attempt, definition, questions, answers, timer, target, environment state | `listStudentAssessments`, `listAssessmentDefinitions`, `updateStudentAssessment` | `GET /assessment-attempts/{id}`, submission endpoints, environment commands/status, finish endpoint |
| `/admin` | Administrator | Administrator landing alias | Redirect to monitoring | Current user role | Route configuration | None beyond current user |
| `/admin/monitoring` | Administrator | Operational assessment-session control | Search/filter, inspect details, approve/start, end, revoke | Sessions, booking, environment/attempt states, progress, action history | `listMonitoringSessions`, `updateMonitoringSession` | `GET /admin/student-sessions`, approval/start/end/revoke commands, audit feed |
| `/admin/assessments` | Administrator | Manage assessment definitions | Search/filter/page, create, edit, duplicate, publish/unpublish, archive, delete draft | Definition summaries and status | `listAssessmentDefinitions`, `saveAssessment`, `deleteAssessment` | Assessment CRUD and lifecycle endpoints with optimistic versioning |
| `/admin/assessments/new` | Administrator | Create a definition | Edit six sections, add questions, check YAML, save draft, publish | Blank definition defaults, allowed levels/types, YAML example | `saveAssessment` | `POST /admin/assessments`, validation endpoints |
| `/admin/assessments/:assessmentId/edit` | Administrator | Edit an existing definition | Edit/reorder/delete/add questions and subquestions, validate YAML, save/publish | Full definition, questions, evaluation rules, YAML | `listAssessmentDefinitions`, `saveAssessment` | `GET/PUT /admin/assessments/{id}`, question mutation/reorder, publish command |
| `/admin/students` | Administrator | Search and manage student access | Filter/sort/page, inspect, activate/deactivate, change eligibility | Student identity, department/year, account, eligibility, attempts, completion | `listStudents`, `updateStudent` | `GET /admin/students`, account and eligibility command endpoints |
| `/admin/students/:studentId` | Administrator | Open a student details drawer by URL | Review completion, attempts, notes placeholder, change state from list | Full student record and attempt summary | `listStudents`, `updateStudent` | `GET /admin/students/{id}`, attempts and notes endpoints |
| `/*` | Public | Explain an invalid address | Return home | None | Static component | None |

## Redirect and permission behavior

- Signed-out access to a protected route redirects to `/login` with the requested path in router state.
- After demo sign-in, the requested path is honored only if it belongs to the selected role.
- A signed-in user who opens the other role's route is sent to `/unauthorized`.
- `/student` and `/admin` never expose content directly; each redirects to its role landing page.
