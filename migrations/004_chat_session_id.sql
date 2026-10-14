alter table public.chat_messages
  alter column conversation_id drop default;

drop policy if exists "Users delete own chat" on public.chat_messages;
create policy "Users delete own chat"
  on public.chat_messages
  for delete
  to authenticated
  using (auth.uid() = user_id);

grant delete on public.chat_messages to authenticated;

create index if not exists chat_messages_user_conversation_idx
  on public.chat_messages(user_id, conversation_id, created_at);

notify pgrst, 'reload schema';
