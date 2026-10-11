SET LOCAL lock_timeout = '50ms';
SET LOCAL statement_timeout = '500ms';

-- Future diagnostic snapshots only. Never queue behind live attempt admission,
-- infer historical sources, clear conflict flags, or rewrite old identifiers.
LOCK TABLE gateway_request_attempt_evidence IN ACCESS EXCLUSIVE MODE NOWAIT;
ALTER TABLE gateway_request_attempt_evidence
    ADD COLUMN IF NOT EXISTS identifier_diagnostics JSONB NOT NULL DEFAULT '{}'::jsonb;

-- NOT VALID avoids scanning historical rows while holding the metadata lock;
-- the constraint still bounds every new/updated snapshot, including old writers.
DO $$ BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'gateway_request_attempt_evidence'::regclass
          AND conname = 'gateway_identifier_diagnostics_bounds'
    ) THEN
        ALTER TABLE gateway_request_attempt_evidence
            ADD CONSTRAINT gateway_identifier_diagnostics_bounds CHECK (
                jsonb_typeof(identifier_diagnostics) = 'object'
                AND octet_length(identifier_diagnostics::text) <= 4096
            ) NOT VALID;
    END IF;
END $$;
