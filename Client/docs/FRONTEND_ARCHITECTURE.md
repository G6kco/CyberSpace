# Frontend Architecture

Last reviewed: 2026-09-10

## Folder structure

```text
src/
  components/
    layout/          application shell and role navigation
    ui/              reusable interaction and feedback primitives
  features/
    auth/            authentication context
    assessments/     assessment editor tests
    learning/        learning interaction tests
    monitoring/      monitoring action tests
  lib/               formatting and optional WebMCP bridge
  mocks/             deterministic fictional seed records
  pages/
    admin/            monitoring, assessment management/editor, students
    student/          learning, material detail, assessments, workspace
  routes/             centralized router and navigation configuration
  services/           repository interface and mock implementation
  test/               shared test setup and render harness
  types/              domain and optional browser API types
```

Configuration is kept at the repository root. Product documentation is in `docs/`; browser QA automation is in `scripts/`.

## Routing

`src/routes/AppRoutes.tsx` is the central route tree. `RequireAuth` redirects signed-out users to `/login` and remembers the requested path. `RequireRole` compares the session role with the required route role and redirects mismatches to `/unauthorized`.

`src/routes/navigation.ts` is the single source for role navigation and landing routes. The shell closes mobile navigation after any location change. Detail routes remain inside the correct role guard.

## Authentication and role guarding

`AuthProvider` owns the small session state. Sign-in selects one deterministic demo user and writes it to local storage. Initial context state reads that record, allowing refresh persistence. Sign-out removes it.

This is a frontend convenience only. The future backend must issue and validate a secure session, enforce authorization for every operation, and return a current-user payload. Frontend guards must never be treated as the production security boundary.

## Component boundaries

- `AppShell` owns desktop/mobile navigation, top bar, role identity, and nested route outlet.
- Page components own query/filter/form interaction state and call the repository.
- Domain-heavy secondary surfaces are kept close to their page (session details, editor sections, student details) rather than abstracted prematurely.
- Shared UI components cover patterns used across several domains: buttons, badges, progress, search, pagination, toasts, dropdowns, loading/empty/error feedback, confirmation dialogs, and drawers.
- Confirmation dialogs and drawers restore focus and handle Escape; the drawer also traps Tab within its panel.

## State management

- React Context: current mock user only.
- Local/component state: forms, filters, selection, overlays, feedback, and temporary drafts.
- Repository singleton: in-memory mock domain mutations during the running session.
- Local storage: selected mock user only.

No global state library is needed at this scale. Backend integration may add a server-state query library later if caching, invalidation, retry, and concurrent updates justify it.

## Mock service layer

`PlatformRepository` declares the operations pages use. `MockPlatformRepository` implements them against cloned deterministic arrays, returns cloned results, and adds a short delay. This prevents components from importing seed data or mutating it directly.

An API repository should implement the same interface initially. As the backend contract matures, list arguments should move from client-side filtering into typed query objects, and mutations should return authoritative records plus version/audit metadata.

## Expected backend integration points

- Replace role selection with session establishment and `GET /me`.
- Move learning progress, assessment attempts, definitions, students, and session records to API repositories.
- Make booking records a read-only integration response.
- Make remaining time, grading, environment state, and administrative action history server-authoritative.
- Send sensitive actions with idempotency keys and record actor/reason/timestamp.
- Validate YAML with a backend schema and security policy before publishing.
- Provision lab environments asynchronously and report a typed provisioning state.
- Add event delivery or polling only when real telemetry exists.

The frontend-driven request/response draft is in `docs/BACKEND_CONTRACT_DRAFT.md`.

## Design token strategy

`src/styles.css` defines Tailwind v4 theme tokens for primary royal blue, dark hover blue, light blue, application background, surface text, secondary text, borders, status colors, and the sans-serif stack. JSX uses semantic token utilities (`bg-primary`, `text-secondary`, `border-border`) instead of scattering hex values.

Base form, page heading, eyebrow, and surface patterns are defined once in the component layer. Status-specific Tailwind colors remain centralized inside `StatusBadge`.

## Optional WebMCP surface

`WebMcpBridge` feature-detects `document.modelContext` and registers one real, non-speculative tool: updating lesson completion through the same repository as the visible interface. It validates inputs, returns concise data, and dispatches a refresh event. Browsers without the proposed API ignore it cleanly.
