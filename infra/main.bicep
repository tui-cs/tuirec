// TUIcast – Main orchestration template
// Deploys Service Bus, Storage, ACR, and the API Container App in one pass.
//
// Deploy:
//   az deployment group create -g tuicast-rg -f main.bicep \
//     -p anthropicApiKey=<key> apiImage=<image>
//
// On first run use the placeholder image; setup.sh updates it after pushing
// the real image to ACR.

@description('Azure region for all resources')
param location string = resourceGroup().location

@secure()
@description('Anthropic API key (leave empty on first deploy; set via setup.sh)')
param anthropicApiKey string = ''

@description('API container image URI (updated after first push to ACR)')
param apiImage string = 'mcr.microsoft.com/azuredocs/containerapps-helloworld:latest'

// ─── Service Bus ──────────────────────────────────────────────────────────────
module serviceBus 'service-bus.bicep' = {
  name: 'serviceBus'
  params: {
    location: location
  }
}

// ─── Storage ──────────────────────────────────────────────────────────────────
module storage 'storage.bicep' = {
  name: 'storage'
  params: {
    location: location
  }
}

// ─── ACR ──────────────────────────────────────────────────────────────────────
module acr 'acr.bicep' = {
  name: 'acr'
  params: {
    location: location
  }
}

// ─── Container App (API) ──────────────────────────────────────────────────────
module api 'container-app.bicep' = {
  name: 'api'
  params: {
    location: location
    apiImage: apiImage
    acrServer: acr.outputs.loginServer
    acrUsername: acr.outputs.adminUsername
    acrPassword: acr.outputs.adminPassword
    anthropicApiKey: anthropicApiKey
    serviceBusConnectionString: serviceBus.outputs.connectionString
    queueName: serviceBus.outputs.queueName
    storageConnectionString: storage.outputs.connectionString
  }
}

// ─── Non-sensitive outputs (used by setup.sh and deploy.yml) ─────────────────
output acrName string = acr.outputs.registryName
output acrLoginServer string = acr.outputs.loginServer
output serviceBusNamespaceName string = serviceBus.outputs.namespaceName
output storageAccountName string = storage.outputs.storageAccountName
output apiAppName string = api.outputs.appName
output containerAppsEnvName string = api.outputs.environmentName
output apiUrl string = api.outputs.apiUrl
