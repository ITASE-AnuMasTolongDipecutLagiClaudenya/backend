alter table public.meals add column if not exists nutrition_sources jsonb not null default '[]'::jsonb;
