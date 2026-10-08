package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type starframeVideoRepository struct{ db *sql.DB }

func NewStarframeVideoRepository(db *sql.DB) service.StarframeVideoRepository {
	return &starframeVideoRepository{db: db}
}
func (r *starframeVideoRepository) Claim(ctx context.Context, t *service.StarframeVideoTask) (bool, error) {
	payload, err := json.Marshal(t)
	if err != nil {
		return false, err
	}
	result, err := r.db.ExecContext(ctx, `INSERT INTO starframe_video_submissions(local_id,user_id,api_key_id,group_id,client_task_id,task_json) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`, t.LocalID, t.Owner.UserID, t.Owner.APIKeyID, t.Owner.GroupID, t.ClientTaskID, string(payload))
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n == 1, err
}
func (r *starframeVideoRepository) Complete(ctx context.Context, t *service.StarframeVideoTask) error {
	payload, err := json.Marshal(t)
	if err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx, `UPDATE starframe_video_submissions SET task_json=$1,completed=TRUE WHERE local_id=$2 AND user_id=$3 AND api_key_id=$4 AND group_id=$5 AND completed=FALSE`, string(payload), t.LocalID, t.Owner.UserID, t.Owner.APIKeyID, t.Owner.GroupID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("StarFrame submission missing or already finalized")
	}
	return nil
}
func (r *starframeVideoRepository) Get(ctx context.Context, id string, owner service.StarframeVideoOwner) (*service.StarframeVideoTask, error) {
	var payload string
	err := r.db.QueryRowContext(ctx, `SELECT task_json FROM starframe_video_submissions WHERE local_id=$1 AND user_id=$2 AND api_key_id=$3 AND group_id=$4 AND completed=TRUE`, id, owner.UserID, owner.APIKeyID, owner.GroupID).Scan(&payload)
	if err != nil {
		return nil, err
	}
	var task service.StarframeVideoTask
	err = json.Unmarshal([]byte(payload), &task)
	return &task, err
}
