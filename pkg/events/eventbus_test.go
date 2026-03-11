package events

import (
	"reflect"
	"testing"
)

func TestTree(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		typ       EventType
		delimiter rune
		want      []EventType
	}{
		{
			"top-level-eventtype",
			EventType("major"),
			':',
			[]EventType{
				EventType("major"),
			},
		},
		{
			"two-leveled-event",
			EventType("major:minor"),
			':',
			[]EventType{
				EventType("major"),
				EventType("major:minor"),
			},
		},
		{
			"multi-level-event",
			EventType("major:minor:fix"),
			':',
			[]EventType{
				EventType("major"),
				EventType("major:minor"),
				EventType("major:minor:fix"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Tree(tt.typ, tt.delimiter)
			
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Tree() = %v, want %v", got, tt.want)
			}
		})
	}
}
