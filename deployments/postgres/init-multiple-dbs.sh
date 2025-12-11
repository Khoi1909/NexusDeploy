#!/bin/bash
set -e
set -u

function create_database() {
    local database=$1
    echo "  Creating database '$database'"
    psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" <<-EOSQL
        CREATE DATABASE $database;
        GRANT ALL PRIVILEGES ON DATABASE $database TO $POSTGRES_USER;
EOSQL
}

if [ -n "$POSTGRES_MULTIPLE_DATABASES" ]; then
    echo "========================================="
    echo "Multiple database creation requested: $POSTGRES_MULTIPLE_DATABASES"
    echo "========================================="
    for db in $(echo $POSTGRES_MULTIPLE_DATABASES | tr ',' ' '); do
        create_database $db
    done
    echo "========================================="
    echo "Multiple databases created successfully!"
    echo "========================================="
fi

