-- Keep the question and rubric with the public artwork independently of admin history.
ALTER TABLE pelican_showcase_items
    ADD COLUMN IF NOT EXISTS pelican_question JSONB;

COMMENT ON COLUMN pelican_showcase_items.pelican_question IS
    'Frozen server-issued SVG drawing question and visual review rubric for this artwork';
