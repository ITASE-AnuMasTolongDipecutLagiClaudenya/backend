drop policy if exists "Users update own meals" on public.meals;
drop policy if exists "Users delete own meals" on public.meals;
drop policy if exists "Users update own meal session name" on public.meals;
drop policy if exists "Users delete own meals rows" on public.meals;

create policy "Users update own meals" on public.meals
  for update to authenticated
  using (auth.uid() = user_id)
  with check (auth.uid() = user_id);

create policy "Users delete own meals" on public.meals
  for delete to authenticated
  using (auth.uid() = user_id);

grant update, delete on public.meals to authenticated;

notify pgrst, 'reload schema';
