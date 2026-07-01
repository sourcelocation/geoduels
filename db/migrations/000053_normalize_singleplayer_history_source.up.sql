update match_history
set ranked = false,
    source_kind = 'solo'
where mode = 'singleplayer';

update match_sessions
set ranked = false,
    source_kind = 'solo',
    updated_at = now()
where mode = 'singleplayer';
