-- Manual AppSpec override storage (plan tenara-agent-paas#17, RB-10 R8).
ALTER TABLE applications ADD COLUMN IF NOT EXISTS current_spec jsonb;
