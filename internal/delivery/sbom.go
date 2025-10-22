package delivery

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

// BuildSBOM aggregates manifest digests and selected metadata for release auditing.
func BuildSBOM(sourceType string, manifests []string, extras map[string]string) map[string]any {
	out := map[string]any{}
	if strings.TrimSpace(sourceType) != "" {
		out["sourceType"] = strings.TrimSpace(sourceType)
	}
	if len(manifests) > 0 {
		hashes := make([]map[string]string, 0, len(manifests))
		agg := sha256.New()
		for idx, doc := range manifests {
			sum := sha256.Sum256([]byte(doc))
			digest := hex.EncodeToString(sum[:])
			hashes = append(hashes, map[string]string{
				"index":  strconv.Itoa(idx),
				"sha256": digest,
			})
			agg.Write([]byte(doc))
		}
		out["manifests"] = hashes
		out["aggregateDigest"] = hex.EncodeToString(agg.Sum(nil))
	}
	for k, v := range extras {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		if strings.TrimSpace(v) == "" {
			continue
		}
		out[key] = v
	}
	return out
}
