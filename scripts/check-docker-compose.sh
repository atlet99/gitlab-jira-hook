#!/bin/bash

# Docker Compose Configuration Check Script
# This script validates Docker Compose configuration files

set -e

echo "🔍 Checking Docker Compose configuration..."

# Check if docker-compose.yml exists
if [ ! -f "docker-compose.yml" ]; then
    echo "❌ docker-compose.yml not found"
    exit 1
fi

# Check if config.env exists
if [ ! -f "config.env" ]; then
    echo "⚠️  config.env not found, copying from example..."
    cp config.env.example config.env
    echo "✅ config.env created from example"
fi

# Validate docker-compose.yml syntax
echo "🔍 Validating docker-compose.yml syntax..."
docker-compose config > /dev/null
if [ $? -eq 0 ]; then
    echo "✅ docker-compose.yml syntax is valid"
else
    echo "❌ docker-compose.yml syntax is invalid"
    exit 1
fi

# Validate docker-compose.prod.yml syntax
if [ -f "docker-compose.prod.yml" ]; then
    echo "🔍 Validating docker-compose.prod.yml syntax..."
    docker-compose -f docker-compose.yml -f docker-compose.prod.yml config > /dev/null
    if [ $? -eq 0 ]; then
        echo "✅ docker-compose.prod.yml syntax is valid"
    else
        echo "❌ docker-compose.prod.yml syntax is invalid"
        exit 1
    fi
fi

# Validate docker-compose.override.yml syntax
if [ -f "docker-compose.override.yml" ]; then
    echo "🔍 Validating docker-compose.override.yml syntax..."
    docker-compose -f docker-compose.yml -f docker-compose.override.yml config > /dev/null
    if [ $? -eq 0 ]; then
        echo "✅ docker-compose.override.yml syntax is valid"
    else
        echo "❌ docker-compose.override.yml syntax is invalid"
        exit 1
    fi
fi

echo "✅ All Docker Compose configurations are valid!"
echo ""
echo "📋 Usage:"
echo "  Development: docker-compose up"
echo "  Production:  docker-compose -f docker-compose.yml -f docker-compose.prod.yml up"
echo "  Build only:  docker-compose build"
echo "  Clean:       docker-compose down --volumes --remove-orphans" 