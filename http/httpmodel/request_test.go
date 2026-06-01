package httpmodel

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		{
			name:     "cross month",
			startDay: "2024-01-28",
			endDay:   "2024-02-05",
			want: []time.Time{
				time.Date(2024, 1, 28, 0, 0, 0, 0, time.Local),
				time.Date(2024, 2, 5, 23, 59, 59, 999999999, time.Local),
			},
			wantErr: false,
		},
		{
			name:     "cross year",
			startDay: "2023-12-28",
			endDay:   "2024-01-05",
			want: []time.Time{
				time.Date(2023, 12, 28, 0, 0, 0, 0, time.Local),
				time.Date(2024, 1, 5, 23, 59, 59, 999999999, time.Local),
			},
			wantErr: false,
		},
		{
			name:     "empty start day",
			startDay: "",
			endDay:   "2024-01-07",
			wantErr:  true,
		},
		{
			name:     "empty end day",
			startDay: "2024-01-01",
			endDay:   "",
			wantErr:  true,
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
				if tt.errMsg != "" {
					assert.EqualError(t, err, tt.errMsg)
				}
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
			name:      "without date range (fallback to this week)",
			startDay:  "",
			endDay:    "",
			wantStart: time.Date(2024, 1, 8, 0, 0, 0, 0, time.Local), // 2024-01-10的本周一
			wantEnd:   time.Date(2024, 1, 14, 0, 0, 0, 0, time.Local), // 2024-01-10的本周日
		},
		{
			name:      "with invalid date range (fallback to this week)",
			startDay:  "invalid",
			endDay:    "date",
			wantStart: time.Date(2024, 1, 8, 0, 0, 0, 0, time.Local), // 2024-01-10的本周一
			wantEnd:   time.Date(2024, 1, 14, 0, 0, 0, 0, time.Local), // 2024-01-10的本周日
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

	expected := time.Date(2026, 5, 25, 0, 0, 0, 0, time.Local) // 2026-05-28的本周一
	assert.Equal(t, expected, monday)
	assert.Equal(t, time.Weekday(1), monday.Weekday())
}

func TestDayStatRequest_getMonday_Sunday(t *testing.T) {
	// 当今天是周日时，offset > 0 会变成 -6，所以周一应该是上周一
	fixedTime := time.Date(2024, 1, 14, 10, 0, 0, 0, time.Local) // 2024-01-14 星期日
	timeNow = func() time.Time { return fixedTime }
	defer func() {
		timeNow = time.Now
	}()

	req := &DayStatRequest{}
	monday := req.getMonday()

	expected := time.Date(2024, 1, 8, 0, 0, 0, 0, time.Local) // 周一的日期
	assert.Equal(t, expected, monday)
	assert.Equal(t, time.Monday, monday.Weekday())
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

func TestDayStatRequest_StatDayRange(t *testing.T) {
	tests := []struct {
		name     string
		startDay string
		endDay   string
		want     []uint
		wantErr  bool
	}{
		{
			name:     "valid date range",
			startDay: "2024-01-01",
			endDay:   "2024-01-07",
			want:     []uint{20240101, 20240107},
			wantErr:  false,
		},
		{
			name:     "same day",
			startDay: "2024-03-15",
			endDay:   "2024-03-15",
			want:     []uint{20240315, 20240315},
			wantErr:  false,
		},
		{
			name:     "cross month",
			startDay: "2024-01-28",
			endDay:   "2024-02-05",
			want:     []uint{20240128, 20240205},
			wantErr:  false,
		},
		{
			name:     "cross year",
			startDay: "2023-12-28",
			endDay:   "2024-01-05",
			want:     []uint{20231228, 20240105},
			wantErr:  false,
		},
		{
			name:     "invalid start day",
			startDay: "invalid",
			endDay:   "2024-01-07",
			wantErr:  true,
		},
		{
			name:     "invalid end day",
			startDay: "2024-01-01",
			endDay:   "bad",
			wantErr:  true,
		},
		{
			name:     "start after end",
			startDay: "2024-01-08",
			endDay:   "2024-01-07",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &DayStatRequest{
				StartDay: tt.startDay,
				EndDay:   tt.endDay,
			}

			got, err := req.StatDayRange()

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDayStatRequest_StatDayRangeOrLastWeek(t *testing.T) {
	// 固定时间以便测试：2024年1月10日 星期三
	fixedTime := time.Date(2024, 1, 10, 10, 30, 0, 0, time.Local)
	timeNow = func() time.Time { return fixedTime }
	defer func() {
		timeNow = time.Now
	}()

	tests := []struct {
		name     string
		startDay string
		endDay   string
		want     []uint
	}{
		{
			name:     "with valid date range",
			startDay: "2024-01-01",
			endDay:   "2024-01-07",
			want:     []uint{20240101, 20240107},
		},
		{
			name:     "without date range (fallback to this week)",
			startDay: "",
			endDay:   "",
			want:     []uint{20240108, 20240114},
		},
		{
			name:     "with invalid date range (fallback to this week)",
			startDay: "invalid",
			endDay:   "date",
			want:     []uint{20240108, 20240114},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &DayStatRequest{
				StartDay: tt.startDay,
				EndDay:   tt.endDay,
			}

			got := req.StatDayRangeOrLastWeek()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCursorSearchRequest(t *testing.T) {
	tests := []struct {
		name   string
		req    CursorSearchRequest
		after  string
		limit  int
	}{
		{
			name:  "zero values",
			req:   CursorSearchRequest{},
			after: "",
			limit: 0,
		},
		{
			name:  "with values",
			req:   CursorSearchRequest{After: "abc123", Limit: 20},
			after: "abc123",
			limit: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.after, tt.req.After)
			assert.Equal(t, tt.limit, tt.req.Limit)
		})
	}
}

func TestResponseModel(t *testing.T) {
	rm := ResponseModel{
		ID:          42,
		CreatedTime: "2024-01-01T00:00:00Z",
		UpdatedTime: "2024-01-02T00:00:00Z",
	}
	assert.Equal(t, uint(42), rm.ID)
	assert.Equal(t, "2024-01-01T00:00:00Z", rm.CreatedTime)
	assert.Equal(t, "2024-01-02T00:00:00Z", rm.UpdatedTime)
}
