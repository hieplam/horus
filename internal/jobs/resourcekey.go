package jobs

import (
	"sort"
	"strings"
)

// ResourceKey is the resource key of a workflow: the SHA-256 of the sorted
// SHA-256 values of its model files, as 64 hex characters. Two workflows
// that load the same model files have the same key, so a worker that holds
// the models of one workflow in memory holds them for the other. A workflow
// with no model file gets the key of its name ("workflow:<name>"), so that
// two such workflows do not share a key.
func ResourceKey(workflow string, modelSHA256 []string) string {
	sums := make([]string, 0, len(modelSHA256))
	for _, s := range modelSHA256 {
		if s = strings.ToLower(strings.TrimSpace(s)); s != "" {
			sums = append(sums, s)
		}
	}
	if len(sums) == 0 {
		return sha256Hex([]byte("workflow:" + workflow))
	}
	sort.Strings(sums)
	return sha256Hex([]byte(strings.Join(sums, "\n")))
}
