-- Publish the first reviewed curated prompt batch.
-- The original import rows and source evidence remain durable audit records.

DO $$
DECLARE
    reviewer_id BIGINT;
    item RECORD;
    prompt_id_value BIGINT;
    public_notice TEXT := '本内容由极速蹬整理、翻译并完成模型适配。原始出处与授权类型已在内容审核记录中留存，内容版权归原权利人所有。';
    review_note TEXT := '首批极速蹬精选提示词完成系统复核并发布。';
BEGIN
    UPDATE users
    SET
        role = 'admin',
        status = 'disabled',
        updated_at = NOW()
    WHERE email = 'prompt-reviewer@jisudeng.local'
      AND deleted_at IS NULL
    RETURNING id INTO reviewer_id;

    IF reviewer_id IS NULL THEN
        INSERT INTO users (email, password_hash, role, status)
        VALUES ('prompt-reviewer@jisudeng.local', 'system-disabled', 'admin', 'disabled')
        RETURNING id INTO reviewer_id;
    END IF;

    FOR item IN
        SELECT
            id,
            job_id,
            source_key,
            external_id,
            normalized_payload,
            source_payload,
            evidence,
            authorization_status,
            prompt_id
        FROM prompt_import_items
        WHERE source_key = 'jisudeng-gpt-image-2-curated-seed-20260717'
          AND status IN ('pending_review', 'approved')
        ORDER BY external_id
    LOOP
        prompt_id_value := item.prompt_id;

        IF prompt_id_value IS NULL THEN
            SELECT source.prompt_id
            INTO prompt_id_value
            FROM prompt_sources source
            WHERE source.source_key = item.source_key
              AND source.external_id = item.external_id
            ORDER BY source.id
            LIMIT 1;
        END IF;

        IF prompt_id_value IS NULL THEN
            INSERT INTO prompts (
                status,
                brand_type,
                provenance_type,
                authorization_status,
                source_evidence_verified,
                title_zh,
                description_zh,
                purpose,
                style,
                subject,
                featured,
                current_version,
                public_attribution_note,
                created_by,
                updated_by
            ) VALUES (
                'pending_review',
                'curated',
                'external',
                'curated',
                TRUE,
                item.normalized_payload->>'title_zh',
                item.normalized_payload->>'description_zh',
                item.normalized_payload->>'purpose',
                item.normalized_payload->>'style',
                item.normalized_payload->>'subject',
                COALESCE(NULLIF(regexp_replace(item.external_id, '\D', '', 'g'), ''), '0')::INT <= 24,
                1,
                public_notice,
                reviewer_id,
                reviewer_id
            )
            RETURNING id INTO prompt_id_value;
        ELSE
            UPDATE prompts
            SET
                brand_type = 'curated',
                provenance_type = 'external',
                authorization_status = 'curated',
                source_evidence_verified = TRUE,
                title_zh = item.normalized_payload->>'title_zh',
                description_zh = item.normalized_payload->>'description_zh',
                purpose = item.normalized_payload->>'purpose',
                style = item.normalized_payload->>'style',
                subject = item.normalized_payload->>'subject',
                featured = COALESCE(NULLIF(regexp_replace(item.external_id, '\D', '', 'g'), ''), '0')::INT <= 24,
                public_attribution_note = COALESCE(NULLIF(public_attribution_note, ''), public_notice),
                updated_by = reviewer_id,
                updated_at = NOW()
            WHERE id = prompt_id_value;
        END IF;

        INSERT INTO prompt_versions (
            prompt_id,
            version,
            brand_type,
            provenance_type,
            authorization_status,
            source_evidence_verified,
            title_zh,
            description_zh,
            purpose,
            style,
            subject,
            featured,
            prompt_text,
            variables,
            models,
            sizes,
            reference_requirement,
            reference_instructions,
            requires_reference,
            public_attribution_note,
            change_note,
            created_by
        ) VALUES (
            prompt_id_value,
            1,
            'curated',
            'external',
            'curated',
            TRUE,
            item.normalized_payload->>'title_zh',
            item.normalized_payload->>'description_zh',
            item.normalized_payload->>'purpose',
            item.normalized_payload->>'style',
            item.normalized_payload->>'subject',
            COALESCE(NULLIF(regexp_replace(item.external_id, '\D', '', 'g'), ''), '0')::INT <= 24,
            item.normalized_payload->>'prompt_text',
            COALESCE(item.normalized_payload->'variables', '{}'::jsonb),
            ARRAY(SELECT jsonb_array_elements_text(COALESCE(item.normalized_payload->'models', '[]'::jsonb))),
            ARRAY(SELECT jsonb_array_elements_text(COALESCE(item.normalized_payload->'sizes', '[]'::jsonb))),
            COALESCE(NULLIF(item.normalized_payload->>'reference_requirement', ''), 'none'),
            COALESCE(item.normalized_payload->>'reference_instructions', ''),
            COALESCE((item.normalized_payload->>'requires_reference')::BOOLEAN, FALSE),
            public_notice,
            review_note,
            reviewer_id
        )
        ON CONFLICT (prompt_id, version) DO UPDATE SET
            brand_type = EXCLUDED.brand_type,
            provenance_type = EXCLUDED.provenance_type,
            authorization_status = EXCLUDED.authorization_status,
            source_evidence_verified = EXCLUDED.source_evidence_verified,
            title_zh = EXCLUDED.title_zh,
            description_zh = EXCLUDED.description_zh,
            purpose = EXCLUDED.purpose,
            style = EXCLUDED.style,
            subject = EXCLUDED.subject,
            featured = EXCLUDED.featured,
            prompt_text = EXCLUDED.prompt_text,
            variables = EXCLUDED.variables,
            models = EXCLUDED.models,
            sizes = EXCLUDED.sizes,
            reference_requirement = EXCLUDED.reference_requirement,
            reference_instructions = EXCLUDED.reference_instructions,
            requires_reference = EXCLUDED.requires_reference,
            public_attribution_note = EXCLUDED.public_attribution_note,
            change_note = EXCLUDED.change_note;

        INSERT INTO prompt_category_links (prompt_id, version, category_id)
        SELECT prompt_id_value, 1, category.id
        FROM (
            VALUES
                ('purpose', item.normalized_payload->>'purpose'),
                ('style', item.normalized_payload->>'style'),
                ('subject', item.normalized_payload->>'subject'),
                ('model', item.normalized_payload->'models'->>0),
                ('size', item.normalized_payload->'sizes'->>0)
        ) AS linked(dimension, slug)
        JOIN prompt_categories category
          ON category.dimension = linked.dimension
         AND category.slug = linked.slug
        ON CONFLICT DO NOTHING;

        INSERT INTO prompt_media (
            prompt_id,
            version,
            media_type,
            url,
            alt_zh,
            sort_order
        )
        SELECT
            prompt_id_value,
            1,
            COALESCE(NULLIF(media.value->>'media_type', ''), 'image'),
            media.value->>'url',
            COALESCE(media.value->>'alt_zh', item.normalized_payload->>'title_zh' || '示例效果'),
            COALESCE((media.value->>'sort_order')::INT, media.ordinality::INT - 1)
        FROM jsonb_array_elements(COALESCE(item.normalized_payload->'media', '[]'::jsonb)) WITH ORDINALITY AS media(value, ordinality)
        WHERE COALESCE(media.value->>'url', '') <> ''
          AND NOT EXISTS (
              SELECT 1
              FROM prompt_media existing
              WHERE existing.prompt_id = prompt_id_value
                AND existing.version = 1
                AND existing.url = media.value->>'url'
          );

        INSERT INTO prompt_sources (
            prompt_id,
            version,
            source_key,
            external_id,
            source_url,
            original_author,
            source_payload,
            evidence,
            authorization_status,
            evidence_verified,
            recorded_by
        )
        SELECT
            prompt_id_value,
            1,
            item.source_key,
            item.external_id,
            item.normalized_payload->>'source_url',
            item.normalized_payload->>'original_author',
            COALESCE(item.source_payload, '{}'::jsonb),
            COALESCE(item.evidence, '{}'::jsonb),
            'curated',
            TRUE,
            reviewer_id
        WHERE NOT EXISTS (
            SELECT 1
            FROM prompt_sources existing
            WHERE existing.prompt_id = prompt_id_value
              AND existing.version = 1
              AND existing.source_key = item.source_key
              AND existing.external_id = item.external_id
        );

        INSERT INTO prompt_review_records (
            prompt_id,
            version,
            decision,
            note,
            reviewer_id
        )
        SELECT prompt_id_value, 1, 'approve', review_note, reviewer_id
        WHERE NOT EXISTS (
            SELECT 1
            FROM prompt_review_records existing
            WHERE existing.prompt_id = prompt_id_value
              AND existing.version = 1
              AND existing.decision = 'approve'
              AND existing.note = review_note
        );

        UPDATE prompts
        SET
            status = 'published',
            published_version = 1,
            published_at = COALESCE(published_at, NOW()),
            published_by = COALESCE(published_by, reviewer_id),
            updated_by = reviewer_id,
            updated_at = NOW()
        WHERE id = prompt_id_value;

        UPDATE prompt_import_items
        SET
            status = 'approved',
            prompt_id = prompt_id_value,
            reviewed_by = reviewer_id,
            reviewed_at = COALESCE(reviewed_at, NOW()),
            updated_at = NOW()
        WHERE id = item.id;
    END LOOP;

    UPDATE prompt_import_jobs job
    SET
        status = CASE
            WHEN EXISTS (
                SELECT 1
                FROM prompt_import_items pending
                WHERE pending.job_id = job.id
                  AND pending.status = 'pending_review'
            ) THEN 'pending_review'
            ELSE 'completed'
        END,
        item_count = (
            SELECT COUNT(*)
            FROM prompt_import_items counted
            WHERE counted.job_id = job.id
        ),
        updated_at = NOW()
    WHERE job.source_key = 'jisudeng-gpt-image-2-curated-seed-20260717';
END
$$;
