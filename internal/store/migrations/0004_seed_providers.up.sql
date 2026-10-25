-- Seed the provider catalogue so the enricher can resolve a finding's provider
-- slug to a display name and a category. Without these rows the enricher logs
-- "provider not in catalog", disclosure mail omits the provider name and
-- category, and no finding gets the cloud/payment blast-radius severity bump.
--
-- One row per slug the bootstrap rules emit. ON CONFLICT keeps this safe to
-- re-run and harmless once the registry (Phase 3) owns these rows.
--
-- Category drives severity: a 'cloud' or 'payment' credential is rescored up by
-- one (plan.md §4.3), because its blast radius is infrastructure or money. This
-- takes effect on the next scan of a repository, not retroactively on findings
-- already enriched.
INSERT INTO providers (id, slug, name, category) VALUES
    (gen_random_uuid(), 'aws',          'Amazon Web Services', 'cloud'),
    (gen_random_uuid(), 'google',       'Google',              'cloud'),
    (gen_random_uuid(), 'stripe',       'Stripe',              'payment'),
    (gen_random_uuid(), 'github',       'GitHub',              'vcs'),
    (gen_random_uuid(), 'slack',        'Slack',               'messaging'),
    (gen_random_uuid(), 'cloudinary',   'Cloudinary',          'cdn'),
    (gen_random_uuid(), 'giphy',        'Giphy',               'media'),
    (gen_random_uuid(), 'resend',       'Resend',              'email'),
    (gen_random_uuid(), 'sendgrid',     'SendGrid',            'email'),
    (gen_random_uuid(), 'smtp',         'SMTP',                'email'),
    (gen_random_uuid(), 'openai',       'OpenAI',              'ai'),
    (gen_random_uuid(), 'anthropic',    'Anthropic',           'ai'),
    (gen_random_uuid(), 'openrouter',   'OpenRouter',          'ai'),
    (gen_random_uuid(), 'groq',         'Groq',                'ai'),
    (gen_random_uuid(), 'huggingface',  'Hugging Face',        'ai'),
    (gen_random_uuid(), 'perplexity',   'Perplexity',          'ai'),
    (gen_random_uuid(), 'xai',          'xAI',                 'ai'),
    (gen_random_uuid(), 'langsmith',    'LangSmith',           'ai'),
    (gen_random_uuid(), 'replicate',    'Replicate',           'ai'),
    (gen_random_uuid(), 'mistral',      'Mistral AI',          'ai'),
    (gen_random_uuid(), 'cohere',       'Cohere',              'ai'),
    (gen_random_uuid(), 'deepseek',     'DeepSeek',            'ai'),
    (gen_random_uuid(), 'azure-openai', 'Azure OpenAI',        'ai'),
    (gen_random_uuid(), 'pinecone',     'Pinecone',            'db'),
    (gen_random_uuid(), 'generic',      'Generic / unmapped',  'generic')
ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name, category = EXCLUDED.category, updated_at = now();
