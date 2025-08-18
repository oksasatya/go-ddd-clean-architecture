CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS users (
                                     id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
                                     email TEXT NOT NULL UNIQUE,
                                     password TEXT NOT NULL,
                                     name TEXT NOT NULL DEFAULT '',
                                     avatar_url TEXT NOT NULL DEFAULT '',
                                     created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
                                     updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users (email);