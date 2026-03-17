// TUIcast – Azure Container Apps environment and API service
// Deploy via main.bicep; not intended to be deployed standalone.

@description('Azure region for all resources')
param location string = resourceGroup().location

@description('Container Apps managed environment name')
param environmentName string = 'tuicast-env'

@description('API Container App name')
param appName string = 'tuicast-api'

@description('API container image (full URI with tag, e.g. myacr.azurecr.io/tuicast-api:latest)')
param apiImage string

@description('ACR login server')
param acrServer string

@description('ACR admin username')
param acrUsername string

@secure()
@description('ACR admin password')
param acrPassword string

@secure()
@description('Anthropic API key')
param anthropicApiKey string

@secure()
@description('Azure Service Bus connection string')
param serviceBusConnectionString string

@description('Name of the Service Bus queue')
param queueName string = 'tuicast-jobs'

@secure()
@description('Azure Blob Storage connection string')
param storageConnectionString string

// ─── Log Analytics workspace (required by Container Apps) ────────────────────
resource logWorkspace 'Microsoft.OperationalInsights/workspaces@2022-10-01' = {
  name: '${environmentName}-logs'
  location: location
  properties: {
    sku: {
      name: 'PerGB2018'
    }
    retentionInDays: 30
  }
}

// ─── Container Apps managed environment ──────────────────────────────────────
resource env 'Microsoft.App/managedEnvironments@2023-05-01' = {
  name: environmentName
  location: location
  properties: {
    appLogsConfiguration: {
      destination: 'log-analytics'
      logAnalyticsConfiguration: {
        customerId: logWorkspace.properties.customerId
        sharedKey: logWorkspace.listKeys().primarySharedKey
      }
    }
  }
}

// ─── API Container App ────────────────────────────────────────────────────────
resource apiApp 'Microsoft.App/containerApps@2023-05-01' = {
  name: appName
  location: location
  properties: {
    managedEnvironmentId: env.id
    configuration: {
      ingress: {
        external: true
        targetPort: 3000
      }
      registries: [
        {
          server: acrServer
          username: acrUsername
          passwordSecretRef: 'acr-password'
        }
      ]
      secrets: [
        { name: 'acr-password',            value: acrPassword }
        { name: 'anthropic-api-key',       value: anthropicApiKey }
        { name: 'service-bus-conn-str',    value: serviceBusConnectionString }
        { name: 'storage-conn-str',        value: storageConnectionString }
      ]
    }
    template: {
      containers: [
        {
          name: 'api'
          image: apiImage
          resources: {
            cpu: json('0.5')
            memory: '1Gi'
          }
          env: [
            { name: 'PORT',                            value: '3000' }
            { name: 'USE_QUEUE',                       value: 'true' }
            { name: 'SERVICE_BUS_QUEUE_NAME',          value: queueName }
            { name: 'ANTHROPIC_API_KEY',               secretRef: 'anthropic-api-key' }
            { name: 'SERVICE_BUS_CONNECTION_STRING',   secretRef: 'service-bus-conn-str' }
            { name: 'STORAGE_CONNECTION_STRING',       secretRef: 'storage-conn-str' }
          ]
        }
      ]
      scale: {
        minReplicas: 1
        maxReplicas: 3
      }
    }
  }
}

// ─── Outputs ─────────────────────────────────────────────────────────────────
output apiUrl string = 'https://${apiApp.properties.configuration.ingress.fqdn}'
output appName string = apiApp.name
output environmentName string = env.name
