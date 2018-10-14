CREATE TABLE IF NOT EXISTS urls (
  url text NOT NULL,
  slug text NOT NULL,
  ip VARCHAR(40),
  created timestamp without time zone DEFAULT timezone('utc'::text, now()) NOT NULL,
  updated timestamp without time zone
 );

ALTER TABLE urls
  ADD CONSTRAINT urls_slug_key UNIQUE (slug);

CREATE INDEX idx_url ON urls USING btree (url);
CREATE INDEX idx_slug ON urls USING btree (slug);

ALTER TABLE urls
  ADD COLUMN counter INT DEFAULT 0 NOT NULL;

ALTER TABLE urls
  ADD COLUMN password text DEFAULT '';

ALTER TABLE urls
  ADD COLUMN expired timestamp without time zone;

ALTER TABLE urls
  ADD COLUMN mindful boolean NOT NULL default false;

CREATE INDEX idx_counter ON urls USING btree (counter);

ALTER TABLE urls
  ADD COLUMN status text NOT NULL DEFAULT 'active';

CREATE TABLE IF NOT EXISTS url_stats (
  slug text NOT NULL,
  country text NOT NULL DEFAULT '',
  state text NOT NULL DEFAULT '',
  counter int NOT NULL DEFAULT 0,
  properties jsonb NOT NULL DEFAULT '{}'::jsonb,
  created timestamp without time zone DEFAULT timezone('utc'::text, now()) NOT NULL,
  updated timestamp without time zone
);

ALTER TABLE url_stats
  ADD CONSTRAINT urls_stats_slug_country_state_pkey UNIQUE (slug, country, state);

CREATE INDEX idx_url_stats_slug ON url_stats USING btree (slug);
CREATE INDEX idx_url_stats_country ON url_stats USING btree (country);
CREATE INDEX idx_url_stats_state ON url_stats USING btree (state);