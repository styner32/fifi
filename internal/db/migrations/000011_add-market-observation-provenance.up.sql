-- Existing rows remain raw evidence; only newly source-validated rows use v1.
ALTER TABLE market_observations ADD COLUMN provenance_version INTEGER NOT NULL DEFAULT 0;
ALTER TABLE market_observations DROP CONSTRAINT market_observations_metric_id_observed_at_key;
ALTER TABLE market_observations ADD CONSTRAINT market_observations_metric_time_provenance_key UNIQUE (metric_id, observed_at, provenance_version);
CREATE INDEX idx_market_obs_provenance_lookup ON market_observations (metric_id, provenance_version, status, observed_at DESC);
