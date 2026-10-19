DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM market_observations GROUP BY metric_id, observed_at HAVING COUNT(*) > 1) THEN
        RAISE EXCEPTION 'Cannot remove observation provenance: corrected and legacy rows share timestamps. Preserve both and resolve rollback explicitly.';
    END IF;
END $$;
DROP INDEX IF EXISTS idx_market_obs_provenance_lookup;
ALTER TABLE market_observations DROP CONSTRAINT market_observations_metric_time_provenance_key;
ALTER TABLE market_observations ADD CONSTRAINT market_observations_metric_id_observed_at_key UNIQUE (metric_id, observed_at);
ALTER TABLE market_observations DROP COLUMN provenance_version;
