-- A durable fencing lease for singleton control-plane responsibilities.
--
-- Redis remains suitable for ephemeral presence and routing, but an active
-- matchmaker must have a durable, observable owner.  The monotonically
-- increasing fencing token prevents a former owner from committing a launch
-- after its lease has been taken by another process.
create table if not exists control_plane_leases (
  name text primary key,
  owner_id text not null,
  fencing_token bigint not null default 1,
  expires_at timestamptz not null,
  updated_at timestamptz not null default now(),
  constraint control_plane_leases_name_check check (name <> ''),
  constraint control_plane_leases_owner_check check (owner_id <> '')
);

create index if not exists idx_control_plane_leases_expiry
  on control_plane_leases(expires_at);
