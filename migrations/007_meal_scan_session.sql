alter table public.meals add column if not exists scan_session_id uuid;
alter table public.meals add column if not exists scan_mode text check (scan_mode is null or scan_mode in ('label', 'receipt'));
create index if not exists meals_scan_session_idx on public.meals(user_id, scan_session_id) where scan_session_id is not null;

notify pgrst, 'reload schema';
