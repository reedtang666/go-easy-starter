BEGIN;
CREATE TABLE users (
 id BIGSERIAL PRIMARY KEY,
 username VARCHAR(32) NOT NULL UNIQUE CHECK (username = lower(username)),
 password_hash TEXT NOT NULL,
 nickname VARCHAR(80) NOT NULL DEFAULT '',
 avatar_id BIGINT,
 status SMALLINT NOT NULL DEFAULT 1 CHECK (status IN (0,1)),
 credential_version BIGINT NOT NULL DEFAULT 1 CHECK (credential_version > 0),
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE attachments (
 id BIGSERIAL PRIMARY KEY,
 owner_id BIGINT NOT NULL REFERENCES users(id),
 storage_name TEXT NOT NULL,
 driver VARCHAR(16) NOT NULL,
 bucket TEXT NOT NULL DEFAULT '',
 object_key TEXT NOT NULL,
 name TEXT NOT NULL,
 mime_type TEXT NOT NULL,
 size BIGINT NOT NULL CHECK (size >= 0),
 digest VARCHAR(64) NOT NULL,
 upload_id VARCHAR(64) UNIQUE,
 private BOOLEAN NOT NULL DEFAULT TRUE,
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 UNIQUE(storage_name,object_key),
 UNIQUE(id,owner_id)
);
ALTER TABLE users ADD CONSTRAINT user_avatar_owner_fk FOREIGN KEY (avatar_id,id) REFERENCES attachments(id,owner_id) ON DELETE RESTRICT;
CREATE INDEX attachments_owner_created_idx ON attachments(owner_id,created_at DESC);
COMMIT;
