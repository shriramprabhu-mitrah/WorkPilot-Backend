#!/usr/bin/env bash
set -e

# Export environment variables for the application
source ./env.sh

# Rebuild and start the application so the latest routes are included
docker compose up -d --build