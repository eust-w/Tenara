-- todo89 (D2-P2-2): widen binding kinds for the Redis/BOS product face.
ALTER TABLE databases DROP CONSTRAINT databases_type_check;
ALTER TABLE databases ADD CONSTRAINT databases_type_check
    CHECK (type IN ('mongodb','redis','storage'));
