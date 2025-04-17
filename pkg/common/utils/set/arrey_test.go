package set

import (
	"testing"
)

func TestArrayContains(t *testing.T) {
	type args struct {
		arr    []string
		target string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "empty nil array",
			args: args{
				arr:    nil,
				target: "1",
			},
			want: false,
		},
		{
			name: "string array contains",
			args: args{
				arr:    []string{"a", "b", "c"},
				target: "b",
			},
			want: true,
		},
		{
			name: "string array does not contain",
			args: args{
				arr:    []string{"a", "b", "c"},
				target: "d",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ArrayContains(tt.args.arr, tt.args.target); got != tt.want {
				t.Errorf("ArrayContains() = %v, want %v", got, tt.want)
			}
		})
	}
}
