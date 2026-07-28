# Database configs
export DB_HOST=localhost
export DB_PORT="5432"
export DB_USERNAME= # Your Username 
export DB_PASSWORD= # Your Password 
export DB_NAME=kanban
export DB_SSL_MODE=require
export DB_AUTOMIGRATE=false

# HTTP configs
export HTTP_PORT=6369

#Logger configs
export LOGGER_TYPE=  # Production or Development
export LOGGER_LEVEL=debug #Info, Debug, Error, Warn, Fatal, Panic

# Redis configs
export REDIS_HOST=redis 
export REDIS_PORT=6379
export REDIS_PASSWORD= # Your Password 

# JWT Key
export JWT_SECRET_KEY = # Your Key 
export JWT_EXPIRY= # Set the duration (in Seconds)
export REFRESH_TOKEN_EXPIRY= # Set the duration (in Seconds)

# Email configs
export BREVO_API_KEY= # Your API Key
export BREVO_FROM_EMAIL= # Sender email for Brevo and fallback

# Gmail SMTP configs
export GMAIL_SMTP_HOST=smtp.gmail.com
export GMAIL_SMTP_PORT=587
export GMAIL_SMTP_USERNAME= # Your Gmail address (e.g. yourgmail@gmail.com)
export GMAIL_SMTP_PASSWORD= # Your Gmail app password (16-character). Do NOT commit secrets here.
export GMAIL_FROM_EMAIL= # Optional sender email for Gmail SMTP. Defaults to BREVO_FROM_EMAIL if blank

# Frontend configs
export FRONTEND_DASHBOARD_URL=http://localhost:3000

#Cookie
export COOKIE_SECURE= #bool
export COOKIE_DOMAIN= #backend domain
export COOKIE_PATH=/ #default path
export COOKIE_SAMESITE= #Controls when browsers send cookies with cross-site requests.