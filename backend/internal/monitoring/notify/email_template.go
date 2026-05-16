package notify

import (
	"fmt"
	"strings"
	"time"
)

// buildHTMLEmail creates a professional HTML email matching the HealthOps dashboard aesthetic.
func buildHTMLEmail(p NotificationPayload, dashboardURL string) string {
	isResolved := p.Status == "resolved"

	var (
		accentColor  string
		headerBg     string
		statusLabel  string
		statusEmoji  string
		badgeBg      string
		badgeText    string
		badgeBorder  string
		dotColor     string
		titlePrefix  string
	)

	switch {
	case isResolved:
		accentColor = "#10b981"
		headerBg = "#065f46"
		statusLabel = "RESOLVED"
		statusEmoji = "✓"
		badgeBg = "#d1fae5"
		badgeText = "#065f46"
		badgeBorder = "#6ee7b7"
		dotColor = "#10b981"
		titlePrefix = "Resolved"
	case p.Severity == "critical":
		accentColor = "#ef4444"
		headerBg = "#7f1d1d"
		statusLabel = "CRITICAL"
		statusEmoji = "!"
		badgeBg = "#fee2e2"
		badgeText = "#991b1b"
		badgeBorder = "#fca5a5"
		dotColor = "#ef4444"
		titlePrefix = "Alert"
	case p.Severity == "warning":
		accentColor = "#f59e0b"
		headerBg = "#78350f"
		statusLabel = "WARNING"
		statusEmoji = "⚠"
		badgeBg = "#fef3c7"
		badgeText = "#92400e"
		badgeBorder = "#fcd34d"
		dotColor = "#f59e0b"
		titlePrefix = "Warning"
	default:
		accentColor = "#3b82f6"
		headerBg = "#1e3a5f"
		statusLabel = "INFO"
		statusEmoji = "i"
		badgeBg = "#dbeafe"
		badgeText = "#1e40af"
		badgeBorder = "#93c5fd"
		dotColor = "#3b82f6"
		titlePrefix = "Info"
	}

	resolvedRow := ""
	if p.ResolvedAt != "" {
		t, err := time.Parse(time.RFC3339, p.ResolvedAt)
		displayTime := p.ResolvedAt
		if err == nil {
			displayTime = t.UTC().Format("Jan 02, 2006 · 15:04:05 UTC")
		}
		resolvedRow = fmt.Sprintf(`
				<tr>
					<td style="padding:10px 16px;font-size:12px;color:#94a3b8;border-bottom:1px solid #1e293b;white-space:nowrap;width:110px">Resolved</td>
					<td style="padding:10px 16px;font-size:13px;color:#e2e8f0;border-bottom:1px solid #1e293b;font-weight:500">%s</td>
				</tr>`, htmlEscape(displayTime))
	}

	serverRow := ""
	if p.Server != "" {
		serverRow = fmt.Sprintf(`
				<tr>
					<td style="padding:10px 16px;font-size:12px;color:#94a3b8;border-bottom:1px solid #1e293b;white-space:nowrap;width:110px">Server</td>
					<td style="padding:10px 16px;font-size:13px;color:#e2e8f0;border-bottom:1px solid #1e293b;font-weight:500">%s</td>
				</tr>`, htmlEscape(p.Server))
	}

	startedDisplay := p.StartedAt
	if t, err := time.Parse(time.RFC3339, p.StartedAt); err == nil {
		startedDisplay = t.UTC().Format("Jan 02, 2006 · 15:04:05 UTC")
	}

	dashboardButton := ""
	incidentPageURL := ""
	if dashboardURL != "" {
		incidentPageURL = fmt.Sprintf("%s/incidents/%s", strings.TrimRight(dashboardURL, "/"), p.IncidentID)
		dashboardButton = fmt.Sprintf(`
			<div style="text-align:center;padding:24px 0 8px">
				<a href="%s"
				   style="display:inline-block;background:%s;color:#ffffff;padding:12px 32px;border-radius:8px;text-decoration:none;font-size:13px;font-weight:600;letter-spacing:0.3px;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">
					View Incident in HealthOps &rarr;
				</a>
			</div>`, incidentPageURL, accentColor)
	}

	year := time.Now().Year()
	shortID := p.IncidentID
	if len(shortID) > 24 {
		shortID = shortID[:24] + "…"
	}

	checkTypeFmt := p.CheckType
	if checkTypeFmt == "" {
		checkTypeFmt = "api"
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en" xmlns="http://www.w3.org/1999/xhtml">
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width,initial-scale=1.0">
	<meta http-equiv="X-UA-Compatible" content="IE=edge">
	<title>HealthOps %s: %s</title>
</head>
<body style="margin:0;padding:0;background:#0f172a;-webkit-font-smoothing:antialiased">
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="background:#0f172a;min-height:100vh">
	<tr>
		<td align="center" style="padding:32px 16px">

			<!-- Outer card: 600px max -->
			<table role="presentation" width="600" cellpadding="0" cellspacing="0" border="0"
			       style="max-width:600px;width:100%%;border-radius:16px;overflow:hidden;box-shadow:0 25px 50px rgba(0,0,0,0.5)">

				<!-- ── HEADER ── -->
				<tr>
					<td style="background:%s;padding:0">
						<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0">
							<tr>
								<!-- Left accent bar -->
								<td style="background:%s;width:4px;padding:0">&nbsp;</td>
								<td style="padding:20px 24px">
									<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0">
										<tr>
											<td>
												<!-- Logo -->
												<table role="presentation" cellpadding="0" cellspacing="0" border="0">
													<tr>
														<td style="background:%s;border-radius:8px;width:32px;height:32px;text-align:center;vertical-align:middle">
															<span style="font-size:16px;font-weight:900;color:#ffffff;line-height:32px;display:block">♥</span>
														</td>
														<td style="padding-left:10px">
															<span style="font-size:16px;font-weight:700;color:#ffffff;letter-spacing:-0.3px;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">HealthOps</span>
														</td>
													</tr>
												</table>
											</td>
											<td align="right">
												<!-- Status badge -->
												<span style="display:inline-block;background:%s;color:%s;padding:5px 14px;border-radius:20px;font-size:11px;font-weight:700;letter-spacing:1px;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;border:1px solid %s">
													%s %s
												</span>
											</td>
										</tr>
									</table>
								</td>
							</tr>
						</table>
					</td>
				</tr>

				<!-- ── TITLE SECTION ── -->
				<tr>
					<td style="background:#1e293b;padding:28px 28px 20px;border-left:1px solid #334155;border-right:1px solid #334155">
						<!-- Status dot + check name -->
						<table role="presentation" cellpadding="0" cellspacing="0" border="0" style="margin-bottom:4px">
							<tr>
								<td style="vertical-align:middle;padding-right:10px">
									<span style="display:inline-block;width:12px;height:12px;border-radius:50%%;background:%s;box-shadow:0 0 0 3px %s33"></span>
								</td>
								<td style="vertical-align:middle">
									<span style="font-size:22px;font-weight:700;color:#f1f5f9;letter-spacing:-0.5px;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">%s</span>
								</td>
							</tr>
						</table>
						<!-- Incident ID -->
						<p style="margin:8px 0 0;font-size:12px;color:#64748b;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">
							Incident &nbsp;<code style="background:#0f172a;color:#94a3b8;padding:2px 8px;border-radius:4px;font-size:11px;font-family:'SFMono-Regular','Consolas',monospace;border:1px solid #334155">%s</code>
						</p>
					</td>
				</tr>

				<!-- ── MESSAGE BOX ── -->
				<tr>
					<td style="background:#1e293b;padding:0 28px 20px;border-left:1px solid #334155;border-right:1px solid #334155">
						<div style="background:#0f172a;border:1px solid #334155;border-left:3px solid %s;border-radius:8px;padding:14px 16px">
							<p style="margin:0;font-size:13px;color:#94a3b8;line-height:1.65;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;word-break:break-word">
								%s
							</p>
						</div>
					</td>
				</tr>

				<!-- ── DETAILS TABLE ── -->
				<tr>
					<td style="background:#1e293b;padding:0 28px 24px;border-left:1px solid #334155;border-right:1px solid #334155">
						<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0"
						       style="border:1px solid #334155;border-radius:10px;overflow:hidden">

							<!-- Table header -->
							<tr style="background:#0f172a">
								<td colspan="2" style="padding:10px 16px;border-bottom:1px solid #334155">
									<span style="font-size:10px;font-weight:600;color:#64748b;letter-spacing:1.2px;text-transform:uppercase;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">Incident Details</span>
								</td>
							</tr>

							<!-- Check -->
							<tr>
								<td style="padding:10px 16px;font-size:12px;color:#94a3b8;border-bottom:1px solid #1e293b;white-space:nowrap;width:110px;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">Check</td>
								<td style="padding:10px 16px;font-size:13px;color:#e2e8f0;border-bottom:1px solid #1e293b;font-weight:600;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">%s</td>
							</tr>

							<!-- Type -->
							<tr>
								<td style="padding:10px 16px;font-size:12px;color:#94a3b8;border-bottom:1px solid #1e293b;white-space:nowrap;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">Type</td>
								<td style="padding:10px 16px;font-size:13px;color:#e2e8f0;border-bottom:1px solid #1e293b;font-weight:500;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">
									<span style="background:#0f172a;color:#94a3b8;padding:2px 8px;border-radius:4px;font-size:11px;font-family:'SFMono-Regular','Consolas',monospace;border:1px solid #334155">%s</span>
								</td>
							</tr>

							%s

							<!-- Severity -->
							<tr>
								<td style="padding:10px 16px;font-size:12px;color:#94a3b8;border-bottom:1px solid #1e293b;white-space:nowrap;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">Severity</td>
								<td style="padding:10px 16px;border-bottom:1px solid #1e293b">
									<span style="background:%s;color:%s;padding:3px 10px;border-radius:12px;font-size:10px;font-weight:700;letter-spacing:0.8px;text-transform:uppercase;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;border:1px solid %s">
										%s
									</span>
								</td>
							</tr>

							<!-- Started -->
							<tr>
								<td style="padding:10px 16px;font-size:12px;color:#94a3b8;border-bottom:1px solid #1e293b;white-space:nowrap;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">Started</td>
								<td style="padding:10px 16px;font-size:13px;color:#e2e8f0;border-bottom:1px solid #1e293b;font-weight:500;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">%s</td>
							</tr>

							%s

							<!-- Status -->
							<tr>
								<td style="padding:10px 16px;font-size:12px;color:#94a3b8;white-space:nowrap;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">Status</td>
								<td style="padding:10px 16px;font-size:13px;font-weight:700;color:%s;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">%s</td>
							</tr>

						</table>
					</td>
				</tr>

				<!-- ── CTA BUTTON ── -->
				%s

				<!-- ── FOOTER ── -->
				<tr>
					<td style="background:#0f172a;padding:20px 28px;border:1px solid #1e293b;border-top:1px solid #334155;border-radius:0 0 16px 16px">
						<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0">
							<tr>
								<td>
									<p style="margin:0;font-size:11px;color:#475569;line-height:1.6;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">
										Sent by <strong style="color:#64748b;font-weight:600">HealthOps Monitoring</strong><br>
										This is an automated alert — do not reply to this email.
									</p>
								</td>
								<td align="right" style="vertical-align:top">
									<p style="margin:0;font-size:11px;color:#334155;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">
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
		// <title>
		titlePrefix, htmlEscape(p.CheckName),
		// header bg
		headerBg,
		// accent bar color
		accentColor,
		// logo box bg
		accentColor,
		// status badge: bg, text, border, emoji, label
		badgeBg, badgeText, badgeBorder, statusEmoji, statusLabel,
		// title dot: dot color, glow color
		dotColor, dotColor,
		// check name h1
		htmlEscape(p.CheckName),
		// incident ID
		htmlEscape(shortID),
		// message box left border color
		accentColor,
		// message text
		htmlEscape(p.Message),
		// details: check name
		htmlEscape(p.CheckName),
		// details: type badge
		htmlEscape(checkTypeFmt),
		// server row (conditional)
		serverRow,
		// severity badge: bg, text, border, label
		badgeBg, badgeText, badgeBorder, htmlEscape(p.Severity),
		// started
		htmlEscape(startedDisplay),
		// resolved row (conditional)
		resolvedRow,
		// status color and label
		accentColor, statusLabel,
		// CTA button (conditional)
		wrapCTARow(dashboardButton),
		// footer year
		year,
	)
}

// wrapCTARow wraps the dashboard button in a table row if non-empty.
func wrapCTARow(button string) string {
	if button == "" {
		return ""
	}
	return fmt.Sprintf(`
			<tr>
				<td style="background:#1e293b;padding:0 28px 24px;border-left:1px solid #334155;border-right:1px solid #334155">
					%s
				</td>
			</tr>`, button)
}

func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

// buildDigestHTMLEmail creates a consolidated digest email for multiple failing checks.
func buildDigestHTMLEmail(payloads []NotificationPayload, critical, warning int, highest, dashboardURL string) string {
	accentColor := "#ef4444"
	headerBg := "#7f1d1d"
	if highest == "warning" {
		accentColor = "#f59e0b"
		headerBg = "#78350f"
	}

	var summaryParts []string
	if critical > 0 {
		summaryParts = append(summaryParts,
			fmt.Sprintf(`<span style="display:inline-block;background:#fee2e2;color:#991b1b;padding:4px 12px;border-radius:12px;font-size:11px;font-weight:700;border:1px solid #fca5a5;margin-right:6px;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">%d CRITICAL</span>`, critical))
	}
	if warning > 0 {
		summaryParts = append(summaryParts,
			fmt.Sprintf(`<span style="display:inline-block;background:#fef3c7;color:#92400e;padding:4px 12px;border-radius:12px;font-size:11px;font-weight:700;border:1px solid #fcd34d;margin-right:6px;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">%d WARNING</span>`, warning))
	}
	summaryBadges := strings.Join(summaryParts, "")

	var rows string
	for _, p := range payloads {
		sevColor := "#ef4444"
		sevBg := "#fee2e2"
		sevBorder := "#fca5a5"
		sevText := "#991b1b"
		if p.Severity == "warning" {
			sevColor = "#f59e0b"
			sevBg = "#fef3c7"
			sevBorder = "#fcd34d"
			sevText = "#92400e"
		}

		serverCell := `<span style="color:#475569;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">—</span>`
		if p.Server != "" {
			serverCell = fmt.Sprintf(`<span style="color:#94a3b8;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">%s</span>`, htmlEscape(p.Server))
		}

		rows += fmt.Sprintf(`
					<tr>
						<td style="padding:10px 14px;border-bottom:1px solid #1e293b">
							<span style="display:inline-block;width:8px;height:8px;border-radius:50%%;background:%s;margin-right:8px;vertical-align:middle"></span>
							<span style="font-size:13px;font-weight:600;color:#e2e8f0;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">%s</span>
						</td>
						<td style="padding:10px 14px;border-bottom:1px solid #1e293b">
							<span style="background:%s;color:%s;padding:2px 9px;border-radius:10px;font-size:10px;font-weight:700;text-transform:uppercase;border:1px solid %s;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">%s</span>
						</td>
						<td style="padding:10px 14px;border-bottom:1px solid #1e293b;font-size:12px">%s</td>
						<td style="padding:10px 14px;border-bottom:1px solid #1e293b;font-size:12px;color:#94a3b8;max-width:180px;word-break:break-word;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">%s</td>
					</tr>`,
			sevColor, htmlEscape(p.CheckName),
			sevBg, sevText, sevBorder, htmlEscape(p.Severity),
			serverCell,
			htmlEscape(p.Message),
		)
	}

	dashboardButton := ""
	if dashboardURL != "" {
		incidentURL := fmt.Sprintf("%s/incidents", strings.TrimRight(dashboardURL, "/"))
		dashboardButton = fmt.Sprintf(`
					<div style="text-align:center;padding:24px 0 8px">
						<a href="%s"
						   style="display:inline-block;background:%s;color:#ffffff;padding:12px 32px;border-radius:8px;text-decoration:none;font-size:13px;font-weight:600;letter-spacing:0.3px;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">
							View All Incidents in HealthOps &rarr;
						</a>
					</div>`, incidentURL, accentColor)
	}

	year := time.Now().Year()

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en" xmlns="http://www.w3.org/1999/xhtml">
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width,initial-scale=1.0">
	<meta http-equiv="X-UA-Compatible" content="IE=edge">
	<title>HealthOps Alert Digest — %d checks failing</title>
</head>
<body style="margin:0;padding:0;background:#0f172a;-webkit-font-smoothing:antialiased">
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="background:#0f172a;min-height:100vh">
	<tr>
		<td align="center" style="padding:32px 16px">
			<table role="presentation" width="640" cellpadding="0" cellspacing="0" border="0"
			       style="max-width:640px;width:100%%;border-radius:16px;overflow:hidden;box-shadow:0 25px 50px rgba(0,0,0,0.5)">

				<!-- ── HEADER ── -->
				<tr>
					<td style="background:%s;padding:0">
						<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0">
							<tr>
								<td style="background:%s;width:4px;padding:0">&nbsp;</td>
								<td style="padding:20px 24px">
									<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0">
										<tr>
											<td>
												<table role="presentation" cellpadding="0" cellspacing="0" border="0">
													<tr>
														<td style="background:%s;border-radius:8px;width:32px;height:32px;text-align:center;vertical-align:middle">
															<span style="font-size:16px;font-weight:900;color:#ffffff;line-height:32px;display:block">♥</span>
														</td>
														<td style="padding-left:10px">
															<span style="font-size:16px;font-weight:700;color:#ffffff;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">HealthOps Alert Digest</span>
														</td>
													</tr>
												</table>
											</td>
											<td align="right">
												<span style="display:inline-block;background:rgba(255,255,255,0.15);color:#fff;padding:5px 14px;border-radius:20px;font-size:11px;font-weight:700;letter-spacing:1px;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">
													%d CHECKS FAILING
												</span>
											</td>
										</tr>
									</table>
								</td>
							</tr>
						</table>
					</td>
				</tr>

				<!-- ── SUMMARY ── -->
				<tr>
					<td style="background:#1e293b;padding:20px 28px 16px;border-left:1px solid #334155;border-right:1px solid #334155">
						<p style="margin:0 0 12px;font-size:14px;color:#94a3b8;line-height:1.6;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">
							Multiple health checks are reporting failures. Here is a consolidated summary:
						</p>
						<div>%s</div>
					</td>
				</tr>

				<!-- ── INCIDENTS TABLE ── -->
				<tr>
					<td style="background:#1e293b;padding:0 28px 24px;border-left:1px solid #334155;border-right:1px solid #334155">
						<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0"
						       style="border:1px solid #334155;border-radius:10px;overflow:hidden">
							<tr style="background:#0f172a">
								<th style="padding:10px 14px;text-align:left;font-size:10px;font-weight:600;color:#64748b;text-transform:uppercase;letter-spacing:1px;border-bottom:1px solid #334155;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">Check</th>
								<th style="padding:10px 14px;text-align:left;font-size:10px;font-weight:600;color:#64748b;text-transform:uppercase;letter-spacing:1px;border-bottom:1px solid #334155;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">Severity</th>
								<th style="padding:10px 14px;text-align:left;font-size:10px;font-weight:600;color:#64748b;text-transform:uppercase;letter-spacing:1px;border-bottom:1px solid #334155;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">Server</th>
								<th style="padding:10px 14px;text-align:left;font-size:10px;font-weight:600;color:#64748b;text-transform:uppercase;letter-spacing:1px;border-bottom:1px solid #334155;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">Details</th>
							</tr>
							%s
						</table>
						%s
					</td>
				</tr>

				<!-- ── FOOTER ── -->
				<tr>
					<td style="background:#0f172a;padding:20px 28px;border:1px solid #1e293b;border-top:1px solid #334155;border-radius:0 0 16px 16px">
						<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0">
							<tr>
								<td>
									<p style="margin:0;font-size:11px;color:#475569;line-height:1.6;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">
										Sent by <strong style="color:#64748b;font-weight:600">HealthOps Monitoring</strong><br>
										This is an automated alert — do not reply to this email.
									</p>
								</td>
								<td align="right" style="vertical-align:top">
									<p style="margin:0;font-size:11px;color:#334155;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">
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
		len(payloads),
		headerBg,
		accentColor,
		accentColor,
		len(payloads),
		summaryBadges,
		rows,
		dashboardButton,
		year,
	)
}
