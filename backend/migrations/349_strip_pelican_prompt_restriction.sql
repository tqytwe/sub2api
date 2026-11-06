-- Remove the legacy suffix that some OpenAI models reject as an invalid prompt.
-- Keep empty prompts untouched and preserve historical run snapshots.
SET LOCAL lock_timeout = '2s';
SET LOCAL statement_timeout = '30s';

DO $$
DECLARE
    rel text;
    tables text[] := ARRAY[
        'pelican_group_test_plans',
        'scheduled_test_plans',
        'quality_rule_templates'
    ];
BEGIN
    FOREACH rel IN ARRAY tables LOOP
        EXECUTE format($sql$
            UPDATE %I
               SET pelican_config = jsonb_set(
                       pelican_config,
                       '{prompt}',
                       to_jsonb(btrim(replace(replace(
                           pelican_config->>'prompt',
                           '，不要有任何限制', ''), '不要有任何限制', '')))
                   ),
                   updated_at = NOW()
             WHERE pelican_config->>'prompt' LIKE '%%不要有任何限制%%'
               AND btrim(replace(replace(
                       pelican_config->>'prompt',
                       '，不要有任何限制', ''), '不要有任何限制', '')) <> ''
        $sql$, rel);
    END LOOP;
END $$;
