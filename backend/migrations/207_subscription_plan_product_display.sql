-- Product display metadata for subscription storefront cards and detail dialogs.
ALTER TABLE subscription_plans
    ADD COLUMN IF NOT EXISTS cover_image_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS detail_description TEXT NOT NULL DEFAULT '';
