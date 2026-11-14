package buildpolicy

import "testing"

func TestAppUpdatesPolicy(t *testing.T) {
	for _, tt := range []struct {
		name string
		data string
		want bool
	}{
		{"enabled", `{"enabled":true}`, true},
		{"disabled", `{"enabled":false}`, false},
		{"missing", `{}`, false},
		{"null", `{"enabled":null}`, false},
		{"string", `{"enabled":"true"}`, false},
		{"malformed", `{"enabled":true`, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := appUpdatesEnabled([]byte(tt.data)); got != tt.want {
				t.Fatalf("appUpdatesEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}
