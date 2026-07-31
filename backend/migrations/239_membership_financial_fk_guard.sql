-- Membership contribution rows are financial history. Payment orders must not
-- be deleted in a way that silently changes VIP or invitation qualification.
ALTER TABLE IF EXISTS play_membership_order_contributions
    DROP CONSTRAINT IF EXISTS play_membership_order_contributions_order_id_fkey;

ALTER TABLE IF EXISTS play_membership_order_contributions
    ADD CONSTRAINT play_membership_order_contributions_order_id_fkey
    FOREIGN KEY (order_id) REFERENCES payment_orders(id) ON DELETE RESTRICT;
