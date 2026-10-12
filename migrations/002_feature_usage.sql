create table if not exists public.feature_usage (
  user_id uuid not null references auth.users(id) on delete cascade,
  feature text not null check (feature in ('scan_label', 'scan_receipt', 'chatbot')),
  request_count bigint not null default 0 check (request_count >= 0),
  updated_at timestamptz not null default now(),
  primary key (user_id, feature)
);

alter table public.feature_usage enable row level security;
drop policy if exists "Users read own feature usage" on public.feature_usage;
create policy "Users read own feature usage" on public.feature_usage
  for select to authenticated using (auth.uid() = user_id);
grant select on public.feature_usage to authenticated;

create or replace function public.record_feature_usage(feature_name text)
returns void
language plpgsql
security definer
set search_path = ''
as $$
begin
  if auth.uid() is null then
    raise exception 'Authentication required';
  end if;
  if feature_name is null or feature_name not in ('scan_label', 'scan_receipt', 'chatbot') then
    raise exception 'Unsupported feature';
  end if;

  insert into public.feature_usage (user_id, feature, request_count, updated_at)
  values (auth.uid(), feature_name, 1, now())
  on conflict (user_id, feature)
  do update set request_count = public.feature_usage.request_count + 1,
                updated_at = now();
end;
$$;

create or replace function public.get_feature_usage()
returns table(feature text, request_count bigint)
language sql
stable
security invoker
set search_path = ''
as $$
  select features.feature, coalesce(usage.request_count, 0)
  from (values ('scan_label'), ('scan_receipt'), ('chatbot')) as features(feature)
  left join public.feature_usage as usage
    on usage.user_id = auth.uid() and usage.feature = features.feature
  order by features.feature;
$$;

revoke all on function public.record_feature_usage(text) from public, anon;
revoke all on function public.get_feature_usage() from public, anon;
grant execute on function public.record_feature_usage(text), public.get_feature_usage() to authenticated;

notify pgrst, 'reload schema';
