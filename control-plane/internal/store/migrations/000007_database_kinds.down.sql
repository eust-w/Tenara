-- restore mongo-only constraint (todo89 rollback).
DELETE FROM databases WHERE type NOT IN ('mongodb');
ALTER TABLE databases DROP CONSTRAINT databases_type_check;
ALTER TABLE databases ADD CONSTRAINT databases_type_check
    CHECK (type IN ('mongodb'));
