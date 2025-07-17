#!/bin/bash

# GitLab-Jira Hook Environment Setup Script
# This script creates config.env from config.env.example

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
CONFIG_EXAMPLE="$PROJECT_ROOT/config.env.example"
CONFIG_FILE="$PROJECT_ROOT/config.env"

echo "🔧 Setting up GitLab-Jira Hook environment configuration..."

# Check if config.env.example exists
if [ ! -f "$CONFIG_EXAMPLE" ]; then
    echo "❌ Error: config.env.example not found at $CONFIG_EXAMPLE"
    exit 1
fi

# Check if config.env already exists
if [ -f "$CONFIG_FILE" ]; then
    echo "⚠️  Warning: config.env already exists at $CONFIG_FILE"
    echo "   This will overwrite the existing file."
    read -p "   Do you want to continue? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "❌ Setup cancelled."
        exit 1
    fi
fi

# Copy config.env.example to config.env
echo "📋 Copying config.env.example to config.env..."
cp "$CONFIG_EXAMPLE" "$CONFIG_FILE"

# Make config.env readable only by owner
chmod 600 "$CONFIG_FILE"

echo "✅ Environment configuration created successfully!"
echo ""
echo "📝 Next steps:"
echo "   1. Edit config.env with your actual configuration values:"
echo "      nano $CONFIG_FILE"
echo ""
echo "   2. Required configuration:"
echo "      - GITLAB_SECRET: Secret token for GitLab webhook validation"
echo "      - JIRA_EMAIL: Your Jira account email"
echo "      - JIRA_TOKEN: Your Jira API token"
echo "      - JIRA_BASE_URL: Your Jira Cloud instance URL"
echo ""
echo "   3. Optional configuration:"
echo "      - GITLAB_BASE_URL: Your GitLab instance URL (default: https://gitlab.com)"
echo "      - ALLOWED_PROJECTS: Comma-separated list of allowed GitLab projects"
echo "      - ALLOWED_GROUPS: Comma-separated list of allowed GitLab groups"
echo "      - PUSH_BRANCH_FILTER: Branch filter patterns"
echo ""
echo "   4. Start the service:"
echo "      docker-compose up -d"
echo ""
echo "🔒 Security note: config.env contains sensitive information."
echo "   Make sure it's not committed to version control." 