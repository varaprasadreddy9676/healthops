package notify

import (
	"fmt"
	"strings"
	"time"
)

// App color palette (matches Tailwind tokens used in the frontend)
const (
	colorSlate50   = "#f8fafc"
	colorSlate100  = "#f1f5f9"
	colorSlate200  = "#e2e8f0"
	colorSlate400  = "#94a3b8"
	colorSlate500  = "#64748b"
	colorSlate700  = "#334155"
	colorSlate900  = "#0f172a"
	colorBlue600   = "#2563eb"
	colorBlue50    = "#eff6ff"
	colorBlue100   = "#dbeafe"
	colorBlue700   = "#1d4ed8"
	colorRed600    = "#dc2626"
	colorRed50     = "#fef2f2"
	colorRed100    = "#fee2e2"
	colorRed200    = "#fecaca"
	colorRed700    = "#b91c1c"
	colorAmber600  = "#d97706"
	colorAmber50   = "#fffbeb"
	colorAmber100  = "#fef3c7"
	colorAmber200  = "#fde68a"
	colorAmber700  = "#92400e"
	colorGreen600  = "#16a34a"
	colorGreen50   = "#f0fdf4"
	colorGreen100  = "#dcfce7"
	colorGreen200  = "#bbf7d0"
	colorGreen700  = "#15803d"
)

type emailPalette struct {
	accentBg     string // header stripe + CTA button
	accentText   string // on accentBg
	badgeBg      string
	badgeText    string
	badgeBorder  string
	dotColor     string
	statusLabel  string
	msgBorder    string // left border on message box
}

func paletteFor(severity, status string) emailPalette {
	switch {
	case status == "resolved":
		return emailPalette{
			accentBg:    colorGreen600, accentText: "#ffffff",
			badgeBg: colorGreen100, badgeText: colorGreen700, badgeBorder: colorGreen200,
			dotColor: colorGreen600, statusLabel: "RESOLVED", msgBorder: colorGreen600,
		}
	case severity == "critical":
		return emailPalette{
			accentBg:    colorRed600, accentText: "#ffffff",
			badgeBg: colorRed100, badgeText: colorRed700, badgeBorder: colorRed200,
			dotColor: colorRed600, statusLabel: "CRITICAL", msgBorder: colorRed600,
		}
	case severity == "warning":
		return emailPalette{
			accentBg:    colorAmber600, accentText: "#ffffff",
			badgeBg: colorAmber100, badgeText: colorAmber700, badgeBorder: colorAmber200,
			dotColor: colorAmber600, statusLabel: "WARNING", msgBorder: colorAmber600,
		}
	default:
		return emailPalette{
			accentBg:    colorBlue600, accentText: "#ffffff",
			badgeBg: colorBlue100, badgeText: colorBlue700, badgeBorder: colorBlue100,
			dotColor: colorBlue600, statusLabel: "INFO", msgBorder: colorBlue600,
		}
	}
}

func fmtTime(s string) string {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC().Format("Jan 02, 2006 · 15:04 UTC")
	}
	return s
}

// buildHTMLEmail returns a professional HTML email matching the HealthOps light-mode UI.
func buildHTMLEmail(p NotificationPayload, dashboardURL string) string {
	pal := paletteFor(p.Severity, p.Status)

	serverRow := ""
	if p.Server != "" {
		serverRow = detailRow("Server", htmlEscape(p.Server), false)
	}
	resolvedRow := ""
	if p.ResolvedAt != "" {
		resolvedRow = detailRow("Resolved", htmlEscape(fmtTime(p.ResolvedAt)), false)
	}

	ctaBlock := ""
	if dashboardURL != "" {
		url := fmt.Sprintf("%s/incidents/%s", strings.TrimRight(dashboardURL, "/"), p.IncidentID)
		ctaBlock = fmt.Sprintf(`
	<tr><td style="padding:0 32px 32px">
		<div style="text-align:center">
			<a href="%s" style="display:inline-block;background:%s;color:%s;padding:12px 36px;border-radius:8px;text-decoration:none;font-size:14px;font-weight:600;letter-spacing:-0.1px;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif">
				View Incident in HealthOps &rarr;
			</a>
		</div>
	</td></tr>`, url, pal.accentBg, pal.accentText)
	}

	checkType := p.CheckType
	if checkType == "" {
		checkType = "api"
	}
	shortID := p.IncidentID
	if len(shortID) > 22 {
		shortID = shortID[:22] + "…"
	}

	year := time.Now().Year()

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en" xmlns="http://www.w3.org/1999/xhtml">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta http-equiv="X-UA-Compatible" content="IE=edge">
<title>HealthOps: %s</title>
</head>
<body style="margin:0;padding:0;background:%s;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Helvetica Neue',Arial,sans-serif;-webkit-font-smoothing:antialiased">

<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0">
<tr><td align="center" style="padding:32px 16px 48px">

	<!-- Card -->
	<table role="presentation" width="600" cellpadding="0" cellspacing="0" border="0"
	       style="max-width:600px;width:100%%;background:#ffffff;border-radius:12px;border:1px solid %s;overflow:hidden">

		<!-- ── ACCENT STRIPE ── -->
		<tr><td style="background:%s;height:4px;font-size:1px;line-height:1px">&nbsp;</td></tr>

		<!-- ── HEADER ── -->
		<tr><td style="padding:20px 28px 16px;border-bottom:1px solid %s">
			<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0">
			<tr>
				<td style="vertical-align:middle">
					<!-- Logo -->
					<table role="presentation" cellpadding="0" cellspacing="0" border="0">
					<tr>
						<td style="background:%s;border-radius:8px;width:32px;height:32px;text-align:center;vertical-align:middle;font-size:18px;line-height:32px;font-weight:900;color:#ffffff">
							&#9829;
						</td>
						<td style="padding-left:10px;vertical-align:middle">
							<span style="font-size:15px;font-weight:700;color:%s;letter-spacing:-0.3px">HealthOps</span>
						</td>
					</tr>
					</table>
				</td>
				<td align="right" style="vertical-align:middle">
					<!-- Severity badge -->
					<span style="display:inline-block;background:%s;color:%s;padding:4px 12px;border-radius:20px;font-size:11px;font-weight:700;letter-spacing:0.8px;border:1px solid %s;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif">
						%s
					</span>
				</td>
			</tr>
			</table>
		</td></tr>

		<!-- ── TITLE ── -->
		<tr><td style="padding:24px 28px 0">
			<table role="presentation" cellpadding="0" cellspacing="0" border="0">
			<tr>
				<td style="vertical-align:middle;padding-right:10px">
					<span style="display:inline-block;width:10px;height:10px;border-radius:50%%;background:%s"></span>
				</td>
				<td style="vertical-align:middle">
					<h1 style="margin:0;font-size:20px;font-weight:700;color:%s;letter-spacing:-0.4px;line-height:1.3">%s</h1>
				</td>
			</tr>
			</table>
			<p style="margin:8px 0 0 20px;font-size:12px;color:%s">
				Incident &nbsp;<span style="background:%s;color:%s;padding:1px 7px;border-radius:4px;font-family:'SFMono-Regular','Consolas','Courier New',monospace;font-size:11px;border:1px solid %s">%s</span>
			</p>
		</td></tr>

		<!-- ── MESSAGE ── -->
		<tr><td style="padding:16px 28px 0">
			<div style="background:%s;border:1px solid %s;border-left:3px solid %s;border-radius:6px;padding:12px 16px">
				<p style="margin:0;font-size:13px;color:%s;line-height:1.65;word-break:break-word">%s</p>
			</div>
		</td></tr>

		<!-- ── DETAILS TABLE ── -->
		<tr><td style="padding:20px 28px 0">
			<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0"
			       style="border:1px solid %s;border-radius:8px;overflow:hidden;font-size:13px">
				<!-- Header -->
				<tr style="background:%s">
					<td colspan="2" style="padding:8px 14px;border-bottom:1px solid %s">
						<span style="font-size:10px;font-weight:600;color:%s;letter-spacing:1px;text-transform:uppercase">Incident Details</span>
					</td>
				</tr>
				%s
				%s
				%s
				%s
				%s
				%s
				%s
			</table>
		</td></tr>

		<!-- ── CTA ── -->
		%s

		<!-- ── FOOTER ── -->
		<tr><td style="padding:20px 28px;background:%s;border-top:1px solid %s;border-radius:0 0 12px 12px">
			<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0">
			<tr>
				<td>
					<p style="margin:0;font-size:11px;color:%s;line-height:1.6">
						Sent by <strong style="color:%s">HealthOps Monitoring</strong><br>
						This is an automated alert &mdash; do not reply to this email.
					</p>
				</td>
				<td align="right" style="vertical-align:top">
					<p style="margin:0;font-size:11px;color:%s">&copy; %d HealthOps</p>
				</td>
			</tr>
			</table>
		</td></tr>

	</table>
</td></tr>
</table>
</body>
</html>`,
		// title
		htmlEscape(p.CheckName),
		// body bg
		colorSlate100,
		// card border
		colorSlate200,
		// accent stripe
		pal.accentBg,
		// header bottom border
		colorSlate200,
		// logo bg
		colorBlue600,
		// "HealthOps" text
		colorSlate900,
		// badge: bg, text, border, label
		pal.badgeBg, pal.badgeText, pal.badgeBorder, pal.statusLabel,
		// dot
		pal.dotColor,
		// h1: color, text
		colorSlate900, htmlEscape(p.CheckName),
		// incident ID pill: label color, pill bg, pill text, pill border, id
		colorSlate500, colorSlate100, colorSlate700, colorSlate200, htmlEscape(shortID),
		// message box: bg, border, left-accent, text
		colorSlate50, colorSlate200, pal.msgBorder, colorSlate700, htmlEscape(p.Message),
		// details table border
		colorSlate200,
		// details header bg, border, label color
		colorSlate50, colorSlate200, colorSlate500,
		// detail rows
		detailRow("Check", `<strong style="color:`+colorSlate900+`">`+htmlEscape(p.CheckName)+`</strong>`, true),
		detailRow("Type", `<span style="background:`+colorSlate100+`;color:`+colorSlate700+`;padding:1px 7px;border-radius:4px;font-family:'SFMono-Regular','Consolas',monospace;font-size:11px;border:1px solid `+colorSlate200+`">`+htmlEscape(checkType)+`</span>`, true),
		serverRow,
		detailRow("Severity", `<span style="background:`+pal.badgeBg+`;color:`+pal.badgeText+`;padding:2px 10px;border-radius:12px;font-size:11px;font-weight:700;letter-spacing:0.6px;border:1px solid `+pal.badgeBorder+`">`+htmlEscape(p.Severity)+`</span>`, true),
		detailRow("Started", htmlEscape(fmtTime(p.StartedAt)), true),
		resolvedRow,
		detailRow("Status", `<span style="font-weight:700;color:`+pal.accentBg+`">`+pal.statusLabel+`</span>`, false),
		// CTA
		ctaBlock,
		// footer bg, border, text, strong, copyright, year
		colorSlate50, colorSlate200, colorSlate400, colorSlate500, colorSlate400, year,
	)
}

// detailRow renders a single table row in the incident details table.
func detailRow(label, valueHTML string, bottomBorder bool) string {
	border := ""
	if bottomBorder {
		border = fmt.Sprintf("border-bottom:1px solid %s;", colorSlate100)
	}
	return fmt.Sprintf(`
			<tr>
				<td style="padding:9px 14px;font-size:12px;color:%s;white-space:nowrap;width:90px;%s">%s</td>
				<td style="padding:9px 14px;%s">%s</td>
			</tr>`, colorSlate500, border, label, border, valueHTML)
}

func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

// buildDigestHTMLEmail creates a clean multi-incident digest matching the app's light-mode UI.
func buildDigestHTMLEmail(payloads []NotificationPayload, critical, warning int, highest, dashboardURL string) string {
	pal := paletteFor(highest, "")

	var summaryParts []string
	if critical > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf(
			`<span style="display:inline-block;background:%s;color:%s;padding:4px 12px;border-radius:12px;font-size:11px;font-weight:700;border:1px solid %s;margin-right:6px">%d CRITICAL</span>`,
			colorRed100, colorRed700, colorRed200, critical))
	}
	if warning > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf(
			`<span style="display:inline-block;background:%s;color:%s;padding:4px 12px;border-radius:12px;font-size:11px;font-weight:700;border:1px solid %s;margin-right:6px">%d WARNING</span>`,
			colorAmber100, colorAmber700, colorAmber200, warning))
	}

	var rows string
	for _, p := range payloads {
		rpal := paletteFor(p.Severity, "")
		server := `<span style="color:`+colorSlate400+`">—</span>`
		if p.Server != "" {
			server = fmt.Sprintf(`<span style="color:%s">%s</span>`, colorSlate700, htmlEscape(p.Server))
		}
		rows += fmt.Sprintf(`
			<tr>
				<td style="padding:10px 14px;border-bottom:1px solid %s">
					<span style="display:inline-block;width:8px;height:8px;border-radius:50%%;background:%s;margin-right:8px;vertical-align:middle"></span>
					<span style="font-size:13px;font-weight:600;color:%s">%s</span>
				</td>
				<td style="padding:10px 14px;border-bottom:1px solid %s">
					<span style="background:%s;color:%s;padding:2px 9px;border-radius:10px;font-size:10px;font-weight:700;text-transform:uppercase;border:1px solid %s">%s</span>
				</td>
				<td style="padding:10px 14px;border-bottom:1px solid %s;font-size:12px;color:%s">%s</td>
				<td style="padding:10px 14px;border-bottom:1px solid %s;font-size:12px;color:%s;max-width:180px;word-break:break-word">%s</td>
			</tr>`,
			colorSlate100, rpal.dotColor, colorSlate900, htmlEscape(p.CheckName),
			colorSlate100, rpal.badgeBg, rpal.badgeText, rpal.badgeBorder, htmlEscape(p.Severity),
			colorSlate100, colorSlate700, server,
			colorSlate100, colorSlate500, htmlEscape(p.Message),
		)
	}

	ctaBlock := ""
	if dashboardURL != "" {
		url := strings.TrimRight(dashboardURL, "/") + "/incidents"
		ctaBlock = fmt.Sprintf(`
	<tr><td style="padding:0 28px 32px">
		<div style="text-align:center">
			<a href="%s" style="display:inline-block;background:%s;color:%s;padding:12px 36px;border-radius:8px;text-decoration:none;font-size:14px;font-weight:600;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif">
				View All Incidents &rarr;
			</a>
		</div>
	</td></tr>`, url, pal.accentBg, pal.accentText)
	}

	year := time.Now().Year()

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en" xmlns="http://www.w3.org/1999/xhtml">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>HealthOps Alert Digest</title>
</head>
<body style="margin:0;padding:0;background:%s;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Helvetica Neue',Arial,sans-serif;-webkit-font-smoothing:antialiased">
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0">
<tr><td align="center" style="padding:32px 16px 48px">

	<table role="presentation" width="640" cellpadding="0" cellspacing="0" border="0"
	       style="max-width:640px;width:100%%;background:#ffffff;border-radius:12px;border:1px solid %s;overflow:hidden">

		<tr><td style="background:%s;height:4px;font-size:1px;line-height:1px">&nbsp;</td></tr>

		<!-- Header -->
		<tr><td style="padding:20px 28px 16px;border-bottom:1px solid %s">
			<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0">
			<tr>
				<td style="vertical-align:middle">
					<table role="presentation" cellpadding="0" cellspacing="0" border="0">
					<tr>
						<td style="background:%s;border-radius:8px;width:32px;height:32px;text-align:center;vertical-align:middle;font-size:18px;line-height:32px;font-weight:900;color:#ffffff">&#9829;</td>
						<td style="padding-left:10px;vertical-align:middle">
							<span style="font-size:15px;font-weight:700;color:%s">HealthOps Alert Digest</span>
						</td>
					</tr>
					</table>
				</td>
				<td align="right" style="vertical-align:middle">
					<span style="display:inline-block;background:%s;color:#ffffff;padding:4px 12px;border-radius:20px;font-size:11px;font-weight:700;letter-spacing:0.5px">%d CHECKS FAILING</span>
				</td>
			</tr>
			</table>
		</td></tr>

		<!-- Summary -->
		<tr><td style="padding:20px 28px 16px">
			<p style="margin:0 0 12px;font-size:13px;color:%s;line-height:1.6">
				Multiple health checks are reporting failures. Here is a consolidated summary:
			</p>
			<div>%s</div>
		</td></tr>

		<!-- Table -->
		<tr><td style="padding:0 28px 4px">
			<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0"
			       style="border:1px solid %s;border-radius:8px;overflow:hidden;font-size:13px">
				<tr style="background:%s">
					<th style="padding:9px 14px;text-align:left;font-size:10px;font-weight:600;color:%s;text-transform:uppercase;letter-spacing:1px;border-bottom:1px solid %s">Check</th>
					<th style="padding:9px 14px;text-align:left;font-size:10px;font-weight:600;color:%s;text-transform:uppercase;letter-spacing:1px;border-bottom:1px solid %s">Severity</th>
					<th style="padding:9px 14px;text-align:left;font-size:10px;font-weight:600;color:%s;text-transform:uppercase;letter-spacing:1px;border-bottom:1px solid %s">Server</th>
					<th style="padding:9px 14px;text-align:left;font-size:10px;font-weight:600;color:%s;text-transform:uppercase;letter-spacing:1px;border-bottom:1px solid %s">Details</th>
				</tr>
				%s
			</table>
		</td></tr>

		%s

		<!-- Footer -->
		<tr><td style="padding:20px 28px;background:%s;border-top:1px solid %s;border-radius:0 0 12px 12px">
			<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0">
			<tr>
				<td>
					<p style="margin:0;font-size:11px;color:%s;line-height:1.6">
						Sent by <strong style="color:%s">HealthOps Monitoring</strong><br>
						This is an automated alert &mdash; do not reply to this email.
					</p>
				</td>
				<td align="right" style="vertical-align:top">
					<p style="margin:0;font-size:11px;color:%s">&copy; %d HealthOps</p>
				</td>
			</tr>
			</table>
		</td></tr>

	</table>
</td></tr>
</table>
</body>
</html>`,
		colorSlate100, colorSlate200,
		pal.accentBg,
		colorSlate200, colorBlue600, colorSlate900,
		pal.accentBg, len(payloads),
		colorSlate500, strings.Join(summaryParts, ""),
		colorSlate200, colorSlate50,
		colorSlate500, colorSlate200,
		colorSlate500, colorSlate200,
		colorSlate500, colorSlate200,
		colorSlate500, colorSlate200,
		rows,
		ctaBlock,
		colorSlate50, colorSlate200, colorSlate400, colorSlate500, colorSlate400, year,
	)
}
