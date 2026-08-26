-- Current schema projection for db/queries/controlplane. Keep this in lockstep
-- with the owning migration; historical migrations remain the source of truth.
create table control_plane_leases (
  name text primary key,
  owner_id text not null,
  fencing_token bigint not null,
  expires_at timestamptz not null,
  updated_at timestamptz not null
);
