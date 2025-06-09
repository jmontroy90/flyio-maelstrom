package main

import (
	"reflect"
	"testing"
)

func TestConvertSliceMap(t *testing.T) {
	tests := []struct {
		name string
		top  any
		want map[string][]string
	}{
		{"complex", map[string]interface{}{
			"a": []interface{}{"b", "c"},
			"d": []interface{}{"2", "3"},
		}, map[string][]string{
			"a": {"b", "c"},
			"d": {"2", "3"},
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertSliceMap(tt.top)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("convertSliceMap() mismatch (-got +want): -%+v +%+v", got, tt.want)
			}
		})
	}
}
