-- +goose Up

-- 租户登记；schema 名与 name 相同。
create table public.tenant (
    name         text primary key check (name ~ '^[a-z][a-z0-9]{1,30}$'),
    display_name text not null,
    created_at   timestamptz not null default now()
);

-- 用户只属于一个租户；邮箱在租户内唯一（统一存小写）。
create table public.app_user (
    id            bigint generated always as identity primary key,
    tenant        text not null references public.tenant (name) on delete cascade,
    email         text not null check (email = lower(email) and email <> ''),
    display_name  text not null,
    role          text not null check (role in ('admin', 'manager', 'field', 'finance')),
    password_hash text not null,
    disabled      boolean not null default false,
    created_at    timestamptz not null default now(),
    unique (tenant, email)
);

create index app_user_email_idx on public.app_user (email);

-- 会话：id 为随机令牌的 SHA-256，令牌本身只在 Cookie 里。
create table public.session (
    id         bytea primary key check (length(id) = 32),
    user_id    bigint not null references public.app_user (id) on delete cascade,
    expires_at timestamptz not null,
    created_at timestamptz not null default now()
);

create index session_user_idx on public.session (user_id);

-- +goose Down
drop table public.session;
drop table public.app_user;
drop table public.tenant;
