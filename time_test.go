package lumberjack

import (
	"fmt"
	"strconv"
	"testing"
	"time"
)

func TestTimeFormat(t *testing.T) {
	n, _ := time.ParseInLocation("2006-01-02 15:04:05", fmt.Sprintf("2018-01-01 %s:00:00", "02"), time.Local)
	s := calRemainderSecondToNextRotateTime(n, RotateDaily, 1, true)
	if s != int64(86400-3600*2) {
		t.Error("calRemainderSecondToNextRotateTime failed", 1, s, 86400-3600)
	}
}

func getZoneDifferentSeconds(now time.Time) int64 {
	now, _ = time.ParseInLocation("2006-01-02 15:04:05", now.Format("2006-01-02 15:04:05"), time.Local)
	utcTime, _ := time.ParseInLocation("2006-01-02 15:04:05", now.UTC().Format("2006-01-02 15:04:05"), time.Local)
	return int64(now.Sub(utcTime).Seconds())
}

func TestCalRemainSeconds(t *testing.T) {
	t.Run("days", func(t *testing.T) {
		for i := 1; i < 30; i++ {
			n, _ := time.ParseInLocation("2006-01-02 15:04:05", "2018-01-01 00:00:00", time.Local)
			s := calRemainderSecondToNextRotateTime(n, RotateDaily, i, true)
			if s != int64(86400*i) {
				t.Error("calRemainderSecondToNextRotateTime failed", i, s, 86400*i)
			}
		}

		for i := 1; i < 24; i++ {
			hour := strconv.Itoa(i)
			if i < 10 {
				hour = "0" + hour
			}
			n, _ := time.ParseInLocation("2006-01-02 15:04:05", fmt.Sprintf("2018-01-01 %s:00:00", hour), time.Local)
			s := calRemainderSecondToNextRotateTime(n, RotateDaily, 1, true)
			if s != int64(86400-i*3600) {
				t.Error("calRemainderSecondToNextRotateTime failed", i, s, 86400-i*3600)
			}
		}
		for i := 1; i < 60; i++ {
			m := strconv.Itoa(i)
			if i < 10 {
				m = "0" + m
			}
			n, _ := time.ParseInLocation("2006-01-02 15:04:05", fmt.Sprintf("2018-01-01 01:%s:00", m), time.Local)
			s := calRemainderSecondToNextRotateTime(n, RotateDaily, 1, true)
			if s != int64(86400-3600-i*60) {
				t.Error("calRemainderSecondToNextRotateTime failed", i, s, 86400-3600-i*60)
			}
		}
		for i := 1; i < 60; i++ {
			seconds := strconv.Itoa(i)
			if i < 10 {
				seconds = "0" + seconds
			}
			n, _ := time.ParseInLocation("2006-01-02 15:04:05", fmt.Sprintf("2018-01-01 01:01:%s", seconds), time.Local)
			s := calRemainderSecondToNextRotateTime(n, RotateDaily, 1, true)
			if s != int64(86400-3600-60-i) {
				t.Error("calRemainderSecondToNextRotateTime failed", i, s, 86400-3600-60-i)
			}
		}
	})

	t.Run("hous", func(t *testing.T) {
		// i current hour
		// j 间隔小时
		for i := 1; i < 24; i++ {
			for j := 1; j < 24; j++ {
				hour := strconv.Itoa(i)
				if i < 10 {
					hour = "0" + hour
				}
				n, _ := time.ParseInLocation("2006-01-02 15:04:05", fmt.Sprintf("2018-01-01 %s:00:00", hour), time.Local)
				s := calRemainderSecondToNextRotateTime(n, RotateHourly, j, true)

				nextHour := ((i / j) + 1) * j
				if nextHour > 24 {
					nextHour = 24
				}

				if int(s) != (nextHour-i)*60*60 {
					t.Error("calRemainderSecondToNextRotateTime failed", i, j, s, (nextHour-i)*60*60)
				} else {
					//	fmt.Printf("clock:%s, interval:%d, next:%d, s:%d\n", hour, j, nextHour, s)
				}
			}
		}

	})
}
func TestCalRemainSecondsUTC(t *testing.T) {
	isLocal := false
	local := time.Local

	zoneDiff := getZoneDifferentSeconds(time.Now())
	if isLocal {
		zoneDiff = 0
	}
	t.Log(zoneDiff)

	t.Run("days", func(t *testing.T) {
		for i := 1; i < 30; i++ {
			n, _ := time.ParseInLocation("2006-01-02 15:04:05", "2018-01-01 00:00:00", local)
			s := calRemainderSecondToNextRotateTime(n, RotateDaily, i, isLocal)
			if s != int64(86400*(i-1))+zoneDiff {
				t.Error("calRemainderSecondToNextRotateTime failed", i, s, int64(86400*(i-1))+zoneDiff)
			}
		}

		for i := 1; i < 24; i++ {
			hour := strconv.Itoa(i)
			if i < 10 {
				hour = "0" + hour
			}
			n, _ := time.ParseInLocation("2006-01-02 15:04:05", fmt.Sprintf("2018-01-01 %s:00:00", hour), local)
			s := calRemainderSecondToNextRotateTime(n, RotateDaily, 1, isLocal)
			beEqual := int64(0)
			// 证明是前一天
			if int64(i)*60*60-zoneDiff < 0 {
				beEqual = zoneDiff - int64(i)*60*60
			} else {
				beEqual = 86400 - (int64(i)*60*60 - zoneDiff)
			}
			if s != beEqual {
				t.Error("calRemainderSecondToNextRotateTime failed", i, s, int64(86400*(i-1))+zoneDiff-int64(i*3600))
			}
		}
	})
	t.Run("hous", func(t *testing.T) {
		// i current hour
		// j 间隔小时
		for i := 1; i < 24; i++ {
			for j := 1; j < 24; j++ {
				hour := strconv.Itoa(i)
				if i < 10 {
					hour = "0" + hour
				}
				n, _ := time.ParseInLocation("2006-01-02 15:04:05", fmt.Sprintf("2018-01-01 %s:00:00", hour), local)
				s := calRemainderSecondToNextRotateTime(n, RotateHourly, j, false)

				diffHour := zoneDiff / 60 / 60
				h := i - int(diffHour)
				if h < 0 {
					h = 24 + h
				}
				nextHour := ((h / j) + 1) * j
				if nextHour > 24 {
					nextHour = 24
				}

				if int(s) != (nextHour-h)*60*60 {
					t.Error("calRemainderSecondToNextRotateTime failed", i, j, s, (nextHour-h)*60*60)
				} else {
					//fmt.Printf("clock:%s, interval:%d, next:%d, s:%d\n", hour, j, nextHour, s)
				}
			}
		}

	})
}
