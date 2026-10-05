package prover

import (
	"regexp"
	"strconv"
	"strings"
)

// Obligation is an unresolved proof obligation returned to the caller.
type Obligation struct {
	Line          int    `json:"line"`
	Status        string `json:"status"`
	FailureReason string `json:"failure_reason,omitempty"`
	id            int
}

type toolboxEvent struct {
	fields     map[string]string
	hasBackend bool
}

var obligationLocationRe = regexp.MustCompile(`^([0-9]+):[0-9]+:[0-9]+:[0-9]+$`)

func parseToolboxObligations(output string) ([]Obligation, *int) {
	var current *toolboxEvent
	var count *int
	indices := make(map[int]int)
	obligations := make([]Obligation, 0)

	for _, rawLine := range strings.Split(output, "\n") {
		line := strings.TrimSpace(rawLine)
		switch {
		case line == "@!!BEGIN":
			if current != nil {
				count = consumeToolboxEvent(current, indices, &obligations, count)
			}
			current = &toolboxEvent{fields: make(map[string]string)}
		case line == "@!!END" && current != nil:
			count = consumeToolboxEvent(current, indices, &obligations, count)
			current = nil
		case current != nil && strings.HasPrefix(line, "@!!"):
			key, value, ok := strings.Cut(strings.TrimPrefix(line, "@!!"), ":")
			if !ok {
				continue
			}
			switch key {
			case "prover":
				current.hasBackend = true
			case "obl", "method", "meth", "already":
				continue
			default:
				current.fields[key] = strings.TrimSpace(value)
			}
		}
	}
	if current != nil {
		count = consumeToolboxEvent(current, indices, &obligations, count)
	}

	return obligations, count
}

func consumeToolboxEvent(event *toolboxEvent, indices map[int]int, obligations *[]Obligation, count *int) *int {
	if event.fields["type"] == "obligationsnumber" {
		if n, err := strconv.Atoi(event.fields["count"]); err == nil {
			return &n
		}
		return count
	}
	if event.fields["type"] != "obligation" {
		return count
	}

	id, err := strconv.Atoi(event.fields["id"])
	if err != nil {
		return count
	}
	index, ok := indices[id]
	if !ok {
		index = len(*obligations)
		indices[id] = index
		*obligations = append(*obligations, Obligation{id: id})
	}
	obligation := &(*obligations)[index]
	if match := obligationLocationRe.FindStringSubmatch(event.fields["loc"]); match != nil {
		obligation.Line, _ = strconv.Atoi(match[1])
	}

	rawStatus := strings.ToLower(strings.TrimSpace(event.fields["status"]))
	if rawStatus != "" && (obligation.Status == "" || event.hasBackend || rawStatus == "trivial") {
		obligation.Status = normalizeObligationStatus(rawStatus)
		if obligation.Status == "failed" {
			obligation.FailureReason = event.fields["reason"]
			if obligation.FailureReason == "" {
				obligation.FailureReason = "TLAPM reported that the obligation failed"
			}
		}
	}
	return count
}

func normalizeObligationStatus(status string) string {
	switch status {
	case "proved", "trivial", "failed", "timeout", "parse-error", "backend-error":
		return status
	case "being proved", "interrupted":
		return "timeout"
	case "to be proved", "normalized":
		return "pending"
	default:
		return "backend-error"
	}
}

func finalizeObligations(obligations []Obligation, success bool, failureReason string) {
	for i := range obligations {
		obligation := &obligations[i]
		if obligation.Status == "pending" {
			if success {
				obligation.Status = "proved"
			} else {
				obligation.Status = "backend-error"
				obligation.FailureReason = failureReason
				if obligation.FailureReason == "" {
					obligation.FailureReason = "TLAPM ended before reporting a final result"
				}
			}
		}
		if obligation.Status == "timeout" && obligation.FailureReason == "" {
			obligation.FailureReason = "backend did not finish before TLAPM stopped waiting"
		}
	}
}

func unresolvedObligations(obligations []Obligation) []Obligation {
	unresolved := make([]Obligation, 0)
	for _, obligation := range obligations {
		if obligation.Status != "proved" && obligation.Status != "trivial" {
			unresolved = append(unresolved, obligation)
		}
	}
	return unresolved
}

func countObligationsWithStatus(obligations []Obligation, statuses ...string) int {
	wanted := make(map[string]bool, len(statuses))
	for _, status := range statuses {
		wanted[status] = true
	}
	count := 0
	for _, obligation := range obligations {
		if wanted[obligation.Status] {
			count++
		}
	}
	return count
}

func hasStructuredObligations(obligations []Obligation) bool {
	for _, obligation := range obligations {
		if obligation.id > 0 {
			return true
		}
	}
	return false
}
