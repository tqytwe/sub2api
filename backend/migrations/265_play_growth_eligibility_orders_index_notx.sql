-- Keeps the user-scoped 30-day completed recharge eligibility lookup bounded.
-- This must remain a non-transactional migration because it is created online.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_payment_orders_growth_eligibility_balance_completed
    ON payment_orders (user_id, completed_at DESC)
    WHERE order_type = 'balance'
      AND status = 'COMPLETED'
      AND completed_at IS NOT NULL;
