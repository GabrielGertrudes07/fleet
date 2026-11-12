package notify

import "strings"

// Closing markers of an autonomous session's final reply.
const (
	MarkerNone       = ""
	MarkerDone       = "done"        // "result:" or "failed:"
	MarkerNeedsInput = "needs input" // "needs input:"
)

// Marker reports which closing marker the reply carries: a line that starts
// (case-insensitively) with "needs input:", "result:" or "failed:". Needing
// input wins over the others.
func Marker(reply string) string {
	found := MarkerNone
	for _, l := range strings.Split(reply, "\n") {
		t := strings.ToLower(strings.TrimLeft(strings.TrimSpace(l), "*_`> "))
		switch {
		case strings.HasPrefix(t, "needs input:"):
			return MarkerNeedsInput
		case strings.HasPrefix(t, "result:"), strings.HasPrefix(t, "failed:"):
			found = MarkerDone
		}
	}
	return found
}
