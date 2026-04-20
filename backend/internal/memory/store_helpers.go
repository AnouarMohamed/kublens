package memory

import (
	"os"
	"strconv"
	"strings"

	"kubelens-backend/internal/model"
)

func normalizeRunbookRequest(req model.MemoryRunbookUpsertRequest) (model.MemoryRunbook, error) {
	title := strings.TrimSpace(req.Title)
	description := strings.TrimSpace(req.Description)
	if title == "" || description == "" {
		return model.MemoryRunbook{}, os.ErrInvalid
	}
	tags := dedupeStrings(req.Tags)
	steps := dedupeStrings(req.Steps)
	if len(steps) == 0 {
		return model.MemoryRunbook{}, os.ErrInvalid
	}
	return model.MemoryRunbook{
		ID:          "",
		Title:       title,
		Tags:        tags,
		Description: description,
		Steps:       steps,
		UsageCount:  0,
		CreatedAt:   "",
		UpdatedAt:   "",
	}, nil
}

func dedupeStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		key := strings.ToLower(normalized)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

func cloneRunbook(in model.MemoryRunbook) model.MemoryRunbook {
	out := in
	out.Tags = append([]string(nil), in.Tags...)
	out.Steps = append([]string(nil), in.Steps...)
	out.Embedding = append([]float32(nil), in.Embedding...)
	return out
}

func runbookEmbeddingText(runbook model.MemoryRunbook) string {
	parts := []string{
		strings.TrimSpace(runbook.Title),
		strings.TrimSpace(runbook.Description),
		strings.Join(runbook.Tags, " "),
		strings.Join(runbook.Steps, " "),
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func cloneFix(in model.MemoryFixPattern) model.MemoryFixPattern {
	out := in
	return out
}

func formatCounter(counter uint64) string {
	return strconv.FormatUint(counter, 10)
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
