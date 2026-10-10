-- +goose Up
-- One authoritative definition of "registered": a user with at least one linked
-- sign-in identity, plus its negation. Queries previously inlined this EXISTS
-- check everywhere.
--
-- Both are plain boolean functions so sqlc types them as non-null booleans.

-- +goose StatementBegin
CREATE FUNCTION public.gd_is_registered(p_user_id uuid) RETURNS boolean
LANGUAGE sql STABLE PARALLEL SAFE AS
$$
    SELECT EXISTS (SELECT 1 FROM public.user_identities i WHERE i.user_id = p_user_id)
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION public.gd_is_guest(p_user_id uuid) RETURNS boolean
LANGUAGE sql STABLE PARALLEL SAFE AS
$$
    SELECT NOT EXISTS (SELECT 1 FROM public.user_identities i WHERE i.user_id = p_user_id)
$$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION public.gd_is_guest(uuid);
DROP FUNCTION public.gd_is_registered(uuid);
