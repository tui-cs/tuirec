// TUIcast – Azure Container Instances for isolated job execution
// Deploy: az deployment group create -g <rg> -f container-instance.bicep -p jobId=<id> ...
//
// Each recording job gets its own ACI instance that is destroyed when done.

@description('Azure region for all resources')
param location string = resourceGroup().location

@description('Unique job identifier (used to name the container instance)')
param jobId string

@description('Worker container image (e.g. myacr.azurecr.io/tuicast-worker:latest)')
param workerImage string

@description('Azure Container Registry login server')
param acrServer string

@description('Azure Container Registry username')
param acrUsername string

@secure()
@description('Azure Container Registry password')
param acrPassword string

@secure()
@description('Anthropic API key')
param anthropicApiKey string

@secure()
@description('Azure Service Bus connection string')
param serviceBusConnectionString string

@secure()
@description('Azure Blob Storage connection string')
param storageConnectionString string

@description('Name of the Service Bus queue')
param queueName string = 'tuicast-jobs'

// ─── Container group ─────────────────────────────────────────────────────────
resource containerGroup 'Microsoft.ContainerInstance/containerGroups@2023-05-01' = {
  name: 'tuicast-worker-${jobId}'
  location: location
  properties: {
    osType: 'Linux'
    restartPolicy: 'Never'
    imageRegistryCredentials: [
      {
        server: acrServer
        username: acrUsername
        password: acrPassword
      }
    ]
    containers: [
      {
        name: 'worker'
        properties: {
          image: workerImage
          resources: {
            requests: {
              cpu: 2
              memoryInGB: 4
            }
          }
          environmentVariables: [
            { name: 'ANTHROPIC_API_KEY',              secureValue: anthropicApiKey }
            { name: 'SERVICE_BUS_CONNECTION_STRING',  secureValue: serviceBusConnectionString }
            { name: 'SERVICE_BUS_QUEUE_NAME',         value: queueName }
            { name: 'STORAGE_CONNECTION_STRING',      secureValue: storageConnectionString }
            { name: 'AGG_PATH',                       value: '/usr/local/bin/agg' }
            { name: 'USE_QUEUE',                      value: 'true' }
          ]
          // No external port exposure needed; worker only reads from queue.
          ports: []
        }
      }
    ]
    // No network egress: jobs run in fully isolated containers.
    subnetIds: []
  }
}

// ─── Outputs ─────────────────────────────────────────────────────────────────
output containerGroupName string = containerGroup.name
output provisioningState string = containerGroup.properties.provisioningState
