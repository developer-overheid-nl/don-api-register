package models

import "testing"

func TestListApisParams_FilterIDs(t *testing.T) {
	ptr := func(v string) *string { return &v }

	tests := []struct {
		name   string
		input  ListApisParams
		expect *string
	}{
		{
			name: "falls back to ids",
			input: ListApisParams{
				Ids: ptr(" 789 , 012 "),
			},
			expect: ptr("789 , 012"),
		},
		{
			name:   "returns nil when empty",
			input:  ListApisParams{},
			expect: nil,
		},
	}

	for _, tc := range tests {
		current := tc
		t.Run(current.name, func(t *testing.T) {
			got := current.input.FilterIDs()
			switch {
			case current.expect == nil && got != nil:
				t.Fatalf("expected nil, got %q", *got)
			case current.expect != nil && got == nil:
				t.Fatalf("expected %q, got nil", *current.expect)
			case current.expect != nil && got != nil && *current.expect != *got:
				t.Fatalf("expected %q, got %q", *current.expect, *got)
			}
		})
	}
}
