-- +goose Up
-- CreatePlayerReportSignal and UpsertRiskEngineSignal use ON CONFLICT against
-- these indexes. The v2 schema dump created them as non-unique, so every write
-- failed at planning time (SQLSTATE 42P10). Restore the unique indexes from
-- 000051_moderation_signals.

DELETE FROM public.moderation_signals a
USING public.moderation_signals b
WHERE a.source = 'player_report'::public.gd_moderation_source
  AND b.source = 'player_report'::public.gd_moderation_source
  AND a.reporter_user_id IS NOT NULL
  AND b.reporter_user_id IS NOT NULL
  AND a.match_id IS NOT NULL
  AND b.match_id IS NOT NULL
  AND a.match_id = b.match_id
  AND a.reporter_user_id = b.reporter_user_id
  AND a.subject_user_id = b.subject_user_id
  AND a.id > b.id;

DELETE FROM public.moderation_signals a
USING public.moderation_signals b
WHERE a.source = 'risk_engine'::public.gd_moderation_source
  AND b.source = 'risk_engine'::public.gd_moderation_source
  AND a.subject_user_id = b.subject_user_id
  AND coalesce(a.match_id, '00000000-0000-0000-0000-000000000000'::uuid)
    = coalesce(b.match_id, '00000000-0000-0000-0000-000000000000'::uuid)
  AND coalesce(a.detector_key, '') = coalesce(b.detector_key, '')
  AND coalesce(a.detector_version, '') = coalesce(b.detector_version, '')
  AND a.reason_code = b.reason_code
  AND a.id > b.id;

DROP INDEX IF EXISTS public.idx_moderation_signals_report_dedupe;
CREATE UNIQUE INDEX idx_moderation_signals_report_dedupe
  ON public.moderation_signals USING btree (match_id, reporter_user_id, subject_user_id)
  WHERE ((source = 'player_report'::public.gd_moderation_source)
    AND (reporter_user_id IS NOT NULL)
    AND (match_id IS NOT NULL));

DROP INDEX IF EXISTS public.idx_moderation_signals_detector_dedupe;
CREATE UNIQUE INDEX idx_moderation_signals_detector_dedupe
  ON public.moderation_signals USING btree (
    subject_user_id,
    COALESCE(match_id, '00000000-0000-0000-0000-000000000000'::uuid),
    COALESCE(detector_key, ''::text),
    COALESCE(detector_version, ''::text),
    reason_code
  )
  WHERE (source = 'risk_engine'::public.gd_moderation_source);

-- +goose Down
-- The duplicate signals the Up removed don't come back.
DROP INDEX IF EXISTS public.idx_moderation_signals_report_dedupe;
CREATE INDEX idx_moderation_signals_report_dedupe ON public.moderation_signals USING btree (match_id, reporter_user_id, subject_user_id) WHERE ((source = 'player_report'::public.gd_moderation_source) AND (reporter_user_id IS NOT NULL) AND (match_id IS NOT NULL));
DROP INDEX IF EXISTS public.idx_moderation_signals_detector_dedupe;
CREATE INDEX idx_moderation_signals_detector_dedupe ON public.moderation_signals USING btree (subject_user_id, COALESCE(match_id, '00000000-0000-0000-0000-000000000000'::uuid), COALESCE(detector_key, ''::text), COALESCE(detector_version, ''::text), reason_code) WHERE (source = 'risk_engine'::public.gd_moderation_source);
