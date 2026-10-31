-- Runtime claim/finish writes change updated_at. Quality evidence needs a
-- separate configuration revision so unchanged rules retain their valid rounds.
SET LOCAL lock_timeout = '2s';
SET LOCAL statement_timeout = '30s';

-- Historical snapshots cannot establish which rule revision they tested. Start
-- their eligible evidence window at this additive migration; old writers may
-- continue omitting this column.
ALTER TABLE scheduled_test_plans
    ADD COLUMN IF NOT EXISTS quality_rule_updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE OR REPLACE FUNCTION update_quality_rule_signal_revision()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    IF ROW(NEW.account_id, NEW.model_id, NEW.enabled, NEW.pelican_config)
       IS DISTINCT FROM ROW(OLD.account_id, OLD.model_id, OLD.enabled, OLD.pelican_config) THEN
        NEW.quality_rule_updated_at := clock_timestamp();
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS quality_rule_signal_revision ON scheduled_test_plans;
CREATE TRIGGER quality_rule_signal_revision
    BEFORE UPDATE OF account_id, model_id, enabled, pelican_config ON scheduled_test_plans
    FOR EACH ROW EXECUTE FUNCTION update_quality_rule_signal_revision();
