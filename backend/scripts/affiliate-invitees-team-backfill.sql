\set ON_ERROR_STOP on

-- Manual one-inviter repair for affiliate invitees that should join the
-- inviter's active Agent Team.
--
-- Example for user #50:
-- psql "$DATABASE_URL" \
--   -v inviter_id=50 \
--   -v team_invite_code=8895EAB6 \
--   -f backend/scripts/affiliate-invitees-team-backfill.sql

BEGIN;

CREATE TEMP TABLE affiliate_team_backfill_candidates ON COMMIT DROP AS
WITH target_team AS (
	SELECT t.id AS team_id
	FROM play_teams t
	JOIN play_team_members inviter_membership
	  ON inviter_membership.team_id = t.id
	WHERE t.invite_code = :'team_invite_code'
	  AND t.archived_at IS NULL
	  AND inviter_membership.user_id = :inviter_id
	  AND inviter_membership.left_at IS NULL
	ORDER BY inviter_membership.joined_at ASC, inviter_membership.id ASC
	LIMIT 1
)
SELECT
	target_team.team_id,
	ua.user_id AS invitee_user_id
FROM target_team
JOIN user_affiliates ua
  ON ua.inviter_id = :inviter_id
WHERE ua.user_id <> :inviter_id
  AND NOT EXISTS (
	  SELECT 1
	  FROM play_team_members active_membership
	  WHERE active_membership.user_id = ua.user_id
	    AND active_membership.left_at IS NULL
  );

SELECT
	COUNT(*) AS eligible_invitees
FROM affiliate_team_backfill_candidates;

WITH inserted_members AS (
	INSERT INTO play_team_members (team_id, user_id)
	SELECT team_id, invitee_user_id
	FROM affiliate_team_backfill_candidates
	ON CONFLICT DO NOTHING
	RETURNING team_id, user_id
),
inserted_events AS (
	INSERT INTO play_team_events (
		team_id,
		actor_user_id,
		subject_user_id,
		event_type,
		detail
	)
	SELECT
		team_id,
		:inviter_id,
		user_id,
		'member_joined',
		jsonb_build_object('source', 'affiliate_invite_backfill', 'inviter_user_id', :inviter_id)
	FROM inserted_members
	RETURNING id
)
SELECT
	(SELECT COUNT(*) FROM inserted_members) AS inserted_members,
	(SELECT COUNT(*) FROM inserted_events) AS inserted_events;

COMMIT;
