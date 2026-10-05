# CyberSpace

CyberSpace is a college platform for cybersecurity learning and practical assessment. It gives students a structured place to study security topics and take scheduled, approved exercises in controlled lab environments. College staff use it to manage student access, assessment content, approvals, and attempts.

## Who uses it

- **Students** browse learning materials, track lesson progress, view imported booking information, and work through assessments they are eligible and approved to take.
- **Administrators** manage student accounts and eligibility, create and maintain assessments, review attempts, and control access to practical exercises.

## How it works

Learning materials prepare students for practical work. Assessment bookings come from an external source and are treated as read-only in CyberSpace. An administrator can review an eligible booking and approve access to an assessment attempt. The product is designed to record the attempt's progress and completion and, eventually, to connect it to an isolated lab environment.

CyberSpace is intended for authorized college training. Access checks, assessment state, grading, and lab operations must be enforced by the server. Student-facing APIs must not reveal expected answers or validation rules. Lab configuration describes intended environments; reviewing configuration must not execute it or provision infrastructure.

## Project state

The React and TypeScript client is a responsive, interactive prototype. Its student and administrator workflows currently use deterministic mock data, and its login and route checks are demonstrations rather than security controls.

The Go server has its startup, configuration, MySQL, migration, and HTTP foundations. Its database schema includes users, authentication sessions, learning, assessments, attempts, labs, and audit records. Google sign-in and session handling are being developed: pieces of the identity flow, persistence, handlers, and middleware have been added, and the end-to-end server and frontend connection remains in progress.

The client uses Vite, React, TypeScript, and Tailwind CSS. The API is written in Go with Gin and MySQL. The frontend repository layer is intended to make it possible to replace mock data with API calls incrementally.

## Development direction

The next architectural milestone is to complete and verify authentication, connect the current-user endpoint, and prove the API pattern with a small read-only learning-material flow. Real lab orchestration should follow after the identity, authorization, persistence, and API patterns are established.
