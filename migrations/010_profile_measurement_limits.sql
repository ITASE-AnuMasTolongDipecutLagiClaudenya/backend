alter table public.profiles
  drop constraint if exists profiles_weight_kg_check,
  drop constraint if exists profiles_height_cm_check;

alter table public.profiles
  add constraint profiles_weight_kg_check check (weight_kg is null or weight_kg between 20 and 200) not valid,
  add constraint profiles_height_cm_check check (height_cm is null or height_cm between 30 and 200) not valid;
