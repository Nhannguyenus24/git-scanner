// Display metadata for the provider slugs the API reports. The API is the
// source of truth for *which* providers are supported (it derives them from the
// live ruleset); this table only supplies a human label and a grouping category
// for presentation. A slug with no entry falls back to a prettified name under
// "Other", so a newly added rule still shows up without a frontend change — it
// just reads as its slug until it is given a nicer label here.

export type ProviderMeta = { label: string; category: string }

// Category order drives how the sections are laid out on the page.
export const CATEGORY_ORDER = [
  'AI & ML',
  'Cloud & Hosting',
  'Databases & Data',
  'Payments & Commerce',
  'Messaging & Email',
  'DevOps & Observability',
  'Developer Tools',
  'Other',
] as const

const META: Record<string, ProviderMeta> = {
  // AI & ML
  openai:         { label: 'OpenAI',          category: 'AI & ML' },
  anthropic:      { label: 'Anthropic',       category: 'AI & ML' },
  'azure-openai': { label: 'Azure OpenAI',    category: 'AI & ML' },
  openrouter:     { label: 'OpenRouter',      category: 'AI & ML' },
  groq:           { label: 'Groq',            category: 'AI & ML' },
  huggingface:    { label: 'Hugging Face',    category: 'AI & ML' },
  replicate:      { label: 'Replicate',       category: 'AI & ML' },
  perplexity:     { label: 'Perplexity',      category: 'AI & ML' },
  xai:            { label: 'xAI',             category: 'AI & ML' },
  langsmith:      { label: 'LangSmith',       category: 'AI & ML' },
  pinecone:       { label: 'Pinecone',        category: 'AI & ML' },
  deepseek:       { label: 'DeepSeek',        category: 'AI & ML' },
  cohere:         { label: 'Cohere',          category: 'AI & ML' },
  mistral:        { label: 'Mistral AI',      category: 'AI & ML' },

  // Cloud & Hosting
  aws:            { label: 'Amazon Web Services', category: 'Cloud & Hosting' },
  google:         { label: 'Google Cloud',    category: 'Cloud & Hosting' },
  digitalocean:   { label: 'DigitalOcean',    category: 'Cloud & Hosting' },
  flyio:          { label: 'Fly.io',          category: 'Cloud & Hosting' },
  tailscale:      { label: 'Tailscale',       category: 'Cloud & Hosting' },
  cloudflare:     { label: 'Cloudflare',      category: 'Cloud & Hosting' },
  alibaba:        { label: 'Alibaba Cloud',   category: 'Cloud & Hosting' },
  tencent:        { label: 'Tencent Cloud',   category: 'Cloud & Hosting' },
  azure:          { label: 'Microsoft Azure', category: 'Cloud & Hosting' },
  heroku:         { label: 'Heroku',          category: 'Cloud & Hosting' },

  // Databases & Data
  postgres:       { label: 'PostgreSQL',      category: 'Databases & Data' },
  mysql:          { label: 'MySQL / MariaDB', category: 'Databases & Data' },
  mongodb:        { label: 'MongoDB',         category: 'Databases & Data' },
  redis:          { label: 'Redis',           category: 'Databases & Data' },
  mssql:          { label: 'SQL Server',      category: 'Databases & Data' },
  rabbitmq:       { label: 'RabbitMQ',        category: 'Databases & Data' },
  supabase:       { label: 'Supabase',        category: 'Databases & Data' },
  planetscale:    { label: 'PlanetScale',     category: 'Databases & Data' },
  databricks:     { label: 'Databricks',      category: 'Databases & Data' },

  // Payments & Commerce
  stripe:         { label: 'Stripe',          category: 'Payments & Commerce' },
  square:         { label: 'Square',          category: 'Payments & Commerce' },
  shopify:        { label: 'Shopify',         category: 'Payments & Commerce' },
  razorpay:       { label: 'Razorpay',        category: 'Payments & Commerce' },
  flutterwave:    { label: 'Flutterwave',     category: 'Payments & Commerce' },
  paypal:         { label: 'PayPal / Braintree', category: 'Payments & Commerce' },
  vnpay:          { label: 'VNPay',           category: 'Payments & Commerce' },
  momo:           { label: 'MoMo',            category: 'Payments & Commerce' },
  zalopay:        { label: 'ZaloPay',         category: 'Payments & Commerce' },

  // Messaging & Email
  slack:          { label: 'Slack',           category: 'Messaging & Email' },
  discord:        { label: 'Discord',         category: 'Messaging & Email' },
  telegram:       { label: 'Telegram',        category: 'Messaging & Email' },
  twilio:         { label: 'Twilio',          category: 'Messaging & Email' },
  resend:         { label: 'Resend',          category: 'Messaging & Email' },
  sendgrid:       { label: 'SendGrid',        category: 'Messaging & Email' },
  mailgun:        { label: 'Mailgun',         category: 'Messaging & Email' },
  mailchimp:      { label: 'Mailchimp',       category: 'Messaging & Email' },
  brevo:          { label: 'Brevo',           category: 'Messaging & Email' },
  smtp:           { label: 'SMTP',            category: 'Messaging & Email' },

  // DevOps & Observability
  github:         { label: 'GitHub',          category: 'DevOps & Observability' },
  gitlab:         { label: 'GitLab',          category: 'DevOps & Observability' },
  npm:            { label: 'npm',             category: 'DevOps & Observability' },
  sentry:         { label: 'Sentry',          category: 'DevOps & Observability' },
  newrelic:       { label: 'New Relic',       category: 'DevOps & Observability' },
  grafana:        { label: 'Grafana',         category: 'DevOps & Observability' },
  doppler:        { label: 'Doppler',         category: 'DevOps & Observability' },
  datadog:        { label: 'Datadog',         category: 'DevOps & Observability' },
  dynatrace:      { label: 'Dynatrace',       category: 'DevOps & Observability' },
  dockerhub:      { label: 'Docker Hub',      category: 'DevOps & Observability' },
  vault:          { label: 'HashiCorp Vault', category: 'DevOps & Observability' },
  terraform:      { label: 'Terraform Cloud', category: 'DevOps & Observability' },
  pulumi:         { label: 'Pulumi',          category: 'DevOps & Observability' },
  pypi:           { label: 'PyPI',            category: 'DevOps & Observability' },
  rubygems:       { label: 'RubyGems',        category: 'DevOps & Observability' },
  prefect:        { label: 'Prefect',         category: 'DevOps & Observability' },

  // Developer Tools
  linear:         { label: 'Linear',          category: 'Developer Tools' },
  notion:         { label: 'Notion',          category: 'Developer Tools' },
  figma:          { label: 'Figma',           category: 'Developer Tools' },
  giphy:          { label: 'GIPHY',           category: 'Developer Tools' },
  cloudinary:     { label: 'Cloudinary',      category: 'Developer Tools' },
  atlassian:      { label: 'Atlassian',       category: 'Developer Tools' },
  postman:        { label: 'Postman',         category: 'Developer Tools' },
  contentful:     { label: 'Contentful',      category: 'Developer Tools' },
  mapbox:         { label: 'Mapbox',          category: 'Developer Tools' },
  twitter:        { label: 'X (Twitter)',     category: 'Developer Tools' },
  age:            { label: 'age encryption',  category: 'Developer Tools' },

  // Other
  generic:        { label: 'Private keys & generic secrets', category: 'Other' },
}

// Title-case a bare slug (aws-foo → "Aws Foo") for a provider with no entry.
function prettify(slug: string): string {
  return slug.split(/[-_]/).filter(Boolean).map(w => w[0].toUpperCase() + w.slice(1)).join(' ')
}

// What a leaked credential from each category lets its holder do, so the detail
// view can say why the provider matters before it says how it is caught.
const CATEGORY_IMPACT: Record<string, string> = {
  'AI & ML':                'providers.impact.ai',
  'Cloud & Hosting':        'providers.impact.cloud',
  'Databases & Data':       'providers.impact.data',
  'Payments & Commerce':    'providers.impact.payments',
  'Messaging & Email':      'providers.impact.messaging',
  'DevOps & Observability': 'providers.impact.devops',
  'Developer Tools':        'providers.impact.tools',
  'Other':                  'providers.impact.other',
}

/** The i18n key describing what a leak in this category allows. */
export function categoryImpactKey(category: string): string {
  return CATEGORY_IMPACT[category] ?? CATEGORY_IMPACT.Other
}

export function providerMeta(slug: string): ProviderMeta {
  return META[slug] ?? { label: prettify(slug), category: 'Other' }
}
