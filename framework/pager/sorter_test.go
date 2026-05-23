package pager

import "testing"

func TestParseSorts(t *testing.T) {
	str := "createdAt,-id,*owner"
	t.Log(ParseSorts(str))
}

func TestSanitizePageSize(t *testing.T) {
	tests := []struct {
		name string
		size int
		want int
	}{
		{"零值返回默认", 0, DefaultPageSize},
		{"负值返回默认", -1, DefaultPageSize},
		{"1 返回1", 1, 1},
		{"正常值原样返回", 50, 50},
		{"等于默认值", DefaultPageSize, DefaultPageSize},
		{"等于最大值", MaxPageSize, MaxPageSize},
		{"超过最大值截断", MaxPageSize + 1, MaxPageSize},
		{"远超最大值截断", 99999, MaxPageSize},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizePageSize(tt.size)
			if got != tt.want {
				t.Errorf("SanitizePageSize(%d) = %d, want %d", tt.size, got, tt.want)
			}
		})
	}
}
