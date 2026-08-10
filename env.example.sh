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

# Email configs (Primary: Brevo, Fallback: Resend)
export BREVO_API_KEY= # Your Brevo API Key
export BREVO_FROM_EMAIL= # Primary sender email

export RESEND_API_KEY= # Optional fallback Resend API Key
export RESEND_FROM_EMAIL= # Optional fallback sender email (defaults to BREVO_FROM_EMAIL)

# Frontend configs
export FRONTEND_DASHBOARD_URL=http://localhost:3000

#Cookie
export COOKIE_SECURE= #bool
export COOKIE_DOMAIN= #backend domain
export COOKIE_PATH=/ #default path
export COOKIE_SAMESITE= #Controls when browsers send cookies with cross-site requests.

# Supabase S3 Storage (for organization logo uploads)
export S3_ENDPOINT=https://rywvrcvpgeenhvlyrtaj.storage.supabase.co/storage/v1/s3
export S3_PUBLIC_ENDPOINT= # Optional public endpoint (e.g. https://rywvrcvpgeenhvlyrtaj.supabase.co/storage/v1/object/public)
export S3_REGION=ap-south-1
export S3_ACCESS_KEY_ID=      # Supabase S3 access key ID
export S3_SECRET_ACCESS_KEY=  # Supabase S3 secret access key
export S3_BUCKET=work_pilot_bucket
export S3_MAX_FILE_SIZE_MB=5  # Maximum logo upload size in megabytes (default: 5)

# Task & Comment Attachment configurations
export ATTACHMENT_MAX_FILE_SIZE_MB=10 # Maximum attachment size in megabytes (default: 10)
export ATTACHMENT_MAX_FILES_COUNT=5   # Maximum number of attachments per request (default: 5)
