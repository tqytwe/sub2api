-- 176_play_campaign_name_i18n.sql
-- Add localized campaign titles in rules_json.name_i18n for UI locale switching.

UPDATE play_campaigns
SET rules_json = rules_json || jsonb_build_object(
    'name_i18n', jsonb_build_object(
        'zh', name,
        'en', CASE name
            WHEN '开服福利周' THEN 'Launch perks week'
            ELSE name
        END
    )
)
WHERE NOT (rules_json ? 'name_i18n');
