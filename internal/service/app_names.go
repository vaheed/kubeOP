package service

import (
	"crypto/sha1"
	"encoding/base32"
	"encoding/hex"
	"strings"

	"kubeop/internal/store"
	"kubeop/internal/util"
)

var (
	base32LowerNoPad = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567").WithPadding(base32.NoPadding)
)

const (
	maxKubeNameLen = 63
)

func resourceSuffix(appID string) string {
	if strings.TrimSpace(appID) == "" {
		return ""
	}
	sum := sha1.Sum([]byte(appID))
	hexPart := hex.EncodeToString(sum[:5])
	if len(hexPart) > 9 {
		hexPart = hexPart[:9]
	}
	base32Part := strings.ToLower(base32LowerNoPad.EncodeToString(sum[5:8]))
	if hexPart == "" {
		return base32Part
	}
	if base32Part == "" {
		return hexPart
	}
	return hexPart + "-" + base32Part
}

func deriveKubeName(baseName, appID string) string {
	base := util.Slugify(baseName)
	if base == "" {
		base = "app"
	}
	suffix := resourceSuffix(appID)
	if suffix == "" {
		if len(base) > maxKubeNameLen {
			return base[:maxKubeNameLen]
		}
		return base
	}
	maxBaseLen := maxKubeNameLen - len(suffix) - 1
	if maxBaseLen < 1 {
		maxBaseLen = 1
	}
	if len(base) > maxBaseLen {
		base = strings.Trim(base[:maxBaseLen], "-")
		if base == "" {
			base = "app"
		}
	}
	return base + "-" + suffix
}

func appKubeName(app store.App) string {
	if slug, ok := app.Source["kubeName"].(string); ok {
		slug = strings.TrimSpace(slug)
		if slug != "" {
			return slug
		}
	}
	return util.Slugify(app.Name)
}

// KubeNameForTest exposes kube name derivation for unit tests.
func KubeNameForTest(baseName, appID string) string {
	return deriveKubeName(baseName, appID)
}

// AppKubeNameForTest exposes lookup of the stored kube name for tests.
func AppKubeNameForTest(app store.App) string {
	return appKubeName(app)
}
