package main

import (
	"reflect"
	"slices"
	"testing"
)

func TestFocusedGenerationEnvironment(t *testing.T) {
	want := []string{
		"JAVA_HOME=/opt/java/openjdk",
		"LANG=C.UTF-8",
		"LC_ALL=C.UTF-8",
		"PATH=/opt/java/openjdk/bin:/usr/bin:/bin",
		"TZ=UTC",
	}
	if got := focusedGenerationEnvironment(); !reflect.DeepEqual(got, want) {
		t.Fatalf("focused generation environment = %v, want %v", got, want)
	}
	for _, forbidden := range []string{
		"JAVA_TOOL_OPTIONS",
		"OPENAPI_GENERATOR_VERSION",
		"OPENAPI_GENERATOR_SKIP_VALIDATE_SPEC",
	} {
		if slices.ContainsFunc(focusedGenerationEnvironment(), func(item string) bool {
			return len(item) > len(forbidden) && item[:len(forbidden)+1] == forbidden+"="
		}) {
			t.Fatalf("focused generation environment contains %s", forbidden)
		}
	}
}
