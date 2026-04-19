package notify

import (
	"fmt"
	"strings"
	"time"
)

// buildHTMLEmail creates a professional, enterprise-grade HTML email for incident notifications.
func buildHTMLEmail(p NotificationPayload, dashboardURL string) string {
	isResolved := p.Status == "resolved"

	// Colors
	var accentColor, bgColor, statusLabel, statusIcon, severityBadge string
	switch {
	case isResolved:
		accentColor = "#10b981"
		bgColor = "#ecfdf5"
		statusLabel = "RESOLVED"
		statusIcon = `<span style="display:inline-block;width:10px;height:10px;border-radius:50%;background:#10b981;margin-right:6px;vertical-align:middle"></span>`
	case p.Severity == "critical":
		accentColor = "#ef4444"
		bgColor = "#fef2f2"
		statusLabel = "CRITICAL"
		statusIcon = `<span style="display:inline-block;width:10px;height:10px;border-radius:50%;background:#ef4444;margin-right:6px;vertical-align:middle"></span>`
	case p.Severity == "warning":
		accentColor = "#f59e0b"
		bgColor = "#fffbeb"
		statusLabel = "WARNING"
		statusIcon = `<span style="display:inline-block;width:10px;height:10px;border-radius:50%;background:#f59e0b;margin-right:6px;vertical-align:middle"></span>`
	default:
		accentColor = "#3b82f6"
		bgColor = "#eff6ff"
		statusLabel = "INFO"
		statusIcon = `<span style="display:inline-block;width:10px;height:10px;border-radius:50%;background:#3b82f6;margin-right:6px;vertical-align:middle"></span>`
	}
	_ = bgColor

	switch p.Severity {
	case "critical":
		severityBadge = fmt.Sprintf(`<span style="background:#fef2f2;color:#dc2626;padding:2px 10px;border-radius:12px;font-size:11px;font-weight:600;letter-spacing:0.5px;text-transform:uppercase;border:1px solid #fecaca">%s</span>`, p.Severity)
	case "warning":
		severityBadge = fmt.Sprintf(`<span style="background:#fffbeb;color:#d97706;padding:2px 10px;border-radius:12px;font-size:11px;font-weight:600;letter-spacing:0.5px;text-transform:uppercase;border:1px solid #fed7aa">%s</span>`, p.Severity)
	default:
		severityBadge = fmt.Sprintf(`<span style="background:#eff6ff;color:#2563eb;padding:2px 10px;border-radius:12px;font-size:11px;font-weight:600;letter-spacing:0.5px;text-transform:uppercase;border:1px solid #bfdbfe">%s</span>`, p.Severity)
	}

	resolvedRow := ""
	if p.ResolvedAt != "" {
		resolvedRow = fmt.Sprintf(`
					<tr>
						<td style="padding:8px 12px;font-size:13px;color:#6b7280;border-bottom:1px solid #f3f4f6;width:140px">Resolved At</td>
						<td style="padding:8px 12px;font-size:13px;color:#111827;border-bottom:1px solid #f3f4f6;font-weight:500">%s</td>
					</tr>`, p.ResolvedAt)
	}

	serverRow := ""
	if p.Server != "" {
		serverRow = fmt.Sprintf(`
					<tr>
						<td style="padding:8px 12px;font-size:13px;color:#6b7280;border-bottom:1px solid #f3f4f6;width:140px">Server</td>
						<td style="padding:8px 12px;font-size:13px;color:#111827;border-bottom:1px solid #f3f4f6;font-weight:500">%s</td>
					</tr>`, htmlEscape(p.Server))
	}

	dashboardButton := ""
	if dashboardURL != "" {
		incidentURL := fmt.Sprintf("%s/incidents/%s", strings.TrimRight(dashboardURL, "/"), p.IncidentID)
		dashboardButton = fmt.Sprintf(`
				<div style="text-align:center;margin:24px 0 8px">
					<a href="%s" style="display:inline-block;background:%s;color:#fff;padding:10px 28px;border-radius:6px;text-decoration:none;font-size:13px;font-weight:600;letter-spacing:0.3px">
						View Incident in Dashboard →
					</a>
				</div>`, incidentURL, accentColor)
	}

	year := time.Now().Year()

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width,initial-scale=1.0">
	<title>HealthOps Alert</title>
</head>
<body style="margin:0;padding:0;background:#f8fafc;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Helvetica Neue',Arial,sans-serif">
	<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background:#f8fafc">
		<tr>
			<td align="center" style="padding:32px 16px">
				<table role="presentation" width="600" cellpadding="0" cellspacing="0" style="max-width:600px;width:100%%">

					<!-- Header -->
					<tr>
						<td style="background:%s;padding:20px 24px;border-radius:12px 12px 0 0">
							<table role="presentation" width="100%%" cellpadding="0" cellspacing="0">
								<tr>
									<td>
										<span style="font-size:18px;font-weight:700;color:#fff;letter-spacing:-0.3px">
											%s HealthOps
										</span>
									</td>
									<td align="right">
										<span style="background:rgba(255,255,255,0.2);color:#fff;padding:4px 12px;border-radius:20px;font-size:11px;font-weight:600;letter-spacing:0.5px">
											%s
										</span>
									</td>
								</tr>
							</table>
						</td>
					</tr>

					<!-- Body -->
					<tr>
						<td style="background:#ffffff;padding:28px 24px;border-left:1px solid #e5e7eb;border-right:1px solid #e5e7eb">

							<!-- Title -->
							<h1 style="margin:0 0 6px;font-size:20px;font-weight:700;color:#111827;line-height:1.3">
								%s
							</h1>
							<p style="margin:0 0 20px;font-size:13px;color:#6b7280">
								Incident ID: <code style="background:#f3f4f6;padding:1px 6px;border-radius:4px;font-size:12px">%s</code>
							</p>

							<!-- Message -->
							<div style="background:#f9fafb;border:1px solid #e5e7eb;border-radius:8px;padding:14px 16px;margin-bottom:20px">
								<p style="margin:0;font-size:13px;color:#374151;line-height:1.6;word-break:break-word">
									%s
								</p>
							</div>

							<!-- Details Table -->
							<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="border:1px solid #e5e7eb;border-radius:8px;overflow:hidden">
								<tr>
									<td style="padding:8px 12px;font-size:13px;color:#6b7280;border-bottom:1px solid #f3f4f6;width:140px">Check</td>
									<td style="padding:8px 12px;font-size:13px;color:#111827;border-bottom:1px solid #f3f4f6;font-weight:600">%s</td>
								</tr>
								<tr>
									<td style="padding:8px 12px;font-size:13px;color:#6b7280;border-bottom:1px solid #f3f4f6">Type</td>
									<td style="padding:8px 12px;font-size:13px;color:#111827;border-bottom:1px solid #f3f4f6;font-weight:500">%s</td>
								</tr>
								<tr>
									<td style="padding:8px 12px;font-size:13px;color:#6b7280;border-bottom:1px solid #f3f4f6">Severity</td>
									<td style="padding:8px 12px;border-bottom:1px solid #f3f4f6">%s</td>
								</tr>%s
								<tr>
									<td style="padding:8px 12px;font-size:13px;color:#6b7280;border-bottom:1px solid #f3f4f6">Started</td>
									<td style="padding:8px 12px;font-size:13px;color:#111827;border-bottom:1px solid #f3f4f6;font-weight:500">%s</td>
								</tr>%s
								<tr>
									<td style="padding:8px 12px;font-size:13px;color:#6b7280">Status</td>
									<td style="padding:8px 12px;font-size:13px;font-weight:600;color:%s">%s</td>
								</tr>
							</table>

							%s
						</td>
					</tr>

					<!-- Footer -->
					<tr>
						<td style="background:#f9fafb;padding:16px 24px;border:1px solid #e5e7eb;border-top:none;border-radius:0 0 12px 12px">
							<table role="presentation" width="100%%" cellpadding="0" cellspacing="0">
								<tr>
									<td>
										<p style="margin:0;font-size:11px;color:#9ca3af;line-height:1.5">
											Sent by <strong style="color:#6b7280">HealthOps Monitoring</strong><br>
											This is an automated alert — do not reply to this email.
										</p>
									</td>
									<td align="right">
										<p style="margin:0;font-size:11px;color:#9ca3af">
											© %d HealthOps
										</p>
									</td>
								</tr>
							</table>
						</td>
					</tr>
				</table>
			</td>
		</tr>
	</table>
</body>
</html>`,
		accentColor,              // header background
		statusIcon,               // header icon
		statusLabel,              // header badge
		htmlEscape(p.CheckName),  // title
		htmlEscape(p.IncidentID), // incident ID
		htmlEscape(p.Message),    // message
		htmlEscape(p.CheckName),  // details: check name
		htmlEscape(p.CheckType),  // details: type
		severityBadge,            // details: severity badge
		serverRow,                // details: server (conditional)
		htmlEscape(p.StartedAt),  // details: started
		resolvedRow,              // details: resolved (conditional)
		accentColor,              // status color
		statusLabel,              // status label
		dashboardButton,          // CTA button
		year,                     // footer year
	)
}

func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

// buildDigestHTMLEmail creates a consolidated email for multiple failing checks.
func buildDigestHTMLEmail(payloads []NotificationPayload, critical, warning int, highest, dashboardURL string) string {
	accentColor := "#ef4444"
	if highest == "warning" {
		accentColor = "#f59e0b"
	}

	// Summary badges
	var summaryParts []string
	if critical > 0 {
		summaryParts = append(summaryParts,
			fmt.Sprintf(`<span style="background:#fef2f2;color:#dc2626;padding:3px 10px;border-radius:12px;font-size:11px;font-weight:600;border:1px solid #fecaca">%d CRITICAL</span>`, critical))
	}
	if warning > 0 {
		summaryParts = append(summaryParts,
			fmt.Sprintf(`<span style="background:#fffbeb;color:#d97706;padding:3px 10px;border-radius:12px;font-size:11px;font-weight:600;border:1px solid #fed7aa">%d WARNING</span>`, warning))
	}
	summaryBadges := strings.Join(summaryParts, " ")

	// Incident rows
	var rows string
	for _, p := range payloads {
		sevColor := "#dc2626"
		sevBg := "#fef2f2"
		if p.Severity == "warning" {
			sevColor = "#d97706"
			sevBg = "#fffbeb"
		}
		dotColor := "#ef4444"
		if p.Severity == "warning" {
			dotColor = "#f59e0b"
		}

		serverCol := ""
		if p.Server != "" {
			serverCol = htmlEscape(p.Server)
		} else {
			serverCol = `<span style="color:#9ca3af">—</span>`
		}

		rows += fmt.Sprintf(`
						<tr>
							<td style="padding:10px 12px;border-bottom:1px solid #f3f4f6">
								<span style="display:inline-block;width:8px;height:8px;border-radius:50%%;background:%s;margin-right:6px;vertical-align:middle"></span>
								<span style="font-size:13px;font-weight:600;color:#111827">%s</span>
							</td>
							<td style="padding:10px 12px;border-bottom:1px solid #f3f4f6">
								<span style="background:%s;color:%s;padding:2px 8px;border-radius:10px;font-size:10px;font-weight:600;text-transform:uppercase">%s</span>
							</td>
							<td style="padding:10px 12px;border-bottom:1px solid #f3f4f6;font-size:12px;color:#6b7280">%s</td>
							<td style="padding:10px 12px;border-bottom:1px solid #f3f4f6;font-size:12px;color:#374151;max-width:200px;word-break:break-word">%s</td>
						</tr>`,
			dotColor, htmlEscape(p.CheckName),
			sevBg, sevColor, htmlEscape(p.Severity),
			serverCol,
			htmlEscape(p.Message),
		)
	}

	dashboardButton := ""
	if dashboardURL != "" {
		incidentURL := fmt.Sprintf("%s/incidents", strings.TrimRight(dashboardURL, "/"))
		dashboardButton = fmt.Sprintf(`
				<div style="text-align:center;margin:24px 0 8px">
					<a href="%s" style="display:inline-block;background:%s;color:#fff;padding:10px 28px;border-radius:6px;text-decoration:none;font-size:13px;font-weight:600;letter-spacing:0.3px">
						View All Incidents →
					</a>
				</div>`, incidentURL, accentColor)
	}

	year := time.Now().Year()

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width,initial-scale=1.0">
	<title>HealthOps Alert Digest</title>
</head>
<body style="margin:0;padding:0;background:#f8fafc;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Helvetica Neue',Arial,sans-serif">
	<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background:#f8fafc">
		<tr>
			<td align="center" style="padding:32px 16px">
				<table role="presentation" width="640" cellpadding="0" cellspacing="0" style="max-width:640px;width:100%%">

					<!-- Header -->
					<tr>
						<td style="background:%s;padding:20px 24px;border-radius:12px 12px 0 0">
							<table role="presentation" width="100%%" cellpadding="0" cellspacing="0">
								<tr>
									<td>
										<span style="font-size:18px;font-weight:700;color:#fff;letter-spacing:-0.3px">
											HealthOps Alert Digest
										</span>
									</td>
									<td align="right">
										<span style="background:rgba(255,255,255,0.2);color:#fff;padding:4px 12px;border-radius:20px;font-size:11px;font-weight:600;letter-spacing:0.5px">
											%d CHECKS FAILING
										</span>
									</td>
								</tr>
							</table>
						</td>
					</tr>

					<!-- Body -->
					<tr>
						<td style="background:#ffffff;padding:28px 24px;border-left:1px solid #e5e7eb;border-right:1px solid #e5e7eb">

							<p style="margin:0 0 16px;font-size:14px;color:#374151;line-height:1.5">
								Multiple health checks are reporting failures. Here is a consolidated summary:
							</p>

							<!-- Summary badges -->
							<div style="margin-bottom:20px">
								%s
							</div>

							<!-- Incidents table -->
							<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="border:1px solid #e5e7eb;border-radius:8px;overflow:hidden">
								<tr style="background:#f9fafb">
									<th style="padding:8px 12px;text-align:left;font-size:11px;font-weight:600;color:#6b7280;text-transform:uppercase;border-bottom:1px solid #e5e7eb">Check</th>
									<th style="padding:8px 12px;text-align:left;font-size:11px;font-weight:600;color:#6b7280;text-transform:uppercase;border-bottom:1px solid #e5e7eb">Severity</th>
									<th style="padding:8px 12px;text-align:left;font-size:11px;font-weight:600;color:#6b7280;text-transform:uppercase;border-bottom:1px solid #e5e7eb">Server</th>
									<th style="padding:8px 12px;text-align:left;font-size:11px;font-weight:600;color:#6b7280;text-transform:uppercase;border-bottom:1px solid #e5e7eb">Details</th>
								</tr>%s
							</table>

							%s
						</td>
					</tr>

					<!-- Footer -->
					<tr>
						<td style="background:#f9fafb;padding:16px 24px;border:1px solid #e5e7eb;border-top:none;border-radius:0 0 12px 12px">
							<table role="presentation" width="100%%" cellpadding="0" cellspacing="0">
								<tr>
									<td>
										<p style="margin:0;font-size:11px;color:#9ca3af;line-height:1.5">
											Sent by <strong style="color:#6b7280">HealthOps Monitoring</strong><br>
											This is an automated alert — do not reply to this email.
										</p>
									</td>
									<td align="right">
										<p style="margin:0;font-size:11px;color:#9ca3af">
											&copy; %d HealthOps
										</p>
									</td>
								</tr>
							</table>
						</td>
					</tr>
				</table>
			</td>
		</tr>
	</table>
</body>
</html>`,
		accentColor,     // header background
		len(payloads),   // header badge count
		summaryBadges,   // summary badges
		rows,            // incident table rows
		dashboardButton, // CTA button
		year,            // footer year
	)
}
