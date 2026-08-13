package iter

import (
	"reflect"
	"testing"
)

func TestIterator_Collect(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		want []int
	}{
		{
			"collect-result",
			[]int{1, 2, 3},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			i := FromSlice([]int{1, 2, 3})
			got := i.Collect()
			// TODO: update the condition below to compare got with tt.want.
			if !reflect.DeepEqual(tt.want, got) {
				t.Errorf("Collect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIterator_Filter(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		filter func(int) bool
		input  []int
		want   []int
	}{
		{
			"filter-mod2",
			func(i int) bool { return i%2 == 0 },
			[]int{1, 2, 3, 4},
			[]int{2, 4},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			i := FromSlice(tt.input)
			got := i.Filter(tt.filter).Collect()
			// TODO: update the condition below to compare got with tt.want.
			if !reflect.DeepEqual(tt.want, got) {
				t.Errorf("Filter() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIterator_Take(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		n     int
		input []int
		want  []int
	}{
		{
			"take-5",
			5,
			[]int{1, 2, 3, 4, 5, 6, 7, 8, 9},
			[]int{1, 2, 3, 4, 5},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// TODO: construct the receiver type.
			i := FromSlice(tt.input)
			got := i.Take(tt.n).Collect()

			if !reflect.DeepEqual(tt.want, got) {
				t.Errorf("Take() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIterator_Skip(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		n     int
		input []int
		want  []int
	}{
		{
			"skip-5",
			5,
			[]int{1, 2, 3, 4, 5, 6, 7, 8, 9},
			[]int{6, 7, 8, 9},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// TODO: construct the receiver type.
			i := FromSlice(tt.input)
			got := i.Skip(tt.n).Collect()

			if !reflect.DeepEqual(tt.want, got) {
				t.Errorf("Skip() = %v, want %v", got, tt.want)
			}
		})
	}
}
