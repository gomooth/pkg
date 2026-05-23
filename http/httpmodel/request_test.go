package httpmodel

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDayStatRequest_StatAtRange(t *testing.T) {
	tests := []struct {
		name     string
		startDay string
		endDay   string
		want     []time.Time
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "valid date range",
			startDay: "2024-01-01",
			endDay:   "2024-01-07",
			want: []time.Time{
				time.Date(2024, 1, 1, 0, 0, 0, 0, time.Local),
				time.Date(2024, 1, 7, 23, 59, 59, 999999999, time.Local),
			},
			wantErr: false,
		},
		{
			name:     "same day",
			startDay: "2024-01-01",
			endDay:   "2024-01-01",
			want: []time.Time{
				time.Date(2024, 1, 1, 0, 0, 0, 0, time.Local),
				time.Date(2024, 1, 1, 23, 59, 59, 999999999, time.Local),
			},
			wantErr: false,
		},
		{
			name:     "invalid start day",
			startDay: "2024-13-01",
			endDay:   "2024-01-07",
			wantErr:  true,
			errMsg:   "开始日期格式错误，必须为 2006-01-02: parsing time \"2024-13-01\": month out of range",
		},
		{
			name:     "invalid end day",
			startDay: "2024-01-01",
			endDay:   "invalid-date",
			wantErr:  true,
			errMsg:   "结束日期格式错误，必须为 2006-01-02: parsing time \"invalid-date\" as \"2006-01-02\": cannot parse \"invalid-date\" as \"2006\"",
		},
		{
			name:     "start after end",
			startDay: "2024-01-08",
			endDay:   "2024-01-07",
			wantErr:  true,
			errMsg:   "开始日期不能在结束日期之后",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &DayStatRequest{
				StartDay: tt.startDay,
				EndDay:   tt.endDay,
			}

			got, err := req.StatAtRange()

			if tt.wantErr {
				assert.Error(t, err)
				assert.EqualError(t, err, tt.errMsg)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDayStatRequest_StatAtRangeOrLastWeek(t *testing.T) {
	// 固定时间以便测试
	fixedTime := time.Date(2024, 1, 10, 10, 30, 0, 0, time.Local) // 星期三
	timeNow = func() time.Time { return fixedTime }
	defer func() {
		timeNow = time.Now // 恢复原始函数
	}()

	tests := []struct {
		name      string
		startDay  string
		endDay    string
		wantStart time.Time
		wantEnd   time.Time
	}{
		{
			name:      "with valid date range",
			startDay:  "2024-01-01",
			endDay:    "2024-01-07",
			wantStart: time.Date(2024, 1, 1, 0, 0, 0, 0, time.Local),
			wantEnd:   time.Date(2024, 1, 7, 23, 59, 59, 999999999, time.Local),
		},
		{
			name:      "without date range (fallback to last week)",
			startDay:  "",
			endDay:    "",
			wantStart: time.Date(2026, 5, 25, 0, 0, 0, 0, time.Local), // 2026-05-28的上周一
			wantEnd:   time.Date(2026, 5, 31, 0, 0, 0, 0, time.Local), // 2026-05-28的周日的 00:00:00 (注意：StatAtRangeOrLastWeek不设置23:59:59)
		},
		{
			name:      "with invalid date range (fallback to last week)",
			startDay:  "invalid",
			endDay:    "date",
			wantStart: time.Date(2026, 5, 25, 0, 0, 0, 0, time.Local), // 2026-05-28的上周一
			wantEnd:   time.Date(2026, 5, 31, 0, 0, 0, 0, time.Local), // 2026-05-28的周日的 00:00:00 (注意：StatAtRangeOrLastWeek不设置23:59:59)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &DayStatRequest{
				StartDay: tt.startDay,
				EndDay:   tt.endDay,
			}

			got := req.StatAtRangeOrLastWeek()

			assert.Equal(t, tt.wantStart, got[0])
			assert.Equal(t, tt.wantEnd, got[1])
		})
	}
}

func TestDayStatRequest_getMonday(t *testing.T) {
	// 固定时间以便测试：2026年5月28日，星期四
	fixedTime := time.Date(2026, 5, 28, 10, 30, 0, 0, time.Local) // 星期四
	timeNow = func() time.Time { return fixedTime }
	defer func() {
		timeNow = time.Now // 恢复原始函数
	}()

	req := &DayStatRequest{}
	monday := req.getMonday()

	expected := time.Date(2026, 5, 25, 0, 0, 0, 0, time.Local) // 2026-05-28的上周一
	assert.Equal(t, expected, monday)
	assert.Equal(t, time.Weekday(1), monday.Weekday())
}

func TestSearchRequest_DefaultValues(t *testing.T) {
	tests := []struct {
		name          string
		request       SearchRequest
		expectedStart int
		expectedLimit int
		expectedSort  string
	}{
		{
			name:          "zero values",
			request:       SearchRequest{},
			expectedStart: 0,
			expectedLimit: 0,
			expectedSort:  "",
		},
		{
			name:          "partial values",
			request:       SearchRequest{Start: 10, Sort: "-id"},
			expectedStart: 10,
			expectedLimit: 0,
			expectedSort:  "-id",
		},
		{
			name:          "full values",
			request:       SearchRequest{Start: 10, Limit: 20, Sort: "+created_at"},
			expectedStart: 10,
			expectedLimit: 20,
			expectedSort:  "+created_at",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedStart, tt.request.Start)
			assert.Equal(t, tt.expectedLimit, tt.request.Limit)
			assert.Equal(t, tt.expectedSort, tt.request.Sort)
		})
	}
}

// 覆盖time.Now以便测试
var timeNow = time.Now
