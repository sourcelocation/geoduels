-- name: AcquireLease :one
insert into control_plane_leases (name, owner_id, fencing_token, expires_at)
values ($1, $2, 1, now() + $3::interval)
on conflict (name) do update
set owner_id = excluded.owner_id,
    fencing_token = case
      when control_plane_leases.owner_id = excluded.owner_id then control_plane_leases.fencing_token
      else control_plane_leases.fencing_token + 1
    end,
    expires_at = excluded.expires_at,
    updated_at = now()
where control_plane_leases.expires_at <= now()
   or control_plane_leases.owner_id = excluded.owner_id
returning fencing_token, expires_at;

-- name: RenewLease :one
update control_plane_leases
set expires_at = now() + $4::interval,
    updated_at = now()
where name = $1
  and owner_id = $2
  and fencing_token = $3
  and expires_at > now()
returning expires_at;

-- name: ReleaseLease :exec
delete from control_plane_leases
where name = $1 and owner_id = $2 and fencing_token = $3;
