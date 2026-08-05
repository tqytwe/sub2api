-- 250_model_catalog_tool_capabilities.sql
--
-- A nullable JSON object keeps an explicit false distinct from an older
-- catalog row that has never been reviewed for mobile tool use. Capability
-- resolution is performed by the service layer with the precedence documented
-- there: explicit catalog value, exact upstream metadata, then false.

ALTER TABLE site_model_catalog
    ADD COLUMN IF NOT EXISTS tool_capabilities JSONB;
