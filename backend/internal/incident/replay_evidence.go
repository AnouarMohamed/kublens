package incident

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"kubelens-backend/internal/model"
)

func BuildReplay(incident model.Incident, now func() time.Time) model.IncidentReplay {
	clock := now
	if clock == nil {
		clock = time.Now
	}

	startedAt := parseReplayTime(strings.TrimSpace(incident.OpenedAt), clock())
	endedAt := startedAt
	if strings.TrimSpace(incident.ResolvedAt) != "" {
		endedAt = parseReplayTime(strings.TrimSpace(incident.ResolvedAt), clock())
	} else if len(incident.Timeline) > 0 {
		endedAt = parseReplayTime(strings.TrimSpace(incident.Timeline[len(incident.Timeline)-1].Timestamp), clock())
	}

	frames := make([]model.IncidentReplayFrame, 0, len(incident.Timeline))
	for _, entry := range incident.Timeline {
		at := parseReplayTime(entry.Timestamp, startedAt)
		offset := int(at.Sub(startedAt).Minutes())
		if offset < 0 {
			offset = 0
		}
		frames = append(frames, model.IncidentReplayFrame{
			Timestamp:     at.UTC().Format(time.RFC3339),
			OffsetMinutes: offset,
			Kind:          entry.Kind,
			Source:        entry.Source,
			Summary:       entry.Summary,
			Resource:      entry.Resource,
			Severity:      entry.Severity,
		})
		if at.After(endedAt) {
			endedAt = at
		}
	}

	sort.SliceStable(frames, func(i, j int) bool {
		if frames[i].OffsetMinutes != frames[j].OffsetMinutes {
			return frames[i].OffsetMinutes < frames[j].OffsetMinutes
		}
		return frames[i].Timestamp < frames[j].Timestamp
	})

	return model.IncidentReplay{
		IncidentID:    strings.TrimSpace(incident.ID),
		IncidentTitle: strings.TrimSpace(incident.Title),
		Status:        incident.Status,
		GeneratedAt:   clock().UTC().Format(time.RFC3339),
		StartedAt:     startedAt.UTC().Format(time.RFC3339),
		EndedAt:       endedAt.UTC().Format(time.RFC3339),
		Duration:      formatReplayDuration(startedAt, endedAt),
		Frames:        frames,
	}
}

func BuildEvidenceBundle(
	incident model.Incident,
	audit []model.AuditEntry,
	remediations []model.RemediationProposal,
	postmortem *model.Postmortem,
	now func() time.Time,
) model.IncidentEvidenceBundle {
	clock := now
	if clock == nil {
		clock = time.Now
	}

	diagnostics := make([]model.TimelineEntry, 0, len(incident.Timeline))
	events := make([]model.TimelineEntry, 0, len(incident.Timeline))
	predictions := make([]model.TimelineEntry, 0, len(incident.Timeline))
	actions := make([]model.TimelineEntry, 0, len(incident.Timeline))
	for _, entry := range incident.Timeline {
		switch entry.Kind {
		case model.TimelineEntryKindDiagnostic:
			diagnostics = append(diagnostics, entry)
		case model.TimelineEntryKindEvent:
			events = append(events, entry)
		case model.TimelineEntryKindPrediction:
			predictions = append(predictions, entry)
		case model.TimelineEntryKindAction:
			actions = append(actions, entry)
		}
	}

	return model.IncidentEvidenceBundle{
		IncidentID:        strings.TrimSpace(incident.ID),
		IncidentTitle:     strings.TrimSpace(incident.Title),
		GeneratedAt:       clock().UTC().Format(time.RFC3339),
		Summary:           buildEvidenceSummary(incident, diagnostics, predictions, remediations, audit),
		AffectedResources: append([]string(nil), incident.AffectedResources...),
		Diagnostics:       diagnostics,
		Events:            events,
		Predictions:       predictions,
		Actions:           actions,
		Audit:             append([]model.AuditEntry(nil), audit...),
		Remediations:      cloneEvidenceRemediations(remediations),
		Postmortem:        cloneEvidencePostmortem(postmortem),
		Markdown:          renderEvidenceMarkdown(incident, diagnostics, events, predictions, actions, audit, remediations, postmortem),
	}
}

func buildEvidenceSummary(
	incident model.Incident,
	diagnostics []model.TimelineEntry,
	predictions []model.TimelineEntry,
	remediations []model.RemediationProposal,
	audit []model.AuditEntry,
) string {
	parts := []string{
		fmt.Sprintf("%d diagnostic findings", len(diagnostics)),
		fmt.Sprintf("%d predictive signals", len(predictions)),
		fmt.Sprintf("%d related audit entries", len(audit)),
	}
	if len(remediations) > 0 {
		parts = append(parts, fmt.Sprintf("%d linked remediations", len(remediations)))
	}
	status := strings.ToUpper(string(incident.Status))
	return fmt.Sprintf("%s incident evidence bundle with %s.", status, strings.Join(parts, ", "))
}

func renderEvidenceMarkdown(
	incident model.Incident,
	diagnostics []model.TimelineEntry,
	events []model.TimelineEntry,
	predictions []model.TimelineEntry,
	actions []model.TimelineEntry,
	audit []model.AuditEntry,
	remediations []model.RemediationProposal,
	postmortem *model.Postmortem,
) string {
	lines := []string{
		fmt.Sprintf("# Incident Evidence Bundle — %s", strings.TrimSpace(incident.Title)),
		"",
		fmt.Sprintf("- Incident ID: %s", strings.TrimSpace(incident.ID)),
		fmt.Sprintf("- Status: %s", strings.ToUpper(string(incident.Status))),
		fmt.Sprintf("- Severity: %s", strings.ToUpper(strings.TrimSpace(incident.Severity))),
		fmt.Sprintf("- Opened: %s", strings.TrimSpace(incident.OpenedAt)),
	}
	if strings.TrimSpace(incident.ResolvedAt) != "" {
		lines = append(lines, fmt.Sprintf("- Resolved: %s", strings.TrimSpace(incident.ResolvedAt)))
	}
	lines = append(
		lines,
		"",
		"## Summary",
		defaultSectionValue(strings.TrimSpace(incident.Summary), "No summary available."),
		"",
		"## Affected Resources",
	)
	lines = append(lines, toBulletList(incident.AffectedResources)...)

	lines = append(lines, "", "## Diagnostics")
	lines = append(lines, timelineEntriesToMarkdown(diagnostics)...)

	lines = append(lines, "", "## Events")
	lines = append(lines, timelineEntriesToMarkdown(events)...)

	lines = append(lines, "", "## Predictions")
	lines = append(lines, timelineEntriesToMarkdown(predictions)...)

	lines = append(lines, "", "## Actions")
	lines = append(lines, timelineEntriesToMarkdown(actions)...)

	lines = append(lines, "", "## Audit Trail")
	if len(audit) == 0 {
		lines = append(lines, "- None")
	} else {
		for _, entry := range audit {
			label := strings.TrimSpace(entry.Action)
			if label == "" {
				label = strings.TrimSpace(entry.Method + " " + entry.Path)
			}
			lines = append(lines, fmt.Sprintf("- %s | %s | %d", strings.TrimSpace(entry.Timestamp), label, entry.Status))
		}
	}

	lines = append(lines, "", "## Remediations")
	if len(remediations) == 0 {
		lines = append(lines, "- None")
	} else {
		for _, proposal := range remediations {
			resource := strings.TrimSpace(proposal.Resource)
			if ns := strings.TrimSpace(proposal.Namespace); ns != "" {
				resource = ns + "/" + resource
			}
			lines = append(lines, fmt.Sprintf("- [%s] %s — %s", strings.ToUpper(proposal.Status), resource, strings.TrimSpace(proposal.Reason)))
		}
	}

	lines = append(lines, "", "## Postmortem")
	if postmortem == nil {
		lines = append(lines, "- Not generated")
	} else {
		lines = append(
			lines,
			fmt.Sprintf("- Method: %s", strings.ToUpper(string(postmortem.Method))),
			fmt.Sprintf("- Root cause: %s", defaultSectionValue(strings.TrimSpace(postmortem.RootCause), "Unavailable")),
			fmt.Sprintf("- Prevention: %s", defaultSectionValue(strings.TrimSpace(postmortem.Prevention), "Unavailable")),
		)
	}

	return strings.Join(lines, "\n")
}

func timelineEntriesToMarkdown(entries []model.TimelineEntry) []string {
	if len(entries) == 0 {
		return []string{"- None"}
	}

	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		summary := defaultSectionValue(strings.TrimSpace(entry.Summary), "No summary")
		if resource := strings.TrimSpace(entry.Resource); resource != "" {
			out = append(out, fmt.Sprintf("- %s | %s | %s", strings.TrimSpace(entry.Timestamp), summary, resource))
			continue
		}
		out = append(out, fmt.Sprintf("- %s | %s", strings.TrimSpace(entry.Timestamp), summary))
	}
	return out
}

func toBulletList(items []string) []string {
	if len(items) == 0 {
		return []string{"- None"}
	}

	out := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		out = append(out, "- "+trimmed)
	}
	if len(out) == 0 {
		return []string{"- None"}
	}
	return out
}

func defaultSectionValue(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func parseReplayTime(raw string, fallback time.Time) time.Time {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(raw))
	if err != nil {
		return fallback.UTC()
	}
	return parsed.UTC()
}

func formatReplayDuration(start, end time.Time) string {
	if end.Before(start) {
		return "0 minutes"
	}
	delta := end.Sub(start)
	hours := int(delta.Hours())
	minutes := int(delta.Minutes()) % 60
	if hours <= 0 {
		return fmt.Sprintf("%d minutes", int(delta.Minutes()))
	}
	return fmt.Sprintf("%d hours %d minutes", hours, minutes)
}

func cloneEvidenceRemediations(items []model.RemediationProposal) []model.RemediationProposal {
	out := make([]model.RemediationProposal, len(items))
	copy(out, items)
	return out
}

func cloneEvidencePostmortem(item *model.Postmortem) *model.Postmortem {
	if item == nil {
		return nil
	}
	cloned := *item
	cloned.Timeline = append([]model.TimelineEntry(nil), item.Timeline...)
	cloned.Runbook = append([]model.RunbookStep(nil), item.Runbook...)
	return &cloned
}
