alter table public.meals add column if not exists scan_session_name text;

notify pgrst, 'reload schema';
