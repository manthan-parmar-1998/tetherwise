package report

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/manthan-parmar-1998/tetherwise/internal/check"
	"github.com/manthan-parmar-1998/tetherwise/internal/model"
)

// Markdown renders a pull-request comment.
func Markdown(result check.Result) string {
	var b strings.Builder
	b.WriteString("## tetherwise\n\n")
	fmt.Fprintf(&b, "**Verdict:** %s\n\n", result.Verdict)
	b.WriteString("This report names who else depends on the shared resources a change reaches, and the headroom on those resources.\n\n")
	writeChanges(&b, result)
	writeAffected(&b, result)
	writeShared(&b, result)
	b.WriteString("### Why\n\n")
	for _, reason := range result.Reasons {
		fmt.Fprintf(&b, "- %s\n", reason)
	}
	b.WriteString("\n### Coverage\n\n")
	fmt.Fprintf(&b, "- Sources loaded: %s\n", joinOrNone(result.Coverage.SourcesLoaded))
	fmt.Fprintf(&b, "- Edge sources: %s\n", joinOrNone(result.Coverage.EdgeSources))
	fmt.Fprintf(&b, "- Resources with no owner: %s\n", joinOrNone(result.Coverage.ResourcesWithoutOwner))
	fmt.Fprintf(&b, "- Low-confidence edges: %s\n", joinOrNone(result.Coverage.LowConfidenceEdges))
	return b.String()
}

func writeChanges(b *strings.Builder, result check.Result) {
	b.WriteString("### Change\n\n")
	if len(result.Changes) == 0 {
		b.WriteString("No resource changes were in the plan.\n\n")
		return
	}
	b.WriteString("| Resource | Action | Attributes |\n")
	b.WriteString("| --- | --- | --- |\n")
	for _, change := range result.Changes {
		attrs := strings.Join(change.ChangedAttrs, ", ")
		if attrs == "" {
			attrs = "-"
		}
		fmt.Fprintf(b, "| %s | %s | %s |\n", cell(change.Resource), cell(string(change.Action)), cell(attrs))
	}
	b.WriteString("\n")
}

func writeAffected(b *strings.Builder, result check.Result) {
	b.WriteString("### Who else is exposed\n\n")
	if len(result.Affected) == 0 {
		b.WriteString("No other service depends on a shared resource this change reaches.\n\n")
		return
	}
	b.WriteString("| Service | Team | Owner | Shared resource | Relationship | Confidence | Headroom |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- | --- |\n")
	for _, item := range result.Affected {
		fmt.Fprintf(b, "| %s | %s | %s | %s | %s | %s | %s |\n",
			cell(item.Name),
			cell(orDash(item.Team)),
			cell(orDash(item.Owner)),
			cell(item.SharedResource),
			cell(item.DependencyKind),
			cell(string(item.Confidence)),
			cell(formatTightest(item.Headroom)),
		)
	}
	b.WriteString("\n")
}

func writeShared(b *strings.Builder, result check.Result) {
	b.WriteString("### Shared resources\n\n")
	if len(result.Shared) == 0 {
		b.WriteString("No shared resource is in the blast radius.\n\n")
		return
	}
	b.WriteString("| Resource | Teams | Headroom |\n")
	b.WriteString("| --- | --- | --- |\n")
	for _, item := range result.Shared {
		teams := strings.Join(item.Teams, ", ")
		if teams == "" {
			teams = "-"
		}
		fmt.Fprintf(b, "| %s | %s | %s |\n", cell(item.Resource), cell(teams), cell(formatLimits(item.Limits)))
	}
	b.WriteString("\n")
}

// JSON renders report.json.
func JSON(result check.Result) ([]byte, error) {
	if result.SchemaVersion == "" {
		result.SchemaVersion = check.SchemaVersion
	}
	return model.Marshal(result)
}

func formatTightest(limits []model.Limit) string {
	if len(limits) == 0 {
		return "unknown"
	}
	best := limits[0]
	bestScore := score(best)
	for _, limit := range limits[1:] {
		got := score(limit)
		if got < bestScore || (got == bestScore && limit.Metric < best.Metric) {
			best = limit
			bestScore = got
		}
	}
	return formatLimit(best)
}

func formatLimits(limits []model.Limit) string {
	if len(limits) == 0 {
		return "unknown"
	}
	parts := make([]string, len(limits))
	for i, limit := range limits {
		parts[i] = formatLimit(limit)
	}
	return strings.Join(parts, "; ")
}

func formatLimit(limit model.Limit) string {
	unit := ""
	if limit.Unit != "" {
		unit = " " + limit.Unit
	}
	return fmt.Sprintf("%s %s (%s/%s%s)", limit.Metric, formatNum(limit.Headroom), formatNum(limit.Current), formatNum(limit.Threshold), unit)
}

func score(limit model.Limit) float64 {
	denom := limit.Threshold
	if denom < 0 {
		denom = -denom
	}
	if denom == 0 {
		denom = 1
	}
	return limit.Headroom / denom
}

func formatNum(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func joinOrNone(items []string) string {
	if len(items) == 0 {
		return "none"
	}
	return strings.Join(items, ", ")
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func cell(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "|", "\\|")
	return s
}
