package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"

	_ "github.com/lib/pq"
)

func main() {
	var (
		execute = flag.Bool("execute", false, "persist recomputed ready users (default is dry-run)")
		userID  = flag.Int64("user-id", 0, "optional single user id")
		limit   = flag.Int("limit", 500, "maximum users to scan")
		timeout = flag.Duration("timeout", 2*time.Minute, "database operation timeout")
		check   = flag.Bool("check-invariants", false, "run withdrawable invariants instead of recomputing")
	)
	flag.Parse()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		os.Exit(2)
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open database: %v\n", err)
		os.Exit(2)
	}
	defer func() { _ = db.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	recomputeService := service.NewWithdrawableRecomputeService(db)
	if *check {
		report, err := recomputeService.CheckInvariants(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "check withdrawable invariants: %v\n", err)
			os.Exit(1)
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			fmt.Fprintf(os.Stderr, "encode report: %v\n", err)
			os.Exit(1)
		}
		if !report.Passed {
			os.Exit(1)
		}
		return
	}
	report, err := recomputeService.Recompute(ctx, service.WithdrawableRecomputeOptions{
		Execute: *execute,
		UserID:  *userID,
		Limit:   *limit,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "recompute withdrawable entitlements: %v\n", err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		fmt.Fprintf(os.Stderr, "encode report: %v\n", err)
		os.Exit(1)
	}
}
