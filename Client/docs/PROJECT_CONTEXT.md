# Project Context

Last reviewed: 2026-09-10

## Product goal

CyberSpace is a college learning and assessment platform for authorized cybersecurity education. It should let students prepare through structured material and complete scheduled practical assessments while administrators control eligibility, approval, assessment definitions, and student records. The product is inspired by the workflow flexibility of established cyber-learning platforms without copying their branding or interface.

## Roles

### Student

Students can access only Learning Materials and Assessments. They can search and filter learning content, update lesson progress, review imported assessment bookings, start approved attempts, interact with simulated lab controls, submit mock answers, and finish an assessment.

### Administrator

Administrators can access only Student Monitoring, Assessment Management, and Student Management. They can inspect session records, approve/start/end/revoke attempts, maintain assessment definitions and questions, review YAML lab intent, and change student account or eligibility state.

Role routes are mutually exclusive. A route guard presents a clear access-denied page when a signed-in user opens another role's route.

## Confirmed requirements

- Vite, React, TypeScript, Tailwind CSS, React Router, and Lucide React.
- Separate protected layouts, navigation, data, permissions, and workflows for students and administrators.
- A persistent mock session using local storage.
- Deterministic fictional college data behind an explicit repository interface.
- Responsive desktop sidebar, compact state, and accessible mobile drawer.
- Shared feedback patterns: toast, confirmation dialog, empty/loading/error states, status badges, search, filters, pagination, dropdowns, and details drawers.
- Student learning library, material details, progress, and lesson completion.
- Read-only display of external booking details.
- All required assessment lifecycle states and an active attempt workspace.
- Administrator monitoring controls with confirmation and immediate mock updates.
- Assessment create/edit/duplicate/archive/delete-draft/publish workflows.
- Extensible flag, short-text, and multiple-choice questions, with subquestions and reordering.
- YAML editing, example/reset, non-executing frontend checks, and a readable environment summary.
- Student search/filter/sort, account state, eligibility, completion, attempts, and notes placeholder.
- Focused tests, production build, responsive browser inspection, and written project context.

## Assessment lifecycle

1. **Not booked** — no external booking record exists; the UI shows eligibility only.
2. **Booked** — booking data has been imported, but the slot or access may not be confirmed.
3. **Awaiting approval** — an administrator must review and approve the attempt.
4. **Ready** — the attempt is approved and can be started in the scheduled window.
5. **Active** — the student workspace is open; the mock lab may be started or stopped.
6. **Completed** — the attempt has been finished and its environment is stopped.
7. **Revoked** — an administrator removed access and stopped the environment.
8. **Expired** — the booking or authorized window ended without a usable attempt.

State transitions that start, end, revoke, publish, archive, delete, or materially change access require explicit confirmation and visible feedback.

## Terminology

- **Learning material:** A structured course-like item containing lessons, prerequisites, outcomes, and progress.
- **Assessment definition:** Administrator-authored template containing metadata, questions, evaluation rules, and lab configuration intent.
- **Assessment attempt:** A student's scheduled or completed instance of an assessment definition.
- **Student session:** Administrator-facing operational record for one assessment attempt.
- **Lab environment:** Intended isolated container-based target. All controls are simulations in the frontend phase.
- **Booking:** Scheduling information supplied by an external system. CyberSpace does not create bookings.
- **Approval:** Administrative authorization that allows an eligible booked attempt to become active.
- **Eligibility:** Student-level permission to receive assessment approval; it is distinct from booking and account state.
- **YAML configuration:** Declarative lab intent that will later be validated and provisioned by backend services.

## Design principles

- Calm campus operations console: white working surfaces on a slate background with royal-blue action hierarchy.
- Dense enough for operational work without turning every field into a card.
- Tables on wide screens and readable record cards on narrow screens.
- 8–12px corner system, restrained borders and shadows, no gradients or decorative effects.
- Status always has text in addition to color.
- The primary action and current state remain visually dominant.
- Professional, concise language with explicit simulated/mock labeling where live infrastructure does not exist.
- Keyboard-visible focus, semantic controls, labeled inputs, focus-managed overlays, and specific destructive labels.

## Current scope

The repository contains a polished frontend blueprint with mock authentication, route authorization, deterministic data, service abstractions, all requested role pages, state-changing interactions, responsive layouts, and test/QA tooling. It is designed to guide backend integration and product review.

## Explicitly out of scope

- Production authentication, password recovery, identity providers, or server-enforced authorization.
- Backend APIs, databases, durable audit storage, and real-time monitoring.
- Docker execution, image pulling, network creation, target IP allocation, or shell/tool execution.
- YAML execution or full schema/security validation.
- External booking creation or modification.
- Real attack execution, offensive tooling, or commands against a target.
- Production grading, anti-cheat, proctoring, telemetry, notifications, and email.
- Instructor role beyond the two confirmed roles.

## Assumptions open for confirmation

- Administrator currently represents instructors and platform operators; a separate instructor role may be needed later.
- Levels use generic labels (`Level 1`, `Level 2`, `Level 3`) pending the college's academic mapping.
- A single passing percentage applies to the whole assessment; per-question partial credit policy is not yet defined.
- Booking data is treated as authoritative and read-only in this application.
- Eligibility is a simple student-level boolean; future rules may be assessment-specific.
- Expected flags/answers are shown in the administrative editor. A production design should define who may view or export them.
- The timer is display-only in the blueprint. The backend should own authoritative remaining time.
- Administrative reasons are optional in the mock UI; policy may require them for revoke/end/account changes.
