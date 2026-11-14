// Package buildpolicy holds capabilities compiled into both the CLI and desktop.
package buildpolicy

import (
	_ "embed"
	"encoding/json"
)

//go:embed app-updates.json
var appUpdatesPolicy []byte

// AppUpdatesEnabled reports whether this build may discover or download AO releases.
// Missing or invalid policy values never enable distribution from the upstream feed.
func AppUpdatesEnabled() bool {
	return appUpdatesEnabled(appUpdatesPolicy)
}

func appUpdatesEnabled(data []byte) bool {
	var policy struct {
		Enabled bool `json:"enabled"`
	}
	return json.Unmarshal(data, &policy) == nil && policy.Enabled
}
