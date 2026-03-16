// TUIcast – Azure Storage account
// Deploy: az deployment group create -g <rg> -f storage.bicep

@description('Azure region for all resources')
param location string = resourceGroup().location

@description('Globally unique storage account name (3-24 lowercase alphanumeric)')
param storageAccountName string = 'tuicast${uniqueString(resourceGroup().id)}'

// ─── Storage account ─────────────────────────────────────────────────────────
resource storageAccount 'Microsoft.Storage/storageAccounts@2023-01-01' = {
  name: storageAccountName
  location: location
  kind: 'StorageV2'
  sku: {
    name: 'Standard_LRS'
  }
  properties: {
    accessTier: 'Hot'
    minimumTlsVersion: 'TLS1_2'
    allowBlobPublicAccess: false
    supportsHttpsTrafficOnly: true
  }
}

// ─── Blob containers ─────────────────────────────────────────────────────────
var containers = ['gifs', 'casts', 'binaries']

resource blobContainers 'Microsoft.Storage/storageAccounts/blobServices/containers@2023-01-01' = [for c in containers: {
  name: '${storageAccount.name}/default/${c}'
  properties: {
    publicAccess: 'None'
  }
}]

// ─── Lifecycle management (auto-delete old blobs) ───────────────────────────
resource lifecycle 'Microsoft.Storage/storageAccounts/managementPolicies@2023-01-01' = {
  parent: storageAccount
  name: 'default'
  properties: {
    policy: {
      rules: [
        {
          name: 'delete-old-recordings'
          enabled: true
          type: 'Lifecycle'
          definition: {
            filters: {
              blobTypes: ['blockBlob']
              prefixMatch: []
            }
            actions: {
              baseBlob: {
                // Delete GIFs and casts after 30 days.
                delete: {
                  daysAfterModificationGreaterThan: 30
                }
              }
            }
          }
        }
      ]
    }
  }
}

// ─── Outputs ─────────────────────────────────────────────────────────────────
output storageAccountName string = storageAccount.name
output connectionString string = 'DefaultEndpointsProtocol=https;AccountName=${storageAccount.name};AccountKey=${storageAccount.listKeys().keys[0].value};EndpointSuffix=core.windows.net'
