-- Allow a small admin team to complete the four review roles with one reviewer.
-- Each role remains unique per campaign version, while reviewer identity may
-- repeat when the deployment has fewer than four administrators.
ALTER TABLE referral_campaign_approvals
    DROP CONSTRAINT IF EXISTS referral_campaign_approvals_campaign_id_version_reviewer_id_key;
