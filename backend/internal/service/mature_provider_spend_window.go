package service

import "time"

// ProfitWalletTodaySkew is how far the profit window's end may sit behind
// "now" and still count as today's consumption snapshot. The today preset
// sends the browser's current time, which is slightly earlier than the server.
const ProfitWalletTodaySkew = 5 * time.Minute

// ProfitConsumptionLocation is the calendar day used for 渠道运维「今日消费」.
// New API asks the upstream for Asia/Shanghai midnight through the probe time.
// Sub2API reads the upstream dashboard field today_actual_cost (else today_cost),
// which that site sums from its own process timezone's start of today. The
// snapshot does not store that timezone; the upstream default is Asia/Shanghai.
// The profit report therefore treats both as the current Asia/Shanghai day.
func ProfitConsumptionLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("Asia/Shanghai", 8*3600)
	}
	return loc
}

// ProfitConsumptionDay is [start, end) of the consumption day containing now.
func ProfitConsumptionDay(now time.Time) (start, end time.Time) {
	loc := ProfitConsumptionLocation()
	local := now.In(loc)
	start = time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	return start, start.AddDate(0, 0, 1)
}

// ProfitWindowUsesWalletToday is true when [from, until) is that same consumption
// day: it starts at midnight, and it either ends at the next midnight or is
// "today so far" (ending no earlier than now minus ProfitWalletTodaySkew, and
// not after the next midnight). A morning slice, yesterday, or a multi-day
// window is not that day.
func ProfitWindowUsesWalletToday(from, until, now time.Time) bool {
	start, end := ProfitConsumptionDay(now)
	if !from.Equal(start) || !until.After(start) || until.After(end) {
		return false
	}
	if until.Equal(end) {
		return true
	}
	return !until.Before(now.Add(-ProfitWalletTodaySkew))
}

// ProfitSnapshotDates maps a half-open window onto the inclusive Asia/Shanghai
// calendar dates accepted by Sub2API's dated usage API. end_date is the
// calendar date of the last instant inside the window. The upstream API then
// treats that date as a whole local day, so a partial day is not narrower
// than the calendar day that contains it.
func ProfitSnapshotDates(from, until time.Time) (startDate, endDate string, ok bool) {
	if !until.After(from) {
		return "", "", false
	}
	loc := ProfitConsumptionLocation()
	start := from.In(loc)
	last := until.Add(-time.Nanosecond).In(loc)
	if last.Before(start) {
		return "", "", false
	}
	return start.Format(time.DateOnly), last.Format(time.DateOnly), true
}
