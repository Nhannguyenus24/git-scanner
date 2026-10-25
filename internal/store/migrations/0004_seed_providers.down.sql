DELETE FROM providers WHERE slug IN (
    'aws','google','stripe','github','slack','cloudinary','giphy','resend','sendgrid','smtp',
    'openai','anthropic','openrouter','groq','huggingface','perplexity','xai','langsmith',
    'replicate','mistral','cohere','deepseek','azure-openai','pinecone','generic');
