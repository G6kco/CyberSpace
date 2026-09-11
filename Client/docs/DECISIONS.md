# Decision Log

## 2026-09-10 — Role-specific navigation and one shared shell

- **Decision:** Use one reusable application shell driven by centralized student/admin navigation arrays, with separate role guards around every route.
- **Reason:** The roles need different permissions and navigation, while shell behavior and visual consistency are shared.
- **Alternatives considered:** Two fully separate shells; a single navigation containing disabled cross-role items.
- **Consequences:** Shell fixes apply to both roles; permission logic remains explicit; users never see irrelevant navigation.
- **Status:** Final for this phase; open to a third role later.

## 2026-09-10 — Compact desktop sidebar and mobile drawer

- **Decision:** Use a 256px fixed desktop sidebar that collapses to 80px at laptop/desktop widths; use a modal slide-over below 1024px.
- **Reason:** Operational pages benefit from stable navigation without sacrificing workspace width. A drawer is more usable than a squeezed sidebar on touch screens.
- **Alternatives considered:** Always-expanded sidebar; top navigation; bottom mobile navigation.
- **Consequences:** The shell must close the drawer on route change and manage Escape/focus behavior. Tablet widths use the mobile trigger.
- **Status:** Final for this phase.

## 2026-09-10 — Repository boundary around deterministic mock data

- **Decision:** Pages depend on `PlatformRepository`; a cloned in-memory implementation owns all mock mutations.
- **Reason:** Backend replacement should not require rewriting page components, and deterministic seed data keeps tests repeatable.
- **Alternatives considered:** Importing mock arrays directly; a global state library; browser-storage persistence for all records.
- **Consequences:** Mutations are session-local and filters are currently client-side. An API implementation can replace the repository later.
- **Status:** Final architectural direction; interface details may evolve with the backend contract.

## 2026-09-10 — Local storage only for the demo session

- **Decision:** Persist the selected mock user, but keep domain mutations in memory.
- **Reason:** Refresh should not sign the reviewer out, while domain persistence could falsely imply a real backend.
- **Alternatives considered:** Persist all mock state; persist nothing.
- **Consequences:** Lesson and administration changes reset on full refresh; the role survives.
- **Status:** Final for this phase.

## 2026-09-10 — Responsive tables become record cards below wide breakpoints

- **Decision:** Monitoring switches to cards below 1280px; student management switches below 1024px. Assessment lists use structured rows/cards at every width.
- **Reason:** A wide operational table is not usable at 375–768px, and hiding secondary columns would remove important state.
- **Alternatives considered:** Horizontal scrolling with sticky columns; aggressively hidden columns.
- **Consequences:** Both representations exist in the DOM and share action logic. Tests select the intended wide-table row explicitly.
- **Status:** Final for this phase; real data density may tune the breakpoints.

## 2026-09-10 — Six-section assessment editor

- **Decision:** Use a persistent section navigator with one active section: basic information, level/difficulty, questions, lab configuration, evaluation, review/publish.
- **Reason:** The definition is too complex for a single long form, and the sections map to distinct administrator decisions.
- **Alternatives considered:** One scrolling form; a linear wizard that prevents revisiting sections; separate routes per section.
- **Consequences:** Draft state remains local across section changes, validation aggregates at publish time, and the form remains manageable on laptops.
- **Status:** Final for this phase.

## 2026-09-10 — Frontend-only YAML checks and readable summary

- **Decision:** Check only for required top-level markers and parse a few display values with non-executing string matching.
- **Reason:** Full schema, image policy, and provisioning validation belong to the backend; the frontend must never execute YAML.
- **Alternatives considered:** Shipping a YAML parser and schema engine; treating any text as valid.
- **Consequences:** Feedback explicitly says checks are basic, and publish readiness cannot guarantee a safe/provisionable lab.
- **Status:** Final for the frontend phase; backend schema is open.

## 2026-09-10 — TypeScript version chosen for ecosystem compatibility

- **Decision:** Pin TypeScript 5.9.3 even though npm's newest TypeScript release is 7.0.2.
- **Reason:** Current `typescript-eslint` 8.70.0 supports TypeScript versions below 6.1. Forcing an unsupported peer combination would undermine the “mutually compatible” requirement.
- **Alternatives considered:** TypeScript 7 with peer overrides; omitting lint; the separate TypeScript 6 compatibility package.
- **Consequences:** The compiler is not the numerically newest package, but the installed toolchain is supported and validation is clean.
- **Status:** Open for revision when TypeScript-ESLint supports TypeScript 7.

## 2026-09-10 — Mock monitoring is explicitly non-live

- **Decision:** Use stable timestamps and immediate local updates; label records as mock/simulated instead of polling or animating telemetry.
- **Reason:** The blueprint must not claim infrastructure that does not exist.
- **Alternatives considered:** Random progress updates; simulated polling; live-looking charts.
- **Consequences:** The UI demonstrates operations and states without suggesting real observation.
- **Status:** Final for this phase.
