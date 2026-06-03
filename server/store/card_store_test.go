package store

import "testing"

func TestSlabTierLabelFormatsLowerDynamicGrades(t *testing.T) {
	cases := map[string]string{
		"PSA_3": "PSA 3",
		"CGC_6": "CGC 6",
		"BGS_2": "BGS 2",
	}

	for tier, expected := range cases {
		if got := slabTierLabel(tier); got != expected {
			t.Fatalf("slabTierLabel(%q) = %q, want %q", tier, got, expected)
		}
	}
}
