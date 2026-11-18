-- Trust is internal and scoped by the row's captured credential identity.
ALTER TABLE quality_oauth_401_streaks
    ADD COLUMN IF NOT EXISTS own_revoked_401 BOOLEAN NOT NULL DEFAULT false;
