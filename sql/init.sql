CREATE TABLE IF NOT EXISTS urls (
  url text NOT NULL,
  slug text NOT NULL,
  ip VARCHAR(40),
  counter INT DEFAULT 0 NOT NULL,
  created timestamp without time zone DEFAULT timezone('utc'::text, now()) NOT NULL,
  updated timestamp without time zone
 );

ALTER TABLE urls
    ADD CONSTRAINT urls_pk PRIMARY KEY (url);
ALTER TABLE urls
    ADD CONSTRAINT urls_slug_key UNIQUE (slug);

CREATE INDEX idx_url ON urls USING btree (url);
CREATE INDEX idx_slug ON urls USING btree (slug);
CREATE INDEX idx_counter ON urls USING btree (counter);