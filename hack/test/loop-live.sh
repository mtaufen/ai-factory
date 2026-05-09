#!/usr/bin/env bash
set -euo pipefail

# Find the repository root
REPO_ROOT=$(git rev-parse --show-toplevel)

# Source the .env file if it exists
ENV_FILE="${REPO_ROOT}/.env/.env"
if [ -f "${ENV_FILE}" ]; then
  # Use set -a to export all variables, source the file, then set +a
  set -a
  source "${ENV_FILE}"
  set +a
else
  echo "Warning: .env/.env file not found. Live test may be skipped if environment variables are missing."
fi

# Run the live loop test
go test -v -count=1 -run TestLoopLiveExecution "${REPO_ROOT}/factory/cmd/factory/runtime/loop"
