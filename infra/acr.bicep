// TUIcast – Azure Container Registry
// Deploy: az deployment group create -g <rg> -f acr.bicep

@description('Azure region for all resources')
param location string = resourceGroup().location

@description('Registry name (5-50 alphanumeric, globally unique)')
param registryName string = 'tuicast${uniqueString(resourceGroup().id)}'

// ─── Container Registry ───────────────────────────────────────────────────────
resource acr 'Microsoft.ContainerRegistry/registries@2023-07-01' = {
  name: registryName
  location: location
  sku: {
    name: 'Basic'
  }
  properties: {
    adminUserEnabled: true
  }
}

// ─── Outputs ─────────────────────────────────────────────────────────────────
output registryName string = acr.name
output loginServer string = acr.properties.loginServer
output adminUsername string = acr.listCredentials().username
@secure()
@description('ACR admin password (sensitive — not stored in deployment logs)')
output adminPassword string = acr.listCredentials().passwords[0].value
