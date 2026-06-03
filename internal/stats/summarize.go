package stats

import "strconv"

func summarizeHours(buckets map[string]string, nowHour int64) (count48h int64, hourly []int64) {
	hourly = make([]int64, windowHours)
	sinceHour := nowHour - windowHours + 1
	for field, val := range buckets {
		h, err := strconv.ParseInt(field, 10, 64)
		if err != nil {
			continue
		}
		if h < sinceHour || h > nowHour {
			continue
		}
		c, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			continue
		}
		hourly[int(h-sinceHour)] += c
		count48h += c
	}
	return count48h, hourly
}
