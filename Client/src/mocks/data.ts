import type { AssessmentDefinition, LearningMaterial, MonitoringSession, StudentAssessment, StudentRecord, User } from '../types/domain'

export const demoUsers: Record<'student' | 'admin', User> = {
  student: { id: 'usr-stu-01', name: 'Maya Iyer', email: 'maya.iyer@demo.cyberspace.edu', role: 'student', initials: 'MI' },
  admin: { id: 'usr-adm-01', name: 'Dr. Arjun Sen', email: 'arjun.sen@demo.cyberspace.edu', role: 'admin', initials: 'AS' },
}

export const learningMaterials: LearningMaterial[] = [
  {
    id: 'net-foundations', title: 'Network Foundations', description: 'Build a practical model of addressing, routing, DNS, and common network services.', category: 'Networking Fundamentals', difficulty: 'Beginner', duration: 180,
    prerequisites: ['No prior networking experience'], outcomes: ['Interpret IPv4 subnets', 'Explain core protocols', 'Trace a basic connection'],
    lessons: [
      { id: 'net-1', title: 'How networks move data', duration: 30, completed: true },
      { id: 'net-2', title: 'IPv4 addressing and subnets', duration: 45, completed: true },
      { id: 'net-3', title: 'DNS, DHCP, and routing', duration: 50, completed: false },
      { id: 'net-4', title: 'Network troubleshooting lab', duration: 55, completed: false },
    ],
  },
  {
    id: 'linux-essentials', title: 'Linux Command Line Essentials', description: 'Navigate Linux systems, manage permissions, and inspect processes with confidence.', category: 'Linux Fundamentals', difficulty: 'Beginner', duration: 150,
    prerequisites: ['Basic computer literacy'], outcomes: ['Use the Linux filesystem', 'Manage users and permissions', 'Inspect services and logs'],
    lessons: [
      { id: 'lin-1', title: 'Shell orientation', duration: 25, completed: true },
      { id: 'lin-2', title: 'Files, pipes, and redirection', duration: 35, completed: true },
      { id: 'lin-3', title: 'Users and permissions', duration: 45, completed: true },
      { id: 'lin-4', title: 'Processes and services', duration: 45, completed: true },
    ],
  },
  {
    id: 'web-security', title: 'Web Security Foundations', description: 'Understand HTTP trust boundaries and identify common server-side security mistakes.', category: 'Web Security', difficulty: 'Intermediate', duration: 240,
    prerequisites: ['Network Foundations', 'Basic HTML knowledge'], outcomes: ['Inspect HTTP exchanges', 'Recognize access-control flaws', 'Apply secure input handling concepts'],
    lessons: [
      { id: 'web-1', title: 'HTTP under the hood', duration: 40, completed: true },
      { id: 'web-2', title: 'Sessions and identity', duration: 45, completed: false },
      { id: 'web-3', title: 'Input handling failures', duration: 55, completed: false },
      { id: 'web-4', title: 'Access control review', duration: 45, completed: false },
      { id: 'web-5', title: 'Defensive review lab', duration: 55, completed: false },
    ],
  },
  {
    id: 'recon-methods', title: 'Authorized Reconnaissance', description: 'Plan safe, scoped information gathering for college lab environments.', category: 'Reconnaissance', difficulty: 'Intermediate', duration: 130,
    prerequisites: ['Network Foundations'], outcomes: ['Define a test scope', 'Organize discovered assets', 'Report findings responsibly'],
    lessons: [
      { id: 'rec-1', title: 'Authorization and scope', duration: 25, completed: false },
      { id: 'rec-2', title: 'Asset discovery concepts', duration: 35, completed: false },
      { id: 'rec-3', title: 'Service enumeration concepts', duration: 35, completed: false },
      { id: 'rec-4', title: 'Evidence and reporting', duration: 35, completed: false },
    ],
  },
  {
    id: 'vulnerability-assessment', title: 'Vulnerability Assessment Practice', description: 'Prioritize findings, verify impact safely, and write actionable remediation notes.', category: 'Vulnerability Assessment', difficulty: 'Advanced', duration: 260,
    prerequisites: ['Web Security Foundations', 'Authorized Reconnaissance'], outcomes: ['Triage findings', 'Distinguish signal from noise', 'Write a concise assessment report'],
    lessons: [
      { id: 'va-1', title: 'Assessment methodology', duration: 45, completed: false },
      { id: 'va-2', title: 'Validation boundaries', duration: 50, completed: false },
      { id: 'va-3', title: 'Risk and prioritization', duration: 50, completed: false },
      { id: 'va-4', title: 'Remediation writing', duration: 55, completed: false },
      { id: 'va-5', title: 'Capstone review', duration: 60, completed: false },
    ],
  },
  {
    id: 'forensics-first-response', title: 'Digital Forensics First Response', description: 'Preserve evidence and build a simple timeline from a simulated incident.', category: 'Digital Forensics', difficulty: 'Intermediate', duration: 195,
    prerequisites: ['Linux Command Line Essentials'], outcomes: ['Preserve evidence integrity', 'Collect host artifacts', 'Build an incident timeline'],
    lessons: [
      { id: 'for-1', title: 'Evidence handling', duration: 35, completed: false },
      { id: 'for-2', title: 'Host artifact collection', duration: 50, completed: false },
      { id: 'for-3', title: 'Timeline analysis', duration: 55, completed: false },
      { id: 'for-4', title: 'Incident summary', duration: 55, completed: false },
    ],
  },
]

export const exampleYaml = `version: "1"
image: ghcr.io/cyberspace-college/web-lab:1.4
services:
  - name: web
    internal_port: 8080
    protocol: http
network:
  mode: isolated
resources:
  cpu: "1.0"
  memory: 768Mi
environment:
  LAB_MODE: assessment
health_check:
  path: /health
  interval_seconds: 10
startup_timeout_seconds: 90`

export const assessmentDefinitions: AssessmentDefinition[] = [
  { id: 'assess-web-1', name: 'Web Security Foundations', slug: 'web-security-foundations', description: 'Validate safe web security analysis in an isolated college lab.', level: 'Level 1', difficulty: 'Intermediate', duration: 90, prerequisites: ['Network Foundations'], outcomes: ['Inspect a web service', 'Submit evidence-based answers'], status: 'published', passingScore: 70, yaml: exampleYaml, createdAt: '2026-07-12T09:00:00Z', updatedAt: '2026-09-02T11:30:00Z', questions: [
    { id: 'q1', prompt: 'What HTTP status code is returned by the protected route?', type: 'short-text', points: 10, expectedAnswer: '401', hint: 'Inspect the response without credentials.' },
    { id: 'q2', prompt: 'Submit the flag shown after completing the access-control exercise.', type: 'flag', points: 25, expectedAnswer: 'CSPACE{demo_access_review}' },
    { id: 'q3', prompt: 'Which control most directly limits cross-site request forgery?', type: 'multiple-choice', points: 15, expectedAnswer: 'Anti-CSRF token', options: ['Rate limiting', 'Anti-CSRF token', 'Content compression'] },
  ] },
  { id: 'assess-linux-1', name: 'Linux Operations Checkpoint', slug: 'linux-operations-checkpoint', description: 'Assess practical Linux navigation and permissions knowledge.', level: 'Level 1', difficulty: 'Beginner', duration: 60, prerequisites: ['Linux Command Line Essentials'], outcomes: ['Inspect permissions', 'Interpret service state'], status: 'published', passingScore: 65, yaml: exampleYaml.replace('web-lab:1.4', 'linux-lab:1.2'), createdAt: '2026-06-18T09:00:00Z', updatedAt: '2026-08-24T13:00:00Z', questions: [
    { id: 'lq1', prompt: 'Which permission set allows the owner to read and write only?', type: 'multiple-choice', points: 10, expectedAnswer: 'rw-------', options: ['rwxr-xr-x', 'rw-------', 'r--r--r--'] },
  ] },
  { id: 'assess-recon-2', name: 'Scoped Reconnaissance Lab', slug: 'scoped-reconnaissance-lab', description: 'A draft lab for documenting scoped asset discovery.', level: 'Level 2', difficulty: 'Intermediate', duration: 120, prerequisites: ['Authorized Reconnaissance'], outcomes: ['Maintain scope', 'Record service evidence'], status: 'draft', passingScore: 75, yaml: exampleYaml, createdAt: '2026-08-30T08:30:00Z', updatedAt: '2026-09-08T16:45:00Z', questions: [] },
  { id: 'assess-forensics-2', name: 'Incident Timeline Review', slug: 'incident-timeline-review', description: 'Archived assessment for reconstructing a simulated incident timeline.', level: 'Level 2', difficulty: 'Advanced', duration: 150, prerequisites: ['Digital Forensics First Response'], outcomes: ['Correlate artifacts', 'Explain a timeline'], status: 'archived', passingScore: 75, yaml: exampleYaml, createdAt: '2026-03-10T10:00:00Z', updatedAt: '2026-07-01T10:00:00Z', questions: [] },
]

export const studentAssessments: StudentAssessment[] = [
  { id: 'stu-assess-1', definitionId: 'assess-web-1', name: 'Web Security Foundations', level: 'Level 1', scheduledAt: '2026-09-12T09:30:00+05:30', duration: 90, status: 'Ready', attempt: 1, progress: 0, eligibility: 'Prerequisites complete', environmentState: 'Stopped', targetIp: '10.20.8.14', answers: {} },
  { id: 'stu-assess-2', definitionId: 'assess-linux-1', name: 'Linux Operations Checkpoint', level: 'Level 1', scheduledAt: '2026-08-28T14:00:00+05:30', duration: 60, status: 'Completed', attempt: 1, progress: 100, eligibility: 'Completed', environmentState: 'Stopped', answers: { lq1: 'rw-------' } },
  { id: 'stu-assess-3', definitionId: 'assess-recon-2', name: 'Scoped Reconnaissance Lab', level: 'Level 2', scheduledAt: '2026-09-18T11:00:00+05:30', duration: 120, status: 'Awaiting approval', attempt: 1, progress: 0, eligibility: 'Eligible after administrator approval' },
  { id: 'stu-assess-4', definitionId: 'assess-forensics-2', name: 'Incident Timeline Review', level: 'Level 2', duration: 150, status: 'Not booked', attempt: 0, progress: 0, eligibility: 'Complete Digital Forensics First Response' },
  { id: 'stu-assess-5', definitionId: 'assess-web-1', name: 'Web Security Retake', level: 'Level 1', scheduledAt: '2026-07-04T09:30:00+05:30', duration: 90, status: 'Expired', attempt: 2, progress: 0, eligibility: 'Booking expired' },
  { id: 'stu-assess-6', definitionId: 'assess-linux-1', name: 'Linux Operations Practice', level: 'Level 1', scheduledAt: '2026-09-14T15:00:00+05:30', duration: 60, status: 'Booked', attempt: 2, progress: 0, eligibility: 'Booking received' },
  { id: 'stu-assess-7', definitionId: 'assess-web-1', name: 'Web Security Practice Session', level: 'Level 1', scheduledAt: '2026-09-10T16:00:00+05:30', duration: 90, status: 'Active', attempt: 2, progress: 33, eligibility: 'Authorized session in progress', environmentState: 'Running', targetIp: '10.20.8.22', answers: { q1: '401' } },
  { id: 'stu-assess-8', definitionId: 'assess-linux-1', name: 'Linux Operations Review', level: 'Level 1', scheduledAt: '2026-06-20T10:00:00+05:30', duration: 60, status: 'Revoked', attempt: 1, progress: 20, eligibility: 'Access revoked by administrator', environmentState: 'Stopped', answers: {} },
]

export const students: StudentRecord[] = [
  { id: 'stu-001', name: 'Maya Iyer', initials: 'MI', registerNumber: 'CS24-1042', email: 'maya.iyer@demo.cyberspace.edu', department: 'Computer Science', year: 3, active: true, eligible: true, lastAssessment: 'Linux Operations Checkpoint', completionRate: 76, notes: 'Consistent lab documentation.', attempts: 3 },
  { id: 'stu-002', name: 'Rohan Mehta', initials: 'RM', registerNumber: 'IT24-1088', email: 'rohan.mehta@demo.cyberspace.edu', department: 'Information Technology', year: 3, active: true, eligible: true, lastAssessment: 'Web Security Foundations', completionRate: 64, notes: '', attempts: 2 },
  { id: 'stu-003', name: 'Nila Joseph', initials: 'NJ', registerNumber: 'CS25-1121', email: 'nila.joseph@demo.cyberspace.edu', department: 'Computer Science', year: 2, active: true, eligible: false, lastAssessment: 'Linux Operations Checkpoint', completionRate: 48, notes: 'Prerequisite review requested.', attempts: 1 },
  { id: 'stu-004', name: 'Kabir Rao', initials: 'KR', registerNumber: 'CY23-1007', email: 'kabir.rao@demo.cyberspace.edu', department: 'Cybersecurity', year: 4, active: false, eligible: false, lastAssessment: 'Incident Timeline Review', completionRate: 82, notes: 'Account paused for semester leave.', attempts: 5 },
  { id: 'stu-005', name: 'Anika Bose', initials: 'AB', registerNumber: 'CY24-1064', email: 'anika.bose@demo.cyberspace.edu', department: 'Cybersecurity', year: 3, active: true, eligible: true, lastAssessment: 'Scoped Reconnaissance Lab', completionRate: 91, notes: '', attempts: 4 },
  { id: 'stu-006', name: 'Vikram Das', initials: 'VD', registerNumber: 'IT25-1190', email: 'vikram.das@demo.cyberspace.edu', department: 'Information Technology', year: 2, active: true, eligible: true, lastAssessment: 'Linux Operations Checkpoint', completionRate: 57, notes: '', attempts: 2 },
  { id: 'stu-007', name: 'Sara Khan', initials: 'SK', registerNumber: 'CS23-0984', email: 'sara.khan@demo.cyberspace.edu', department: 'Computer Science', year: 4, active: true, eligible: true, lastAssessment: 'Web Security Foundations', completionRate: 88, notes: 'Peer mentor for Level 1 labs.', attempts: 6 },
  { id: 'stu-008', name: 'Dev Patel', initials: 'DP', registerNumber: 'CY25-1203', email: 'dev.patel@demo.cyberspace.edu', department: 'Cybersecurity', year: 2, active: true, eligible: false, lastAssessment: 'No attempts', completionRate: 24, notes: '', attempts: 0 },
]

export const monitoringSessions: MonitoringSession[] = [
  { id: 'ses-001', studentId: 'stu-002', studentName: 'Rohan Mehta', registerNumber: 'IT24-1088', email: 'rohan.mehta@demo.cyberspace.edu', assessment: 'Web Security Foundations', level: 'Level 1', scheduledAt: '2026-09-10T10:00:00+05:30', bookingState: 'Confirmed', environmentState: 'Running', assessmentState: 'Active', timeRemaining: 46, progress: 58, startedAt: '2026-09-10T10:03:00+05:30', submittedAnswers: 4, totalAnswers: 7, actionHistory: ['Assessment approved by Dr. Arjun Sen', 'Environment started (simulated)'] },
  { id: 'ses-002', studentId: 'stu-003', studentName: 'Nila Joseph', registerNumber: 'CS25-1121', email: 'nila.joseph@demo.cyberspace.edu', assessment: 'Linux Operations Checkpoint', level: 'Level 1', scheduledAt: '2026-09-10T11:30:00+05:30', bookingState: 'Booked', environmentState: 'Not provisioned', assessmentState: 'Awaiting approval', timeRemaining: 60, progress: 0, submittedAnswers: 0, totalAnswers: 5, actionHistory: ['Booking imported from external system'] },
  { id: 'ses-003', studentId: 'stu-005', studentName: 'Anika Bose', registerNumber: 'CY24-1064', email: 'anika.bose@demo.cyberspace.edu', assessment: 'Scoped Reconnaissance Lab', level: 'Level 2', scheduledAt: '2026-09-10T09:00:00+05:30', bookingState: 'Confirmed', environmentState: 'Stopped', assessmentState: 'Completed', timeRemaining: 0, progress: 100, startedAt: '2026-09-10T09:02:00+05:30', endedAt: '2026-09-10T10:42:00+05:30', submittedAnswers: 8, totalAnswers: 8, actionHistory: ['Assessment approved', 'Environment started (simulated)', 'Assessment completed by student'] },
  { id: 'ses-004', studentId: 'stu-008', studentName: 'Dev Patel', registerNumber: 'CY25-1203', email: 'dev.patel@demo.cyberspace.edu', assessment: 'Linux Operations Checkpoint', level: 'Level 1', scheduledAt: '2026-09-10T13:00:00+05:30', bookingState: 'Waitlisted', environmentState: 'Not provisioned', assessmentState: 'Booked', timeRemaining: 60, progress: 0, submittedAnswers: 0, totalAnswers: 5, actionHistory: ['Booking imported; awaiting confirmed slot'] },
  { id: 'ses-005', studentId: 'stu-004', studentName: 'Kabir Rao', registerNumber: 'CY23-1007', email: 'kabir.rao@demo.cyberspace.edu', assessment: 'Incident Timeline Review', level: 'Level 2', scheduledAt: '2026-09-09T14:00:00+05:30', bookingState: 'Confirmed', environmentState: 'Stopped by admin', assessmentState: 'Revoked', timeRemaining: 0, progress: 32, startedAt: '2026-09-09T14:01:00+05:30', endedAt: '2026-09-09T14:28:00+05:30', submittedAnswers: 2, totalAnswers: 9, actionHistory: ['Assessment approved', 'Environment started (simulated)', 'Access revoked: account status review'] },
  { id: 'ses-006', studentId: 'stu-001', studentName: 'Maya Iyer', registerNumber: 'CS24-1042', email: 'maya.iyer@demo.cyberspace.edu', assessment: 'Web Security Foundations', level: 'Level 1', scheduledAt: '2026-09-10T15:00:00+05:30', bookingState: 'Confirmed', environmentState: 'Stopped', assessmentState: 'Ready', timeRemaining: 90, progress: 0, submittedAnswers: 0, totalAnswers: 7, actionHistory: ['Assessment approved by Dr. Arjun Sen'] },
]
