-- Catalogue rows for the database, cache and broker providers the connection-URL
-- rules emit (postgres-url, redis-url, …). Without these the enricher logs
-- "provider not in catalog" for a leaked database URL, and disclosure mail omits
-- the provider name and category. See 0004_seed_providers for the rationale.
--
-- Category 'datastore' / 'broker' is not in the cloud/payment set, so these do
-- not get the blast-radius severity bump; the rule already rates them 'high',
-- and the liveness probe is what sharpens a live one.
INSERT INTO providers (id, slug, name, category) VALUES
    (gen_random_uuid(), 'postgres', 'PostgreSQL',          'datastore'),
    (gen_random_uuid(), 'mysql',    'MySQL / MariaDB',     'datastore'),
    (gen_random_uuid(), 'mongodb',  'MongoDB',             'datastore'),
    (gen_random_uuid(), 'redis',    'Redis',               'datastore'),
    (gen_random_uuid(), 'mssql',    'Microsoft SQL Server', 'datastore'),
    (gen_random_uuid(), 'rabbitmq', 'RabbitMQ',            'broker')
ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name, category = EXCLUDED.category, updated_at = now();
