import type { LearningMaterial, User } from '../types/domain'

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
