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
	execute := flag.Bool("execute", false, "apply only snapshot-complete rows; default is preview")
	after := flag.Int64("after-order-id", 0, "resume after this order id")
	limit := flag.Int("limit", 500, "maximum rows to process")
	timeout := flag.Duration("timeout", 2*time.Minute, "database operation timeout")
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
	report, err := service.NewMembershipReconciliationService(db).Reconcile(ctx, service.MembershipReconciliationOptions{Execute: *execute, AfterOrderID: *after, Limit: *limit})
	if err != nil {
		fmt.Fprintf(os.Stderr, "reconcile membership contributions: %v\n", err)
		os.Exit(1)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		fmt.Fprintf(os.Stderr, "encode report: %v\n", err)
		os.Exit(1)
	}
}
