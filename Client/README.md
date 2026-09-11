# CyberSpace Frontend Blueprint

CyberSpace is a responsive frontend prototype for college cybersecurity learning and assessment. It provides separate student and administrator workspaces, a simulated isolated-lab workflow, deterministic fictional data, and a replaceable repository layer. It does not provision containers or execute assessment configuration.

## Technology stack

- Vite 8.2.2
- React 19.2.8 and React DOM 19.2.8
- TypeScript 5.9.3 (the newest release supported by the current TypeScript-ESLint peer range)
- Tailwind CSS 4.3.3 through the official Vite plugin
- React Router DOM 7.18.3
- Lucide React 1.42.0
- Vitest 5 and Testing Library

The versions are pinned in `package.json` and `package-lock.json` for repeatable installs.

## Setup

Prerequisites: Node.js 20.19+, 22.12+, or a newer supported release and npm.

```bash
npm install
npm run dev
```

Open the local URL printed by Vite. Any non-empty email and password can be used on the mock login screen.

## Development commands

```bash
npm run dev        # start the Vite development server
npm run lint       # run ESLint
npm run typecheck  # run strict TypeScript checks
npm test           # run the focused interaction suite once
npm run test:watch # run tests in watch mode
npm run build      # type-check and create the production bundle
```

The optional visual QA script uses the local Google Chrome installation configured in `scripts/visual-qa.mjs`:

```bash
node scripts/visual-qa.mjs
```

## Demo roles

The login page contains Student and Administrator role controls. The fictional demo identities are:

- Student: Maya Iyer — `maya.iyer@demo.cyberspace.edu`
- Administrator: Dr. Arjun Sen — `arjun.sen@demo.cyberspace.edu`

The password is not validated in this phase. The selected mock session is stored under `cyberspace-demo-session` in browser local storage so refreshes preserve the role.

## Route overview

Public:

- `/login`
- `/unauthorized`
- unmatched routes show a not-found page

Student:

- `/student/learning`
- `/student/learning/:materialId`
- `/student/assessments`
- `/student/assessments/:assessmentId`

Administrator:

- `/admin/monitoring`
- `/admin/assessments`
- `/admin/assessments/new`
- `/admin/assessments/:assessmentId/edit`
- `/admin/students`
- `/admin/students/:studentId`

`/`, `/student`, and `/admin` redirect to the correct role landing page. See `docs/ROUTES.md` for the full route contract.

## Mock data and future backend connection

Page components depend on the `PlatformRepository` interface in `src/services/repositories.ts`. The current `MockPlatformRepository` keeps deterministic seed records in memory and adds a small artificial delay so loading states are visible. A future API implementation can satisfy the same interface with HTTP requests, then be injected without rewriting page components.

The frontend-driven request/response draft is documented in `docs/BACKEND_CONTRACT_DRAFT.md`. Domain models are centralized in `src/types/domain.ts`, while seed records remain in `src/mocks/data.ts`.

## Current limitations

- Authentication, authorization, and passwords are simulated; route guards are a UX blueprint, not a security boundary.
- Mock repository mutations last only for the current browser/runtime session. Only the selected role survives a refresh.
- Booking is read-only imported data; no external booking integration exists.
- Timers and monitoring records are representative, not live telemetry.
- YAML checks are intentionally shallow and never execute YAML or Docker.
- Lab provisioning, target IP allocation, container control, answer grading, real-time events, auditing, and notifications require backend services.
- Forgot-password and notification controls are visual placeholders.
- WebMCP lesson-completion registration is feature-detected; it cannot be validated where the browser does not expose `document.modelContext`.

## Documentation

- `docs/PROJECT_CONTEXT.md` — stable product scope and terminology
- `docs/FRONTEND_ARCHITECTURE.md` — implementation structure and integration boundaries
- `docs/ROUTES.md` — per-route responsibilities and data needs
- `docs/DECISIONS.md` — lightweight architectural decision log
- `docs/FRONTEND_PROGRESS.md` — implementation and validation handoff
- `docs/BACKEND_CONTRACT_DRAFT.md` — frontend-driven API discussion draft
