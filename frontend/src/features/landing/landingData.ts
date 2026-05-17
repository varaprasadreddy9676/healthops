import {
  Activity,
  AlertTriangle,
  Bot,
  Database,
  FileText,
  Github,
  Globe2,
  HeartPulse,
  ShieldCheck,
  Workflow,
  type LucideIcon,
} from 'lucide-react'

export const repoUrl = 'https://github.com/varaprasadreddy9676/healthops'

export const proofItems = [
  { label: 'Stack', value: 'Go + MongoDB' },
  { label: 'Repo', value: 'Fresh OSS' },
  { label: 'License', value: 'MIT' },
  { label: 'Install', value: 'Docker Compose' },
]

export const quickStartCommand = `docker compose -f compose.demo.yaml up -d --build
# open http://localhost:18080`

export const productionCommand = `export HEALTHOPS_BOOTSTRAP_ADMIN_PASSWORD='change-this'
docker compose up -d --build`

export const productScreens = [
  {
    title: 'Dashboard',
    label: 'Command overview',
    image: '/landing/dashboard.png',
    copy: 'See checks, incidents, uptime, heartbeats, and recent events before switching tools.',
  },
  {
    title: 'Log Events',
    label: 'Ingestion review',
    image: '/landing/log-events.png',
    copy: 'Group app, server, audit, access, database, and security logs into patterns responders can scan.',
  },
  {
    title: 'Public Status Page',
    label: 'Customer communication',
    image: '/landing/status-page.png',
    copy: 'Publish service health, maintenance windows, and incident history from the same system.',
  },
  {
    title: 'Incident Operations',
    label: 'Response workflow',
    image: '/landing/incident-detail.png',
    copy: 'Everything responders need on one screen: evidence, timeline, owner, and recovery state.',
  },
]

export interface FeatureItem {
  icon: LucideIcon
  title: string
  copy: string
}

export const featureItems: FeatureItem[] = [
  {
    icon: Activity,
    title: 'Uptime and health checks',
    copy: 'HTTP, TCP, DNS, SSL, domain, SSH, command, process, MySQL, ping, log freshness, and heartbeat checks.',
  },
  {
    icon: AlertTriangle,
    title: 'Incident lifecycle',
    copy: 'Deduplication, ownership, timelines, auto-resolution, and MTTR context.',
  },
  {
    icon: Bot,
    title: 'BYOK root-cause analysis',
    copy: 'Bring your own provider key for evidence-grounded summaries, or run without it.',
  },
  {
    icon: FileText,
    title: 'Log ingestion API',
    copy: 'Applications, scripts, sidecars, or forwarders post events to HealthOps for pattern triage.',
  },
  {
    icon: Database,
    title: 'MySQL visibility',
    copy: 'Connections, slow queries, process lists, thread pressure, and database health signals.',
  },
  {
    icon: Globe2,
    title: 'Status pages',
    copy: 'Public status pages translate internal state into clean service health updates.',
  },
]

export const engineeringItems: FeatureItem[] = [
  {
    icon: HeartPulse,
    title: 'One Go service',
    copy: 'The API, scheduler, ingestion, notification outbox, and incident lifecycle run in one service.',
  },
  {
    icon: Workflow,
    title: 'Goroutines per check',
    copy: 'Checks, heartbeats, and log evaluations run concurrently without thread pools or worker fleets.',
  },
  {
    icon: ShieldCheck,
    title: 'Boring deployments',
    copy: 'Single image. Docker Compose for the demo, Kubernetes-ready for production.',
  },
]

export const replacementItems = [
  {
    category: 'Uptime monitoring',
    examples: 'UptimeRobot, Pingdom, StatusCake-style checks',
    answer: 'HTTP, TCP, DNS, SSL, domain, ping, heartbeat, and freshness checks.',
  },
  {
    category: 'Incident response',
    examples: 'Lightweight PagerDuty or Opsgenie-style workflows',
    answer: 'Incidents, ownership, timelines, notifications, and auto-resolution updates.',
  },
  {
    category: 'Status communication',
    examples: 'Statuspage or Instatus-style public updates',
    answer: 'Public status pages backed by checks, incidents, and maintenance windows.',
  },
  {
    category: 'Log triage',
    examples: 'Tail and pattern-group operational logs',
    answer: 'Log ingestion, pattern grouping, severity distribution, and incident correlation.',
  },
  {
    category: 'Database triage',
    examples: 'Small-team MySQL troubleshooting workflows',
    answer: 'MySQL connection, query, thread, and health context alongside incidents.',
  },
]

export const scenarioCommands = [
  'scripts/demo-scenario.sh api-down',
  'scripts/demo-scenario.sh api-slow',
  'scripts/demo-scenario.sh mysql-load',
  'scripts/demo-scenario.sh rca',
  'scripts/demo-scenario.sh recover',
]

export const footerLinks = [
  { label: 'GitHub', href: repoUrl, external: true, icon: Github },
  { label: 'Docs', href: '/help/getting-started' },
  { label: 'Log ingestion guide', href: '/help/log-events' },
  { label: 'MIT License', href: `${repoUrl}/blob/main/LICENSE`, external: true },
]

export const workflowSteps = [
  { step: '01', title: 'Detect', copy: 'A check fails, a heartbeat misses, or a log pattern spikes.' },
  { step: '02', title: 'Open', copy: 'An incident opens, deduped against existing ones, with the failing evidence attached.' },
  { step: '03', title: 'Notify', copy: 'Configured email and webhook channels receive incident notifications.' },
  { step: '04', title: 'Explain', copy: 'If you configure an AI provider, it drafts a root-cause summary against the evidence.' },
  { step: '05', title: 'Recover', copy: 'When checks recover, HealthOps resolves the incident and sends recovery updates.' },
]

export const ossBadges = [
  'Open source',
  'Self-hosted',
  'BYOK AI',
]
