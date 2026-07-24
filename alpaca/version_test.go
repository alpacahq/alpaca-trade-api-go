package alpaca

import (
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// userAgentRE matches the required User-Agent format:
// APCA-<PLATFORM>/<sdk-version> <Runtime>/<runtime-version>
var userAgentRE = regexp.MustCompile(`^APCA-GO/\S+ GoRuntime/\S+$`)

func TestVersion_Format(t *testing.T) {
	v := Version()
	assert.Regexp(t, userAgentRE, v)
	assert.NotContains(t, v, "APCA-GO/v", "sdk version should not have a leading 'v'")
	assert.NotContains(t, v, "GoRuntime/go", "runtime version should not have a leading 'go'")
}

func TestVersion_MatchesHelpers(t *testing.T) {
	assert.Equal(t, "APCA-GO/"+sdkVersion()+" GoRuntime/"+runtimeVersion(), Version())
}

func TestRuntimeVersion(t *testing.T) {
	got := runtimeVersion()
	assert.Equal(t, strings.TrimPrefix(runtime.Version(), "go"), got)
	assert.False(t, strings.HasPrefix(got, "go"))
}

func TestSdkVersion_NoLeadingV(t *testing.T) {
	assert.False(t, strings.HasPrefix(sdkVersion(), "v"))
}

func TestIsValidVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    bool
	}{
		{"empty", "", false},
		{"devel", "(devel)", false},
		{"semver", "v3.9.1", true},
		{"pseudo-version", "v0.0.0-20240101000000-abcdef123456", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isValidVersion(tt.version))
		})
	}
}
