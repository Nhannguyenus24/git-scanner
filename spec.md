make a plan of multi git scanner service with latest lts golang version that have these function
- An api, enter repo github link, scan all git history and find secret key leak and it type of app
- an api, enter username, scan all git history and find secret key leak and it type of app
- an api that trigger scan all entire public repo in github app, then store key leak, it can on off and save current progress 

- when find secret key, call service that map with api for info, seperate it type, if it have been in use and leak resource risk, mail to owner by smtp and add a coffee link, all result must have been store 
- must have analytic service


# --- POSTGRESQL ---
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_DB=app_db
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres123
POSTGRES_URL=postgresql://postgres:postgres123@localhost:5432/app_db

# --- RABBITMQ ---
RABBITMQ_HOST=localhost
RABBITMQ_PORT=5672
RABBITMQ_DEFAULT_USER=admin
RABBITMQ_DEFAULT_PASS=admin123
RABBITMQ_VHOST=/
RABBITMQ_MANAGEMENT_URL=http://localhost:15672  # Username: admin | Password: admin123
RABBITMQ_URL=amqp://admin:admin123@localhost:5672/

# --- REDIS ---
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=redis123
REDIS_URL=redis://:redis123@localhost:6379/0

use this infra (already deploy in docker)