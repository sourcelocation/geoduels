update match_history
set source_kind = 'queue'
where mode = 'singleplayer'
  and source_kind = 'solo';

update match_sessions
set source_kind = 'queue',
    updated_at = now()
where mode = 'singleplayer'
  and source_kind = 'solo';
