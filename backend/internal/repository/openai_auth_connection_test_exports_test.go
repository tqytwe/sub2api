//go:build integration

package repository

import "database/sql"

func ChannelOperationsTestDatabase() *sql.DB { return integrationDB }
