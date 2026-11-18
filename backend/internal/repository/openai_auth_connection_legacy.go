package repository

import (
	"context"
	"database/sql"
	"errors"
)

func (r *openAIOAuthReauthRepository) GetExcelCredentials(ctx context.Context, accountID int64) (string, error) {
	var ciphertext string
	err := r.db.QueryRowContext(ctx, `SELECT credentials_encrypted FROM openai_excel_oauth_credentials WHERE account_id = $1`, accountID).Scan(&ciphertext)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return ciphertext, err
}
