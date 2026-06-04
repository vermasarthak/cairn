CREATE EXTENSION IF NOT EXISTS pgcrypto;
ALTER TABLE jobs ADD COLUMN reservation_tenant_id TEXT;
ALTER TABLE jobs ADD COLUMN reservation_subject_id TEXT;
ALTER TABLE jobs ADD COLUMN reservation_policy_id TEXT;
ALTER TABLE jobs ADD COLUMN reservation_action_key TEXT;
ALTER TABLE jobs ADD COLUMN reservation_local_day DATE;
ALTER TABLE jobs ADD CONSTRAINT jobs_reservation_unique UNIQUE (reservation_tenant_id, reservation_subject_id, reservation_policy_id, reservation_action_key, reservation_local_day);
ALTER TABLE jobs ADD CONSTRAINT jobs_reservation_fk FOREIGN KEY (reservation_tenant_id, reservation_subject_id, reservation_policy_id, reservation_action_key, reservation_local_day) REFERENCES reservations (tenant_id, subject_id, policy_id, action_key, local_day);
