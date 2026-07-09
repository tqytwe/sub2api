-- 177_play_quiz_i18n_and_pool.sql
-- Add language-aware quiz bank and expand pool to >=100 per language.

ALTER TABLE play_quiz_questions
    ADD COLUMN IF NOT EXISTS language VARCHAR(8) NOT NULL DEFAULT 'en';

UPDATE play_quiz_questions
SET language = 'en'
WHERE language IS NULL OR TRIM(language) = '';

CREATE INDEX IF NOT EXISTS idx_play_quiz_questions_lang_active_sort
    ON play_quiz_questions(language, active, sort_order, id);

WITH zh_templates AS (
    SELECT * FROM (VALUES
        (1, '以下哪项最符合 API 网关的核心职责', '统一鉴权、限流和路由', '替代数据库事务', '负责前端页面渲染', '直接训练大模型', 0),
        (2, '调用模型接口时，最常见的计费维度是', '数据库连接数', 'CPU 温度', '输入与输出 token 数', '网卡带宽峰值', 2),
        (3, '当接口返回 401 时，通常表示', '请求成功', '需要登录或鉴权失败', '资源永久删除', '服务器重启中', 1),
        (4, '幂等键（Idempotency-Key）的主要作用是', '提升图片清晰度', '防止重试造成重复扣费或重复创建', '绕过权限校验', '跳过日志记录', 1),
        (5, '在生产环境中处理超时，推荐做法是', '无限等待', '直接忽略错误', '设置超时并做有限重试', '每次都重启服务', 2),
        (6, '提示词前缀缓存（Prompt Cache）的主要收益是', '提高重复上下文请求的速度并降低成本', '关闭流式输出', '禁用鉴权', '强制返回 HTML', 0),
        (7, '要定位一次失败请求，最优先查看的是', '随机截图', '浏览器主题色', '请求 ID 与错误码', '鼠标 DPI', 2),
        (8, '以下哪项属于客户端应避免的行为', '校验参数后再发请求', '遇到 429 时退避重试', '把密钥硬编码到前端公开代码', '记录关键错误日志', 2),
        (9, '流式输出（streaming）最主要的用户价值是', '首字节更快、边生成边展示', '保证永不失败', '自动修复业务逻辑', '免除计费', 0),
        (10, '对多语言题库最稳妥的实现方式是', '仅靠前端翻译题干', '后端按语言返回对应题目', '随机返回任意语言', '固定英文不处理', 1)
    ) AS t(template_order, prompt, opt1, opt2, opt3, opt4, correct_index)
),
zh_pool AS (
    SELECT
        template_order,
        prompt,
        opt1,
        opt2,
        opt3,
        opt4,
        correct_index,
        variant,
        ROW_NUMBER() OVER (ORDER BY template_order, variant) AS rn
    FROM zh_templates
    CROSS JOIN generate_series(1, 10) AS variant
),
zh_need AS (
    SELECT GREATEST(0, 100 - COUNT(*))::int AS need
    FROM play_quiz_questions
    WHERE language = 'zh'
),
zh_base AS (
    SELECT COALESCE(MAX(sort_order), 0) AS max_sort
    FROM play_quiz_questions
    WHERE language = 'zh'
)
INSERT INTO play_quiz_questions (language, prompt, options, correct_index, sort_order, active)
SELECT
    'zh',
    zp.prompt || '（第' || zp.variant || '题）',
    jsonb_build_array(zp.opt1, zp.opt2, zp.opt3, zp.opt4),
    zp.correct_index,
    zb.max_sort + zp.rn,
    TRUE
FROM zh_pool zp
CROSS JOIN zh_need zn
CROSS JOIN zh_base zb
WHERE zp.rn <= zn.need;

WITH en_templates AS (
    SELECT * FROM (VALUES
        (1, 'Which option best describes an API gateway responsibility', 'Unified auth, rate limiting, and routing', 'Replacing all database transactions', 'Rendering frontend pages on the server', 'Training foundation models directly', 0),
        (2, 'The most common billing unit for LLM APIs is', 'Database connection count', 'CPU package temperature', 'Input and output token usage', 'Peak network bandwidth', 2),
        (3, 'An HTTP 401 response usually means', 'The request succeeded', 'Authentication is required or failed', 'The resource was permanently deleted', 'The server is restarting', 1),
        (4, 'The main purpose of an idempotency key is to', 'Improve image resolution', 'Avoid duplicate side effects during retries', 'Bypass authorization checks', 'Skip audit logging completely', 1),
        (5, 'A good production strategy for timeouts is to', 'Wait forever without limits', 'Ignore timeout errors', 'Set explicit timeouts with bounded retries', 'Restart services for every timeout', 2),
        (6, 'Prompt prefix caching is mainly used to', 'Reduce cost and latency for repeated context', 'Disable streaming responses', 'Turn off authentication', 'Force HTML outputs', 0),
        (7, 'The first thing to inspect for a failed request is', 'A random screenshot', 'Your browser color theme', 'Request ID and error code', 'Mouse DPI settings', 2),
        (8, 'Which client behavior should be avoided', 'Validate parameters before requests', 'Back off and retry on 429', 'Hard-code secrets in public frontend code', 'Log critical failures with context', 2),
        (9, 'The biggest user-facing advantage of streaming is', 'Faster time-to-first-token and progressive display', 'Guaranteed zero failures', 'Automatic business logic correction', 'No billing needed', 0),
        (10, 'A robust multilingual quiz backend should', 'Rely only on frontend translation', 'Return question sets by requested language', 'Randomly return any language', 'Always return English', 1)
    ) AS t(template_order, prompt, opt1, opt2, opt3, opt4, correct_index)
),
en_pool AS (
    SELECT
        template_order,
        prompt,
        opt1,
        opt2,
        opt3,
        opt4,
        correct_index,
        variant,
        ROW_NUMBER() OVER (ORDER BY template_order, variant) AS rn
    FROM en_templates
    CROSS JOIN generate_series(1, 10) AS variant
),
en_need AS (
    SELECT GREATEST(0, 100 - COUNT(*))::int AS need
    FROM play_quiz_questions
    WHERE language = 'en'
),
en_base AS (
    SELECT COALESCE(MAX(sort_order), 0) AS max_sort
    FROM play_quiz_questions
    WHERE language = 'en'
)
INSERT INTO play_quiz_questions (language, prompt, options, correct_index, sort_order, active)
SELECT
    'en',
    enp.prompt || ' (#' || enp.variant || ')',
    jsonb_build_array(enp.opt1, enp.opt2, enp.opt3, enp.opt4),
    enp.correct_index,
    eb.max_sort + enp.rn,
    TRUE
FROM en_pool enp
CROSS JOIN en_need en
CROSS JOIN en_base eb
WHERE enp.rn <= en.need;
