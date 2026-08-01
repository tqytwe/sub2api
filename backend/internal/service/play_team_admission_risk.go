package service

import (
	"context"
	"fmt"
)

// defaultPlayTeamAdmissionRisk is installed whenever the production play
// repository is present. It keeps team admission protected even without a
// separately configured third-party fraud provider.
type defaultPlayTeamAdmissionRisk struct {
	repo PlayTeamAdmissionRiskRepository
}

func (p defaultPlayTeamAdmissionRisk) CheckTeamAdmission(ctx context.Context, _ PlayTeamAdmissionAction, userID, _ int64) error {
	if userID <= 0 {
		return fmt.Errorf("invalid team admission user")
	}
	blocked, err := p.repo.HasBlockingTeamAdmissionRisk(ctx, userID)
	if err != nil {
		return fmt.Errorf("read team admission risk: %w", err)
	}
	if blocked {
		return fmt.Errorf("team admission blocked by account risk")
	}
	return nil
}
