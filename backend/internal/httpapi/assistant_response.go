package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"kubelens-backend/internal/ai"
	"kubelens-backend/internal/model"
	"kubelens-backend/internal/redact"
)

func (s *Server) writeAssistantResponse(w http.ResponseWriter, reqCtx context.Context, ctx assistantContext) {
	answer := strings.TrimSpace(ctx.localAnswer)
	if s.ai != nil {
		if enhanced, err := s.enhanceAssistantAnswer(reqCtx, ctx); err == nil && strings.TrimSpace(enhanced) != "" {
			answer = enhanced
		} else if err != nil && s.logger != nil {
			s.logger.Warn("assistant provider fallback",
				"provider", s.aiName(),
				"error", redact.Error(err),
			)
		}
	}

	writeJSON(w, http.StatusOK, s.assistantResponse(answer, ctx.hints, ctx.resources, ctx.docReferences))

	summary := strings.TrimSpace(answer)
	if len(summary) > 320 {
		summary = summary[:320] + "..."
	}
	if summary != "" {
		s.notifyChatOps(func(chatCtx context.Context) {
			if s.chatops != nil {
				s.chatops.NotifyAssistantFinding(chatCtx, summary, ctx.resources)
			}
		})
	}
}

func (s *Server) enhanceAssistantAnswer(ctx context.Context, c assistantContext) (string, error) {
	if s.ai == nil {
		return "", fmt.Errorf("provider not configured")
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, s.aiTTL)
	defer cancel()

	enrichedContext := s.buildEnrichedContext(timeoutCtx, c)
	in := ai.Input{
		UserMessage:          c.userMessage,
		Intent:               c.intent,
		SystemContext:        c.promptContext,
		LocalAnswer:          c.localAnswer,
		DiagnosticsSummary:   c.diagnosticsSummary,
		Diagnostics:          c.diagnosticBriefs,
		PriorityActions:      c.priorityActions,
		ReferencedResources:  dedupeStrings(c.resources),
		ClusterSnapshotBrief: buildClusterSnapshotBrief(c.pods, c.nodes),
		DocumentationContext: buildDocumentationContext(c.docReferences),
		DocumentationRefs:    mapDocReferencesForAI(c.docReferences),
		EnrichedContext:      enrichedContext,
	}

	if toolingProvider, ok := s.ai.(ai.ToolingProvider); ok {
		return s.generateAssistantWithTools(timeoutCtx, toolingProvider, in)
	}

	answer, err := s.ai.Generate(timeoutCtx, in)
	if err != nil {
		return "", err
	}
	return answer, nil
}

func (s *Server) aiName() string {
	if s.ai == nil {
		return "none"
	}
	return s.ai.Name()
}

func buildClusterSnapshotBrief(pods []model.PodSummary, nodes []model.NodeSummary) string {
	var running, pending, failed int
	for _, pod := range pods {
		switch pod.Status {
		case model.PodStatusRunning:
			running++
		case model.PodStatusPending:
			pending++
		case model.PodStatusFailed:
			failed++
		}
	}

	var ready, notReady int
	for _, node := range nodes {
		if node.Status == model.NodeStatusReady {
			ready++
		}
		if node.Status == model.NodeStatusNotReady {
			notReady++
		}
	}

	return fmt.Sprintf(
		"pods=%d (running=%d pending=%d failed=%d), nodes=%d (ready=%d notReady=%d)",
		len(pods), running, pending, failed, len(nodes), ready, notReady,
	)
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	out := make([]string, 0, len(values))
	for _, v := range values {
		if strings.TrimSpace(v) == "" {
			continue
		}
		if slices.Contains(out, v) {
			continue
		}
		out = append(out, v)
	}
	return out
}

func (s *Server) assistantResponse(
	answer string,
	hints []string,
	resources []string,
	docRefs []model.DocumentationReference,
) model.AssistantResponse {
	return model.AssistantResponse{
		Answer:              answer,
		Hints:               append([]string(nil), hints...),
		ReferencedResources: append([]string(nil), resources...),
		References:          append([]model.DocumentationReference(nil), docRefs...),
		Timestamp:           s.now().UTC().Format(time.RFC3339),
	}
}
