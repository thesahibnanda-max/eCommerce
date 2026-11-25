package utils_test

import (
	"reflect"
	"testing"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

func TestUnionOfMaps(t *testing.T) {
	tests := []struct {
		name  string
		input []map[string]any
		want  map[string]any
	}{
		{
			name:  "no maps returns empty map",
			input: nil,
			want:  map[string]any{},
		},
		{
			name:  "single nil map returns empty map",
			input: []map[string]any{nil},
			want:  map[string]any{},
		},
		{
			name:  "single map is copied",
			input: []map[string]any{{"a": 1, "b": "two"}},
			want:  map[string]any{"a": 1, "b": "two"},
		},
		{
			name:  "disjoint maps are merged",
			input: []map[string]any{{"a": 1}, {"b": 2}, {"c": 3}},
			want:  map[string]any{"a": 1, "b": 2, "c": 3},
		},
		{
			name:  "later map wins on key conflict",
			input: []map[string]any{{"a": 1, "b": 1}, {"b": 2}, {"b": 3, "c": 3}},
			want:  map[string]any{"a": 1, "b": 3, "c": 3},
		},
		{
			name:  "nil maps mixed with non-nil maps are skipped",
			input: []map[string]any{nil, {"a": 1}, nil, {"b": 2}},
			want:  map[string]any{"a": 1, "b": 2},
		},
		{
			name:  "nil values are kept and can override",
			input: []map[string]any{{"a": 1}, {"a": nil}},
			want:  map[string]any{"a": nil},
		},
		{
			name: "nested values are kept as-is",
			input: []map[string]any{
				{"nested": map[string]any{"x": 1}},
				{"list": []int{1, 2, 3}},
			},
			want: map[string]any{
				"nested": map[string]any{"x": 1},
				"list":   []int{1, 2, 3},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := utils.UnionOfMaps(tt.input...)
			if got == nil {
				t.Fatal("UnionOfMaps returned nil map, want non-nil")
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UnionOfMaps() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnionOfMaps_NonStringKeyAndValueTypes(t *testing.T) {
	got := utils.UnionOfMaps(
		map[int]string{1: "one", 2: "two"},
		map[int]string{2: "TWO", 3: "three"},
	)
	want := map[int]string{1: "one", 2: "TWO", 3: "three"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("UnionOfMaps() = %v, want %v", got, want)
	}

	empty := utils.UnionOfMaps[int, string]()
	if empty == nil || len(empty) != 0 {
		t.Errorf("UnionOfMaps[int, string]() = %v, want empty non-nil map", empty)
	}
}

func TestUnionOfMaps_DoesNotMutateInputs(t *testing.T) {
	first := map[string]any{"a": 1}
	second := map[string]any{"a": 2, "b": 2}

	got := utils.UnionOfMaps(first, second)
	got["c"] = 3

	if !reflect.DeepEqual(first, map[string]any{"a": 1}) {
		t.Errorf("first input mutated: %v", first)
	}
	if !reflect.DeepEqual(second, map[string]any{"a": 2, "b": 2}) {
		t.Errorf("second input mutated: %v", second)
	}
}

func TestUnionOfMaps_ResultIsIndependentOfSingleInput(t *testing.T) {
	in := map[string]any{"a": 1}

	got := utils.UnionOfMaps(in)
	got["a"] = 99

	if in["a"] != 1 {
		t.Errorf("writing to result changed input: in[\"a\"] = %v, want 1", in["a"])
	}
}
