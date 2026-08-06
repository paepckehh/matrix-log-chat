// Package transform converts parsed syslog events into short, chat-friendly
// strings decorated with emoji that signal severity and facility at a glance.
// The output is plain UTF-8 text suitable for Matrix's m.room.message
// (msgtype m.text) body.
package transform

import (
	"fmt"
	"strings"

	"paepcke.de/matrix-log-chat/internal/syslog"
)

// severityEmoji maps a syslog severity to an emoji prefix that conveys
// urgency in a chat context.
var severityEmoji = map[int]string{
	syslog.SevEmergency: "🔴🚨",
	syslog.SevAlert:     "🚨",
	syslog.SevCritical:  "🟥",
	syslog.SevError:     "❌",
	syslog.SevWarning:   "⚠️",
	syslog.SevNotice:    "📢",
	syslog.SevInfo:      "ℹ️",
	syslog.SevDebug:     "🐛",
}

// facilityEmoji gives each facility its own small icon so a room reader can
// tell where a line originated before reading any text.
var facilityEmoji = map[int]string{
	syslog.FacKern:     "🐧",
	syslog.FacUser:     "👤",
	syslog.FacMail:     "📧",
	syslog.FacDaemon:   "⚙️",
	syslog.FacAuth:     "🔐",
	syslog.FacSyslog:   "📜",
	syslog.FacLPR:      "🖨️",
	syslog.FacNews:     "📰",
	syslog.FacUUCP:     "📦",
	syslog.FacClock:    "🕐",
	syslog.FacAuthPriv: "🛡️",
	syslog.FacFTP:      "📁",
	syslog.FacNTP:      "🕒",
	syslog.FacLogAudit: "🧾",
	syslog.FacLogAlert: "📣",
	syslog.FacLocal0:   "🟦",
	syslog.FacLocal1:   "🟩",
	syslog.FacLocal2:   "🟨",
	syslog.FacLocal3:   "🟧",
	syslog.FacLocal4:   "🟥",
	syslog.FacLocal5:   "🟪",
	syslog.FacLocal6:   "🟫",
	syslog.FacLocal7:   "⬜",
}

// ToChat renders an Event as a single chat line. The shape is intentionally
// fixed so a reader learns to scan the leading icon triple quickly:
//
//	[severity] [facility] <app[pid]> message  [@host] [· timestamp]
//
// Raw (non-syslog) input is forwarded verbatim so the bridge doubles as a
// generic stdin-to-Matrix pipe.
func ToChat(e syslog.Event) string {
	if e.Format == "raw" {
		return e.Message
	}

	var b strings.Builder
	// Severity emoji first: in a noisy room this is the signal that a
	// human eye locks onto.
	if se, ok := severityEmoji[e.Severity]; ok {
		b.WriteString(se)
		b.WriteByte(' ')
	} else {
		b.WriteString("❔ ")
	}
	if fe, ok := facilityEmoji[e.Facility]; ok {
		b.WriteString(fe)
		b.WriteByte(' ')
	}

	// Source identity: app and optional pid, falling back to hostname.
	if e.App != "" {
		b.WriteByte('<')
		b.WriteString(e.App)
		if e.PID != "" {
			b.WriteByte('[')
			b.WriteString(e.PID)
			b.WriteByte(']')
		}
		b.WriteByte('>')
	} else if e.Hostname != "" {
		b.WriteByte('<')
		b.WriteString(e.Hostname)
		b.WriteByte('>')
	}
	b.WriteByte(' ')

	b.WriteString(strings.TrimSpace(e.Message))

	// Trailing provenance block, only when it adds information.
	tail := make([]string, 0, 2)
	if e.Hostname != "" && e.App != "" {
		tail = append(tail, "@"+e.Hostname)
	}
	if !e.Timestamp.IsZero() {
		tail = append(tail, "· "+e.Timestamp.Format("15:04:05"))
	}
	if len(tail) > 0 {
		b.WriteString("  ")
		b.WriteString(strings.Join(tail, " "))
	}
	return b.String()
}

// Summary returns a one-line human description of the config the bridge is
// about to run with, written for the operator at startup.
func Summary(homeserver, user, room string, dryRun bool) string {
	mode := "LIVE"
	if dryRun {
		mode = "DRY-RUN"
	}
	return fmt.Sprintf(
		"matrix-log-chat [%s] → %s as %s in room %s",
		mode, homeserver, user, room,
	)
}
