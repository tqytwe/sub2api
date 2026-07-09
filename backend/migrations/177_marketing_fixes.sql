-- Marketing data fixes: misconfigured affiliate rebate stored as 0.1 (percent typo).
UPDATE settings
SET value = '20', updated_at = NOW()
WHERE key = 'affiliate_rebate_rate'
  AND TRIM(value) IN ('0.1', '0.10', '0.10000000');
