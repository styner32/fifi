CREATE TABLE IF NOT EXISTS market_observations (
    id              BIGSERIAL PRIMARY KEY,
    metric_id       TEXT        NOT NULL,
    business_date   DATE        NOT NULL,
    observed_at     TIMESTAMPTZ NOT NULL,
    retrieved_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    value           DOUBLE PRECISION,
    unit            TEXT,
    source          TEXT,
    source_field    TEXT,
    status          TEXT        NOT NULL,
    missing_reason  TEXT,
    raw             JSONB,
    UNIQUE (metric_id, observed_at)
);

CREATE INDEX IF NOT EXISTS idx_market_obs_lookup
    ON market_observations (metric_id, status, observed_at DESC);
