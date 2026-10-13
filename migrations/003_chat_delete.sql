drop policy if exists "Users delete own chat" on public.chat_messages;
create policy "Users delete own chat"
  on public.chat_messages
  for delete
  to authenticated
  using (auth.uid() = user_id);

grant delete on public.chat_messages to authenticated;
