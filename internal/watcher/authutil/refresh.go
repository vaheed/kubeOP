package authutil

import "time"

const (
	// minAccessRefreshMargin ensures we always refresh at least this duration before
	// the server-side expiry time to tolerate clock skew.
	minAccessRefreshMargin = 30 * time.Second
	// minAccessRefreshDelay avoids tight refresh loops when the observed lifetime is
	// extremely small while still allowing immediate refresh when the token is
	// effectively expired.
	minAccessRefreshDelay = 15 * time.Second
)

// NextAccessRefresh calculates when the watcher should proactively refresh its access
// token based on the server-provided expiry timestamp. The returned time is always in
// the future (or equal to now) and ensures refresh happens well before the token would
// expire according to the local clock, handling potential clock skew between the
// watcher node and the kubeOP API.
func NextAccessRefresh(now, expires time.Time) time.Time {
	if now.IsZero() {
		now = time.Now()
	}
	if expires.IsZero() {
		return now
	}
	expires = expires.UTC()
	if !expires.After(now) {
		return now
	}

	expiresIn := expires.Sub(now)
	// Default to refreshing halfway through the observed lifetime so we keep tokens
	// fresh even when the API clock is ahead of the watcher node.
	refreshAt := now.Add(expiresIn / 2)

	// Ensure we still refresh with a minimum safety margin before expiry.
	margin := expires.Add(-minAccessRefreshMargin)
	if margin.Before(now) {
		margin = now
	}
	if refreshAt.After(margin) {
		refreshAt = margin
	}

	// Avoid hammering the API when the observed lifetime is extremely small but
	// still provide a quick retry window.
	minDelay := now.Add(minAccessRefreshDelay)
	if minDelay.After(margin) {
		minDelay = margin
	}
	if refreshAt.Before(minDelay) {
		refreshAt = minDelay
	}
	if refreshAt.Before(now) {
		return now
	}
	return refreshAt
}
