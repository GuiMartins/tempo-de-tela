CREATE TABLE IF NOT EXISTS settings (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

-- Um aparelho é identificado pelo MAC ("mac:aa:bb:..."); sem MAC conhecido, pelo IP ("ip:...").
CREATE TABLE IF NOT EXISTS devices (
  id               INTEGER PRIMARY KEY,
  key              TEXT NOT NULL UNIQUE,
  mac              TEXT NOT NULL DEFAULT '',
  name             TEXT NOT NULL DEFAULT '',
  kind             TEXT NOT NULL DEFAULT '',
  hostname         TEXT NOT NULL DEFAULT '',
  mdns_name        TEXT NOT NULL DEFAULT '',
  vendor           TEXT NOT NULL DEFAULT '',
  dhcp_vendor      TEXT NOT NULL DEFAULT '',
  dhcp_fingerprint TEXT NOT NULL DEFAULT '',
  os_guess         TEXT NOT NULL DEFAULT '',
  kind_guess       TEXT NOT NULL DEFAULT '',
  random_mac       INTEGER NOT NULL DEFAULT 0,
  ipv4             TEXT NOT NULL DEFAULT '',
  ipv6             TEXT NOT NULL DEFAULT '',
  first_seen       INTEGER NOT NULL,
  last_seen        INTEGER NOT NULL,
  last_dns         INTEGER NOT NULL DEFAULT 0,
  hidden           INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS persons (
  id         INTEGER PRIMARY KEY,
  name       TEXT NOT NULL,
  emoji      TEXT NOT NULL DEFAULT '',
  kind       TEXT NOT NULL DEFAULT 'child',
  monitoring INTEGER NOT NULL DEFAULT 1,
  created_at INTEGER NOT NULL
);

-- Vínculo aparelho → pessoa com período (until NULL = vínculo atual).
CREATE TABLE IF NOT EXISTS assignments (
  id        INTEGER PRIMARY KEY,
  device_id INTEGER NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
  person_id INTEGER NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
  since     INTEGER NOT NULL,
  until     INTEGER
);
CREATE INDEX IF NOT EXISTS idx_assign_device ON assignments(device_id, since);
CREATE INDEX IF NOT EXISTS idx_assign_person ON assignments(person_id, since);

CREATE TABLE IF NOT EXISTS domains (
  id   INTEGER PRIMARY KEY,
  name TEXT NOT NULL UNIQUE
);

-- status: 0 respondida, 1 bloqueada, 2 erro/sem resposta
CREATE TABLE IF NOT EXISTS queries (
  ts        INTEGER NOT NULL,
  device_id INTEGER NOT NULL,
  domain_id INTEGER NOT NULL,
  qtype     INTEGER NOT NULL,
  status    INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_queries_device_ts ON queries(device_id, ts);
CREATE INDEX IF NOT EXISTS idx_queries_ts ON queries(ts);

CREATE TABLE IF NOT EXISTS tokens (
  id         INTEGER PRIMARY KEY,
  name       TEXT NOT NULL,
  hash       TEXT NOT NULL UNIQUE,
  created_at INTEGER NOT NULL,
  last_used  INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS events (
  id        INTEGER PRIMARY KEY,
  ts        INTEGER NOT NULL,
  type      TEXT NOT NULL,
  device_id INTEGER,
  person_id INTEGER,
  data      TEXT NOT NULL DEFAULT '{}'
);
CREATE INDEX IF NOT EXISTS idx_events_ts ON events(ts);

-- Intervalos confirmados pelos coletores, em nanossegundos. Um IP pode trocar de dono.
CREATE TABLE IF NOT EXISTS device_addresses (
  ip         TEXT NOT NULL,
  since      INTEGER NOT NULL,
  valid_until INTEGER NOT NULL,
  device_id  INTEGER NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
  PRIMARY KEY (ip, since)
);
CREATE INDEX IF NOT EXISTS idx_device_addresses_expiry ON device_addresses(valid_until);

-- Sessões do painel web (hash SHA-256 do cookie).
CREATE TABLE IF NOT EXISTS sessions (
  hash       TEXT PRIMARY KEY,
  created_at INTEGER NOT NULL,
  expires_at INTEGER NOT NULL
);

-- Catálogo do usuário: apps criados no app Android e classificações de domínios.
CREATE TABLE IF NOT EXISTS catalog_apps (
  id         TEXT PRIMARY KEY,
  name       TEXT NOT NULL,
  category   TEXT NOT NULL,
  created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS catalog_rules (
  domain     TEXT PRIMARY KEY,
  app_id     TEXT NOT NULL,
  created_at INTEGER NOT NULL
);

-- Regras por pessoa (limites, hora de dormir, aula), em JSON.
CREATE TABLE IF NOT EXISTS person_rules (
  person_id  INTEGER PRIMARY KEY REFERENCES persons(id) ON DELETE CASCADE,
  rules      TEXT NOT NULL,
  updated_at INTEGER NOT NULL
);

-- Apps que já apareceram para cada pessoa (alerta de "primeira vez").
CREATE TABLE IF NOT EXISTS person_apps (
  person_id  INTEGER NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
  app_id     TEXT NOT NULL,
  first_seen INTEGER NOT NULL,
  PRIMARY KEY (person_id, app_id)
);
