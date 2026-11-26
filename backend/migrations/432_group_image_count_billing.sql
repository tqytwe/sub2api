-- 分组级图片按张计费策略。默认关闭以保持既有分组结算口径。
ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS image_count_billing BOOLEAN NOT NULL DEFAULT false;

COMMENT ON COLUMN groups.image_count_billing IS '图片端点按实际出图张数结算';
