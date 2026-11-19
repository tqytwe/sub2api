-- Start observation at migration time using committed Fork balance transactions.
CREATE TABLE balance_forecast_coverage (
    id boolean PRIMARY KEY DEFAULT true CHECK (id),
    started_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
INSERT INTO balance_forecast_coverage (id) VALUES (true);
