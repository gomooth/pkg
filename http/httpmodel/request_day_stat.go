package httpmodel

import (
	"strconv"
	"time"

	"github.com/save95/xerror"
)

// DayStatRequest 通用日统计请求。都为空时，默认获取当周数据
type DayStatRequest struct {
	SearchRequest

	StartDay string `form:"startDay" format:"2006-01-02"`
	EndDay   string `form:"endDay" format:"2006-01-02"`
}

func (in *DayStatRequest) StatAtRange() ([]time.Time, error) {
	return in.computeStatAt()
}

func (in *DayStatRequest) StatAtRangeOrLastWeek() []time.Time {
	r, err := in.computeStatAt()
	if nil != err {
		monday := in.getMonday()
		sunday := monday.Add(6 * 24 * time.Hour)

		return []time.Time{monday, sunday}
	}

	return r
}

func (in *DayStatRequest) StatDayRange() ([]uint, error) {
	r, err := in.computeStatAt()
	if nil != err {
		return nil, err
	}

	start, _ := strconv.Atoi(r[0].Format("20060102"))
	end, _ := strconv.Atoi(r[1].Format("20060102"))

	return []uint{uint(start), uint(end)}, nil
}

func (in *DayStatRequest) StatDayRangeOrLastWeek() []uint {
	r, err := in.computeStatAt()
	if nil != err {
		monday := in.getMonday()
		start, _ := strconv.Atoi(monday.Format("20060102"))

		sunday := monday.Add(6 * 24 * time.Hour)
		end, _ := strconv.Atoi(sunday.Format("20060102"))

		return []uint{uint(start), uint(end)}
	}

	start, _ := strconv.Atoi(r[0].Format("20060102"))
	end, _ := strconv.Atoi(r[1].Format("20060102"))

	return []uint{uint(start), uint(end)}
}

func (in *DayStatRequest) computeStatAt() ([]time.Time, error) {
	startAt, err := time.ParseInLocation("2006-01-02", in.StartDay, time.Local)
	if nil != err {
		return nil, xerror.Wrap(err, "开始日期格式错误，必须为 2006-01-02")
	}

	endAt, err := time.ParseInLocation("2006-01-02", in.EndDay, time.Local)
	if nil != err {
		return nil, xerror.Wrap(err, "结束日期格式错误，必须为 2006-01-02")
	}

	endAt = time.Date(endAt.Year(), endAt.Month(), endAt.Day(), 23, 59, 59, 1e9-1, time.Local)
	if startAt.After(endAt) {
		return nil, xerror.New("开始日期不能在结束日期之后")
	}

	return []time.Time{startAt, endAt}, nil
}

// getMonday 获取本周周一的日期
func (in *DayStatRequest) getMonday() time.Time {
	now := time.Now()

	offset := int(time.Monday - now.Weekday())
	if offset > 0 {
		offset = -6
	}

	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).
		AddDate(0, 0, offset)
}
