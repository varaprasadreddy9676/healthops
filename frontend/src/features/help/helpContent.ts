export interface HelpCodeBlock {
  label: string
  code: string
}

export interface HelpSection {
  title: string
  paragraphs?: string[]
  bullets?: string[]
  code?: HelpCodeBlock
}

export interface HelpTopic {
  slug: string
  title: string
  summary: string
  intent: string
  sections: HelpSection[]
}

const logIngestionCurl = `curl -X POST https://your-healthops.example.com/api/v1/logs/ingest \\
  -H "Authorization: Bearer <healthops-api-token>" \\
  -H "Content-Type: application/json" \\
  -d '{
    "entries": [
      {
        "timestamp": "2026-05-17T08:30:00Z",
        "level": "error",
        "source": "auth-service",
        "server": "prod-web-01",
        "message": "JWT signature verification failed for request_id=req-123",
        "tags": ["security", "auth"],
        "meta": { "service": "auth", "reason": "signature_invalid" }
      }
    ]
  }'`

const logIngestionAgent = `# Examples of callers that can send log events
application code -> POST /api/v1/logs/ingest
sidecar or daemon -> tails files, batches events, posts every few seconds
cron/script -> posts operational errors from batch jobs
log forwarder -> transforms external logs into HealthOps events`

export const HELP_TOPICS: HelpTopic[] = [
  {
    slug: 'dashboard',
    title: 'Dashboard',
    summary: 'The operating overview for health, incidents, uptime, latency, and active risk.',
    intent: 'Use this page first when you want to know whether the monitored estate is healthy right now.',
    sections: [
      {
        title: 'What It Shows',
        bullets: [
          'Current status counts from configured checks.',
          'Recent incidents and open alerts that need attention.',
          'Availability and latency trends from stored check results.',
          'Server and service health summaries from the same backend state used by the API.',
        ],
      },
      {
        title: 'Where Data Comes From',
        paragraphs: [
          'The scheduler runs checks, stores every result, and the dashboard queries the summary API. It does not invent status. If a card looks wrong, open the related check or incident to inspect the raw evidence.',
        ],
      },
      {
        title: 'How To Use It',
        bullets: [
          'Start with critical counts, then open the incident or check behind the number.',
          'Use trend charts to distinguish a live outage from old noise.',
          'Use Refresh when you just changed checks or demo data.',
        ],
      },
    ],
  },
  {
    slug: 'servers',
    title: 'Servers',
    summary: 'Inventory and live operating signals for hosts that HealthOps monitors.',
    intent: 'Use this page to confirm which machines are visible and whether CPU, memory, disk, process, and network data look normal.',
    sections: [
      {
        title: 'How Servers Are Monitored',
        paragraphs: [
          'Servers are discovered from configured server records and check targets. HealthOps can collect host state through SSH checks, process checks, command checks, MySQL checks, and demo agents depending on the environment.',
        ],
      },
      {
        title: 'What To Trust First',
        bullets: [
          'Last seen tells you whether HealthOps still has fresh data.',
          'Processes and resource usage explain why checks are failing.',
          'Server detail pages are the place to compare live host state with check failures.',
        ],
      },
    ],
  },
  {
    slug: 'checks',
    title: 'Checks',
    summary: 'The monitoring rules that HealthOps runs on a schedule.',
    intent: 'Use checks to define what HealthOps should test, how often it should test it, and when failure should become an incident.',
    sections: [
      {
        title: 'Check Types',
        bullets: [
          'API checks call HTTP endpoints and validate response status, latency, and optional content.',
          'TCP, DNS, ping, SSL, and domain checks validate network and certificate health.',
          'Process, command, SSH, and MySQL checks inspect infrastructure behavior.',
          'Log file checks monitor file freshness or content on a host. These are separate from Log Events ingestion.',
          'Heartbeat checks expect an external job to ping HealthOps before a deadline.',
        ],
      },
      {
        title: 'Incident Rules',
        paragraphs: [
          'Failures do not need to alert immediately. Threshold settings such as failures-to-open and successes-to-resolve decide when a noisy check becomes a real incident and when it closes.',
        ],
      },
    ],
  },
  {
    slug: 'incidents',
    title: 'Incidents',
    summary: 'A focused timeline for active and historical operational problems.',
    intent: 'Use incidents to answer what broke, when it started, what evidence proves it, and what action is next.',
    sections: [
      {
        title: 'How Incidents Open',
        paragraphs: [
          'A check result, heartbeat miss, log signal, or MySQL signal crosses its configured threshold. HealthOps records the evidence and opens or updates an incident.',
        ],
      },
      {
        title: 'How To Read One',
        bullets: [
          'Start with the evidence summary, not AI text.',
          'Look at latest observed value, expected threshold, and last successful run.',
          'Use the timeline to see whether the problem is getting worse or already recovering.',
        ],
      },
    ],
  },
  {
    slug: 'status-pages',
    title: 'Status Pages',
    summary: 'Public or internal pages that communicate customer-facing service status.',
    intent: 'Use status pages to publish clear availability and incident updates without exposing private operational details.',
    sections: [
      {
        title: 'How They Work',
        paragraphs: [
          'A status page is configured with the checks or services it should represent. HealthOps converts internal check state into public component status and human-readable incident updates.',
        ],
      },
      {
        title: 'What Is Public',
        bullets: [
          'Component names, status, uptime, and public incident summaries can be shown.',
          'Internal paths, stack traces, credentials, raw metadata, and private server names should stay inside the authenticated app.',
          'Use public wording that explains impact rather than raw monitor internals.',
        ],
      },
    ],
  },
  {
    slug: 'root-cause',
    title: 'Root Cause',
    summary: 'AI-assisted incident analysis built from HealthOps evidence.',
    intent: 'Use this only after AI is configured. Treat it as a second opinion over the evidence, not as the source of truth.',
    sections: [
      {
        title: 'How It Works',
        paragraphs: [
          'HealthOps sends a constrained incident context to the configured AI provider. The prompt includes evidence such as check output, timings, service names, and recent related signals.',
        ],
      },
      {
        title: 'How To Validate It',
        bullets: [
          'Confirm that the suggested cause matches the raw evidence.',
          'Reject any answer that references data not visible in HealthOps.',
          'Prefer specific next checks over broad remediation advice.',
        ],
      },
    ],
  },
  {
    slug: 'mysql',
    title: 'MySQL',
    summary: 'Database health, connections, slow queries, replication signals, and process analysis.',
    intent: 'Use this page when application errors may be caused by database saturation, lock contention, slow queries, or connectivity.',
    sections: [
      {
        title: 'What HealthOps Reads',
        bullets: [
          'Connection utilization and active sessions.',
          'Slow query digests and query throughput.',
          'Thread and process list state.',
          'Server version, uptime, and core status counters.',
        ],
      },
      {
        title: 'Required Setup',
        paragraphs: [
          'MySQL checks use configured DSN environment variables. The monitored user should have enough read permissions for health views without granting unnecessary write access.',
        ],
      },
    ],
  },
  {
    slug: 'analytics',
    title: 'Analytics',
    summary: 'Longer-window reliability metrics for checks, incidents, latency, and availability.',
    intent: 'Use analytics to find recurring reliability patterns instead of reacting only to the latest alert.',
    sections: [
      {
        title: 'What It Measures',
        bullets: [
          'Availability and uptime by check or service.',
          'Latency percentiles and response-time trends.',
          'Incident volume and severity distribution.',
          'Recurring noisy monitors that may need tuning.',
        ],
      },
    ],
  },
  {
    slug: 'ai-results',
    title: 'AI Results',
    summary: 'Saved AI analyses for incidents and operational signals.',
    intent: 'Use this page to audit prior AI outputs and compare them against the underlying incidents.',
    sections: [
      {
        title: 'When It Appears',
        paragraphs: [
          'AI Results is only visible when AI is enabled and a provider is configured. Without AI configuration, HealthOps hides AI surfaces to avoid implying that analysis is running.',
        ],
      },
      {
        title: 'Review Checklist',
        bullets: [
          'Check provider, model, confidence, and creation time.',
          'Open the linked incident to verify the evidence.',
          'Do not treat generated remediation as approved automation.',
        ],
      },
    ],
  },
  {
    slug: 'assistant',
    title: 'Ask AI',
    summary: 'A conversational assistant for asking operational questions about HealthOps data.',
    intent: 'Use Ask AI for guided investigation, not for hidden actions. It should explain what data it used.',
    sections: [
      {
        title: 'What It Can Answer',
        bullets: [
          'Summaries of current incidents and unhealthy checks.',
          'Questions about recent patterns, affected services, and possible next diagnostics.',
          'Explanations of HealthOps concepts such as heartbeat checks, log ingestion, and status pages.',
        ],
      },
      {
        title: 'Boundaries',
        paragraphs: [
          'The assistant depends on configured AI access and available HealthOps data. If the answer lacks evidence, open the source screen and verify manually.',
        ],
      },
    ],
  },
  {
    slug: 'monitor-tuning',
    title: 'Monitor Tuning',
    summary: 'Recommendations for reducing alert noise and improving monitor coverage.',
    intent: 'Use this page to make checks more useful: fewer false positives, clearer thresholds, and better coverage of real risks.',
    sections: [
      {
        title: 'How Recommendations Are Produced',
        paragraphs: [
          'HealthOps inspects check history, incident frequency, failures, recoveries, and stale signals. When AI is enabled, optional AI text can enrich the recommendation, but the base recommendation should still be grounded in monitor data.',
        ],
      },
      {
        title: 'Good Changes',
        bullets: [
          'Increase failures-to-open for noisy but low-risk checks.',
          'Lower thresholds when a check is missing real incidents.',
          'Split broad checks into precise service-level checks.',
        ],
      },
    ],
  },
  {
    slug: 'remediation',
    title: 'Remediation',
    summary: 'Suggested or assisted operational actions for known incident types.',
    intent: 'Use remediation to standardize response steps. Review actions before running anything against production.',
    sections: [
      {
        title: 'Assisted Automation',
        paragraphs: [
          'Assisted automation means HealthOps can prepare guided actions from incident context, such as commands, restart steps, or runbook tasks. It should not silently mutate production. A user reviews the evidence and approves the action.',
        ],
      },
      {
        title: 'Safety Model',
        bullets: [
          'Prefer read-only diagnostics first.',
          'Show the target host, command, and reason before execution.',
          'Record what was attempted and whether it changed the incident state.',
        ],
      },
    ],
  },
  {
    slug: 'log-events',
    title: 'Log Events',
    summary: 'Application and server log messages sent into HealthOps, grouped into recurring patterns.',
    intent: 'Use Log Events when you want HealthOps to detect repeated errors across services instead of reading raw log files by hand.',
    sections: [
      {
        title: 'Who Calls The Ingestion API',
        paragraphs: [
          'Your application, a small sidecar agent, a cron script, or a log forwarder calls `POST /api/v1/logs/ingest`. HealthOps does not magically read every log file on every machine. Something must send the log event payload.',
          'In the demo, seeded services and demo scripts generate events so the screen has realistic data. In production, you wire the endpoint into your own services or logging pipeline.',
        ],
        code: { label: 'Common callers', code: logIngestionAgent },
      },
      {
        title: 'Raw Events vs Patterns',
        bullets: [
          'A raw log event is one message, for example one failed JWT validation line.',
          'A pattern is HealthOps grouping repeated messages that look like the same problem.',
          'The list page shows patterns so operators can see recurring issues without scrolling through thousands of raw lines.',
          'The detail page shows sample messages and recent raw entries for that pattern.',
        ],
      },
      {
        title: 'How To Send Logs',
        paragraphs: [
          'Send JSON events with timestamp, level, source, server, message, optional tags, and optional metadata. Batch multiple entries in one request when possible.',
        ],
        code: { label: 'Example request', code: logIngestionCurl },
      },
      {
        title: 'How Categories Are Assigned',
        paragraphs: [
          'HealthOps first uses deterministic rules. For example, timeout messages become Timeout, disk-full messages become Disk I/O, failed login messages become Security or Permission, and HTTP access lines become Access Log.',
          'Unclassified means the rule-based classifier could not confidently identify the pattern. If AI is configured, AI Categorize can review those recurring patterns.',
        ],
      },
      {
        title: 'How This Differs From Log File Checks',
        paragraphs: [
          'A Log file check is a scheduled health check that asks whether a specific file is fresh or contains expected content. Log Events ingestion is a stream of log messages that your apps or agents send to HealthOps for clustering and review.',
        ],
      },
    ],
  },
  {
    slug: 'notifications',
    title: 'Notifications',
    summary: 'Delivery channels and notification history for incidents and monitor events.',
    intent: 'Use notifications to decide who gets alerted, through which channel, and whether delivery succeeded.',
    sections: [
      {
        title: 'Channels',
        bullets: [
          'Configure destinations such as webhooks, email, Slack-compatible endpoints, or other supported channels.',
          'Test a channel before relying on it for production incidents.',
          'Review notification logs when a person says they did not receive an alert.',
        ],
      },
    ],
  },
  {
    slug: 'users',
    title: 'Users',
    summary: 'User accounts, roles, and access management for the HealthOps workspace.',
    intent: 'Use this page to grant only the access each operator needs.',
    sections: [
      {
        title: 'Role Guidance',
        bullets: [
          'Admins can configure checks, users, integrations, and sensitive settings.',
          'Operators should be able to investigate and acknowledge operational issues.',
          'Read-only users should inspect status without changing monitors or secrets.',
        ],
      },
    ],
  },
  {
    slug: 'settings',
    title: 'Settings',
    summary: 'Workspace-level configuration, AI provider setup, security settings, and runtime defaults.',
    intent: 'Use settings when changing integrations or behavior that affects the whole HealthOps installation.',
    sections: [
      {
        title: 'AI Configuration',
        paragraphs: [
          'AI keys are configured as provider settings and masked in responses. AI surfaces stay hidden until a provider is enabled and healthy.',
        ],
      },
      {
        title: 'Operational Rule',
        paragraphs: [
          'Changing settings can affect alerting, status pages, and automation. Verify with a small test incident or demo check before relying on new production behavior.',
        ],
      },
    ],
  },
]

export const HELP_TOPIC_BY_SLUG = Object.fromEntries(HELP_TOPICS.map((topic) => [topic.slug, topic])) as Record<string, HelpTopic>

const PATH_TOPIC_RULES: Array<[RegExp, string]> = [
  [/^\/servers\b/, 'servers'],
  [/^\/checks\b/, 'checks'],
  [/^\/incidents\b/, 'incidents'],
  [/^\/status-pages\b/, 'status-pages'],
  [/^\/rca\b/, 'root-cause'],
  [/^\/mysql\b/, 'mysql'],
  [/^\/analytics\b/, 'analytics'],
  [/^\/ai\b/, 'ai-results'],
  [/^\/assistant\b/, 'assistant'],
  [/^\/recommendations\b/, 'monitor-tuning'],
  [/^\/automation\b/, 'remediation'],
  [/^\/logs\b/, 'log-events'],
  [/^\/notifications\b/, 'notifications'],
  [/^\/users\b/, 'users'],
  [/^\/settings\b/, 'settings'],
  [/^\/help\b/, 'dashboard'],
  [/^\/$/, 'dashboard'],
]

export function getHelpSlugForPath(pathname: string): string {
  return PATH_TOPIC_RULES.find(([pattern]) => pattern.test(pathname))?.[1] ?? 'dashboard'
}
