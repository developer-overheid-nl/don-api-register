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

func TestListApisParams_ApiFilters(t *testing.T) {
	org := "https://example.org/org"
	ids := " api-1, api-2 "
	params := &ListApisParams{
		Organisation: &org,
		Query:        "search",
		Ids:          &ids,
		Status:       []string{"active"},
		OasVersion:   []string{"3.1.0"},
		Version:      []string{"1.0.0"},
		AdrScore:     []string{"unknown"},
		Auth:         []string{"oauth2"},
	}

	got := params.ApiFilters()

	if got.Organisation == nil || *got.Organisation != org {
		t.Fatalf("expected organisation %q, got %#v", org, got.Organisation)
	}
	if got.Ids == nil || *got.Ids != "api-1, api-2" {
		t.Fatalf("expected trimmed ids, got %#v", got.Ids)
	}
	if got.Query != "search" || got.Status[0] != "active" || got.Auth[0] != "oauth2" {
		t.Fatalf("unexpected filters: %#v", got)
	}

	params.Status[0] = "mutated"
	if got.Status[0] != "active" {
		t.Fatalf("expected defensive copy, got %q", got.Status[0])
	}

	empty := (*ListApisParams)(nil).ApiFilters()
	if empty == nil {
		t.Fatal("expected empty filters for nil params")
	}
}
