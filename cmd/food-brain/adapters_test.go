package main

import (
	"testing"

	"github.com/androidand/spisordning/internal/httpapi"
)

// TestResolvePlanSchool asserts the school fallback for REST plan runs: an
// explicit request value wins, the configured default applies when the caller
// omits it, and empty leaves the school-dedup signal off. Without this the REST
// auto-plan hardcoded "" and never deduped against the configured SKOLMATEN_SCHOOL.
func TestResolvePlanSchool(t *testing.T) {
	cases := []struct {
		name       string
		req        httpapi.PlanRunInput
		configured string
		want       string
	}{
		{"explicit request wins", httpapi.PlanRunInput{School: "mariaskolan"}, "grinda", "mariaskolan"},
		{"configured default applies", httpapi.PlanRunInput{}, "grinda", "grinda"},
		{"empty request and empty config stays off", httpapi.PlanRunInput{}, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolvePlanSchool(tc.req, tc.configured); got != tc.want {
				t.Errorf("resolvePlanSchool(%+v, %q) = %q, want %q", tc.req, tc.configured, got, tc.want)
			}
		})
	}
}
