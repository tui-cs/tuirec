#!/usr/bin/env bash
# TUIcast – Initial Azure Deployment
#
# Run once to provision all Azure infrastructure and populate GitHub Actions
# secrets. Subsequent deployments are handled automatically by the
# .github/workflows/deploy.yml workflow on every push to main.
#
# Prerequisites:
#   az CLI   – https://docs.microsoft.com/en-us/cli/azure/install-azure-cli
#   gh CLI   – https://cli.github.com/
#   Docker   – https://www.docker.com/
#   jq       – https://stedolan.github.io/jq/
#
# Usage:
#   cd <repo-root>
#   ANTHROPIC_API_KEY=sk-ant-... ./infra/setup.sh
#
# Optional environment variables:
#   TUICAST_RG       Resource group name (default: tuicast-rg)
#   TUICAST_LOCATION Azure region        (default: eastus)
#   GITHUB_REPO      owner/repo          (default: auto-detected from git remote)

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# ── Configuration ─────────────────────────────────────────────────────────────
RESOURCE_GROUP="${TUICAST_RG:-tuicast-rg}"
LOCATION="${TUICAST_LOCATION:-eastus}"
ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY:-}"

# Auto-detect GitHub repo from git remote if not set
if [[ -z "${GITHUB_REPO:-}" ]]; then
  GITHUB_REPO=$(git -C "$REPO_ROOT" remote get-url origin \
    | sed -E 's|.*github\.com[:/]||; s|\.git$||')
fi

# ── Helpers ───────────────────────────────────────────────────────────────────
info() { echo -e "\033[34m▶\033[0m $*"; }
ok()   { echo -e "\033[32m✓\033[0m $*"; }
warn() { echo -e "\033[33m!\033[0m $*"; }
die()  { echo -e "\033[31m✗\033[0m $*" >&2; exit 1; }

# ── Prerequisites ─────────────────────────────────────────────────────────────
for cmd in az gh docker jq; do
  command -v "$cmd" &>/dev/null || die "'$cmd' is not installed. See script header for links."
done

[[ -z "$ANTHROPIC_API_KEY" ]] && \
  warn "ANTHROPIC_API_KEY is not set. AI-driven jobs will fail until you set it."

# ── Azure login ───────────────────────────────────────────────────────────────
info "Checking Azure login..."
if ! az account show &>/dev/null; then
  az login
fi
ok "Logged in to Azure ($(az account show --query name -o tsv))."

# ── GitHub login ──────────────────────────────────────────────────────────────
info "Checking GitHub login..."
gh auth status &>/dev/null || gh auth login
ok "Logged in to GitHub."

# ── Resource group ────────────────────────────────────────────────────────────
info "Creating resource group '$RESOURCE_GROUP' in '$LOCATION'..."
az group create --name "$RESOURCE_GROUP" --location "$LOCATION" --output none
ok "Resource group ready."

# ── Provision infrastructure ──────────────────────────────────────────────────
info "Deploying infrastructure (this takes ~5 minutes)..."
DEPLOY_OUTPUT=$(az deployment group create \
  --resource-group "$RESOURCE_GROUP" \
  --name "tuicast-main" \
  --template-file "$SCRIPT_DIR/main.bicep" \
  --parameters anthropicApiKey="$ANTHROPIC_API_KEY" \
  --query "properties.outputs" \
  --output json)

ACR_NAME=$(echo "$DEPLOY_OUTPUT"         | jq -r '.acrName.value')
ACR_SERVER=$(echo "$DEPLOY_OUTPUT"       | jq -r '.acrLoginServer.value')
SB_NS=$(echo "$DEPLOY_OUTPUT"            | jq -r '.serviceBusNamespaceName.value')
STORAGE_ACCOUNT=$(echo "$DEPLOY_OUTPUT"  | jq -r '.storageAccountName.value')
API_APP_NAME=$(echo "$DEPLOY_OUTPUT"     | jq -r '.apiAppName.value')
CONTAINER_ENV=$(echo "$DEPLOY_OUTPUT"    | jq -r '.containerAppsEnvName.value')
API_URL=$(echo "$DEPLOY_OUTPUT"          | jq -r '.apiUrl.value')
ok "Infrastructure deployed."

# ── Build and push Docker images ──────────────────────────────────────────────
info "Logging in to ACR '$ACR_SERVER'..."
az acr login --name "$ACR_NAME"

info "Building and pushing tuicast-api..."
docker build -t "$ACR_SERVER/tuicast-api:latest" -f "$REPO_ROOT/Dockerfile.api" "$REPO_ROOT"
docker push "$ACR_SERVER/tuicast-api:latest"

info "Building and pushing tuicast-worker..."
docker build -t "$ACR_SERVER/tuicast-worker:latest" -f "$REPO_ROOT/Dockerfile.worker" "$REPO_ROOT"
docker push "$ACR_SERVER/tuicast-worker:latest"
ok "Images pushed to ACR."

# ── Update Container App with the real API image ──────────────────────────────
info "Updating Container App '$API_APP_NAME' with built image..."
az containerapp update \
  --name "$API_APP_NAME" \
  --resource-group "$RESOURCE_GROUP" \
  --image "$ACR_SERVER/tuicast-api:latest" \
  --output none
ok "Container App updated."

# ── Retrieve secrets ──────────────────────────────────────────────────────────
info "Retrieving connection strings..."

ACR_PASSWORD=$(az acr credential show \
  --name "$ACR_NAME" \
  --query "passwords[0].value" -o tsv)

SB_CONN=$(az servicebus namespace authorization-rule keys list \
  --resource-group "$RESOURCE_GROUP" \
  --namespace-name "$SB_NS" \
  --name TUIcastWorker \
  --query primaryConnectionString -o tsv)

STORAGE_CONN=$(az storage account show-connection-string \
  --resource-group "$RESOURCE_GROUP" \
  --name "$STORAGE_ACCOUNT" \
  --output tsv)

# ── Create Azure service principal for GitHub Actions ─────────────────────────
info "Creating service principal for GitHub Actions CI/CD..."
SUBSCRIPTION_ID=$(az account show --query id -o tsv)
SP_JSON=$(az ad sp create-for-rbac \
  --name "tuicast-github-actions" \
  --role contributor \
  --scopes "/subscriptions/$SUBSCRIPTION_ID/resourceGroups/$RESOURCE_GROUP" \
  --sdk-auth \
  --output json)
ok "Service principal created."

# ── Store all secrets in GitHub Actions ──────────────────────────────────────
info "Storing secrets in GitHub repo '$GITHUB_REPO'..."

gh secret set AZURE_CREDENTIALS              --body "$SP_JSON"          --repo "$GITHUB_REPO"
gh secret set AZURE_RESOURCE_GROUP          --body "$RESOURCE_GROUP"    --repo "$GITHUB_REPO"
gh secret set AZURE_ACR_NAME               --body "$ACR_NAME"            --repo "$GITHUB_REPO"
gh secret set AZURE_ACR_SERVER             --body "$ACR_SERVER"          --repo "$GITHUB_REPO"
gh secret set AZURE_ACR_PASSWORD           --body "$ACR_PASSWORD"        --repo "$GITHUB_REPO"
gh secret set AZURE_API_APP_NAME           --body "$API_APP_NAME"        --repo "$GITHUB_REPO"
gh secret set AZURE_CONTAINER_ENV          --body "$CONTAINER_ENV"       --repo "$GITHUB_REPO"
gh secret set SERVICE_BUS_CONNECTION_STRING --body "$SB_CONN"            --repo "$GITHUB_REPO"
gh secret set STORAGE_CONNECTION_STRING     --body "$STORAGE_CONN"       --repo "$GITHUB_REPO"

if [[ -n "$ANTHROPIC_API_KEY" ]]; then
  gh secret set ANTHROPIC_API_KEY          --body "$ANTHROPIC_API_KEY"  --repo "$GITHUB_REPO"
  ok "ANTHROPIC_API_KEY stored."
else
  warn "ANTHROPIC_API_KEY was not stored — set it manually when ready:"
  echo "  gh secret set ANTHROPIC_API_KEY --repo $GITHUB_REPO"
fi

ok "GitHub secrets stored."

# ── Done ──────────────────────────────────────────────────────────────────────
echo ""
echo "╔══════════════════════════════════════════════════════════════════╗"
echo "║  TUIcast deployed successfully!                                  ║"
echo "║                                                                  ║"
printf "║  API URL: %-55s║\n" "$API_URL"
echo "║                                                                  ║"
echo "║  Future pushes to main will automatically update the deployment. ║"
echo "╚══════════════════════════════════════════════════════════════════╝"
