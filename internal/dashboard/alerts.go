package dashboard

import "github.com/LDTorres/redis-stats/internal/redisstats"

type alertView struct {
	ID       string `json:"id"`
	Category string `json:"category"`
	Severity string `json:"severity"`
	Title    string `json:"title"`
	Message  string `json:"message"`
}

func prioritizedAlerts(report redisstats.Report, scope string) []alertView {
	alerts := make([]alertView, 0, 6)

	if report.MemoryTrend != nil && report.MemoryTrend.SustainedGrowth {
		alerts = append(alerts, alertView{
			ID:       "memory-trend",
			Category: "memory",
			Severity: "high",
			Title:    "Sustained memory growth",
			Message:  "used_memory is climbing consistently across the recent window.",
		})
	} else if report.Delta != nil && report.Delta.UsedMemory > 1<<20 {
		alerts = append(alerts, alertView{
			ID:       "memory-delta",
			Category: "memory",
			Severity: "medium",
			Title:    "Memory jumped in the latest sample",
			Message:  "The latest interval showed a material used_memory increase.",
		})
	}

	switch {
	case report.Snapshot.Memory.FragmentationRatio >= 2:
		alerts = append(alerts, alertView{
			ID:       "fragmentation-high",
			Category: "fragmentation",
			Severity: "high",
			Title:    "Fragmentation is elevated",
			Message:  "RSS is materially above logical memory usage.",
		})
	case report.Snapshot.Memory.FragmentationRatio >= 1.5:
		alerts = append(alerts, alertView{
			ID:       "fragmentation-medium",
			Category: "fragmentation",
			Severity: "medium",
			Title:    "Fragmentation is above target",
			Message:  "Resident memory is running above the expected overhead range.",
		})
	}

	if report.Snapshot.Clients.BlockedClients > 0 {
		alerts = append(alerts, alertView{
			ID:       "blocked-clients",
			Category: "connections",
			Severity: "medium",
			Title:    "Blocked clients detected",
			Message:  "This can be normal for blocking commands, but confirm it matches workload intent.",
		})
	}
	if report.Delta != nil && report.Delta.RejectedConnections > 0 {
		alerts = append(alerts, alertView{
			ID:       "rejected-connections",
			Category: "connections",
			Severity: "high",
			Title:    "Rejected connections increased",
			Message:  "Redis rejected new clients in the latest interval.",
		})
	}

	ratio := hitRatio(report.Snapshot)
	switch {
	case ratio > 0 && ratio < 0.40:
		alerts = append(alerts, alertView{
			ID:       "cache-hit-ratio-high",
			Category: "cache-efficiency",
			Severity: "high",
			Title:    "Cache hit ratio is poor",
			Message:  "If this Redis instance is used primarily as a cache, the miss rate is too high.",
		})
	case ratio > 0 && ratio < 0.80:
		alerts = append(alerts, alertView{
			ID:       "cache-hit-ratio-medium",
			Category: "cache-efficiency",
			Severity: "medium",
			Title:    "Cache hit ratio is below target",
			Message:  "This matters mainly when the workload is cache-heavy rather than queue/session oriented.",
		})
	}

	if severity := pingSeverity(report.Snapshot.PingLatency.Milliseconds(), scope); severity != "" {
		title := "Ping latency is elevated"
		message := "Round-trip time is above the local baseline."
		if scope == "remote" {
			title = "Ping likely reflects network distance"
			message = "For remote environments, ping includes WAN and TLS overhead, so it is a lower-priority signal."
		}
		alerts = append(alerts, alertView{
			ID:       "ping-latency",
			Category: "latency",
			Severity: severity,
			Title:    title,
			Message:  message,
		})
	}

	return sortAlerts(alerts)
}

func pingSeverity(latencyMS int64, scope string) string {
	switch scope {
	case "local", "private":
		switch {
		case latencyMS >= 150:
			return "high"
		case latencyMS >= 50:
			return "medium"
		}
	case "remote":
		switch {
		case latencyMS >= 300:
			return "medium"
		case latencyMS >= 120:
			return "low"
		}
	}
	return ""
}

func sortAlerts(alerts []alertView) []alertView {
	severityWeight := map[string]int{"high": 0, "medium": 1, "low": 2}
	categoryWeight := map[string]int{
		"memory":           0,
		"fragmentation":    1,
		"connections":      2,
		"cache-efficiency": 3,
		"latency":          4,
	}

	for i := range alerts {
		for j := i + 1; j < len(alerts); j++ {
			left, right := alerts[i], alerts[j]
			if severityWeight[right.Severity] < severityWeight[left.Severity] ||
				(severityWeight[right.Severity] == severityWeight[left.Severity] && categoryWeight[right.Category] < categoryWeight[left.Category]) {
				alerts[i], alerts[j] = alerts[j], alerts[i]
			}
		}
	}

	return alerts
}
