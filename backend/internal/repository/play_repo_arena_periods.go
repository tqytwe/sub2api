package repository

import (
	"database/sql"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func applyPlayArenaPeriodOptionalFields(p *service.PlayArenaPeriod, periodType sql.NullString, settledAt sql.NullTime) {
	if periodType.Valid {
		p.PeriodType = periodType.String
	}
	if p.PeriodType == "" {
		p.PeriodType = "monthly"
	}
	if settledAt.Valid {
		p.SettledAt = &settledAt.Time
	}
}
