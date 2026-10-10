-- +goose Up
-- Collapse account types. A user is "registered" when they have at least one
-- linked sign-in identity and a guest otherwise, so the stored discriminator is
-- no longer needed. Guests keep an ordinary users row and session; only the
-- account_type column, its enum, and the constraints that referenced it change.

-- Every stored 'registered' account must already have a linked sign-in
-- identity, because the new model derives registration from identity presence.
-- Abort loudly rather than silently downgrading such accounts to guests.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM public.users u
        WHERE u.account_type = 'registered'
          AND NOT EXISTS (SELECT 1 FROM public.user_identities i WHERE i.user_id = u.id)
    ) THEN
        RAISE EXCEPTION 'users with account_type=registered and no user_identities row exist; backfill identities before applying 002010';
    END IF;
END $$;
-- +goose StatementEnd

ALTER TABLE public.users DROP CONSTRAINT users_claimed_nickname_format_check;
DROP INDEX public.users_claimed_nickname_unique;
ALTER TABLE public.users DROP COLUMN account_type;
DROP TYPE public.gd_account_type;

ALTER TABLE public.users
    ADD CONSTRAINT users_claimed_nickname_format_check CHECK (
        nickname_claimed_at IS NULL
        OR (
            display_name ~ '^[A-Za-z0-9._]{2,14}$'::text
            AND POSITION(('..'::text) IN display_name) = 0
            AND POSITION(('__'::text) IN display_name) = 0
        )
    );

CREATE UNIQUE INDEX users_claimed_nickname_unique
    ON public.users USING btree (lower(display_name))
    WHERE (nickname_claimed_at IS NOT NULL);

-- +goose Down
-- A user with a linked sign-in identity was registered; everyone else, a guest.
CREATE TYPE public.gd_account_type AS ENUM ('guest', 'registered');
ALTER TABLE public.users ADD COLUMN account_type public.gd_account_type DEFAULT 'registered'::public.gd_account_type NOT NULL;
UPDATE public.users u SET account_type = 'guest'
WHERE NOT EXISTS (SELECT 1 FROM public.user_identities i WHERE i.user_id = u.id);
ALTER TABLE public.users DROP CONSTRAINT users_claimed_nickname_format_check;
DROP INDEX public.users_claimed_nickname_unique;
ALTER TABLE public.users ADD CONSTRAINT users_claimed_nickname_format_check CHECK (((nickname_claimed_at IS NULL) OR ((account_type = 'registered'::public.gd_account_type) AND (display_name ~ '^[A-Za-z0-9._]{2,14}$'::text) AND (POSITION(('..'::text) IN (display_name)) = 0) AND (POSITION(('__'::text) IN (display_name)) = 0)))) NOT VALID;
CREATE UNIQUE INDEX users_claimed_nickname_unique ON public.users USING btree (lower(display_name)) WHERE ((account_type = 'registered'::public.gd_account_type) AND (nickname_claimed_at IS NOT NULL));
