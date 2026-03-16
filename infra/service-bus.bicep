// TUIcast – Azure Service Bus namespace and queue
// Deploy: az deployment group create -g <rg> -f service-bus.bicep

@description('Azure region for all resources')
param location string = resourceGroup().location

@description('Globally unique name for the Service Bus namespace')
param namespaceName string = 'tuicast-sb-${uniqueString(resourceGroup().id)}'

@description('Name of the job queue')
param queueName string = 'tuicast-jobs'

// ─── Service Bus namespace ──────────────────────────────────────────────────
resource sbNamespace 'Microsoft.ServiceBus/namespaces@2022-10-01-preview' = {
  name: namespaceName
  location: location
  sku: {
    name: 'Standard'
    tier: 'Standard'
  }
  properties: {
    minimumTlsVersion: '1.2'
  }
}

// ─── Job queue ───────────────────────────────────────────────────────────────
resource sbQueue 'Microsoft.ServiceBus/namespaces/queues@2022-10-01-preview' = {
  parent: sbNamespace
  name: queueName
  properties: {
    maxSizeInMegabytes: 1024
    // Allow messages to sit in the queue for up to 1 day.
    defaultMessageTimeToLive: 'P1D'
    // Move un-processable messages to the dead-letter sub-queue after 5 attempts.
    maxDeliveryCount: 5
    deadLetteringOnMessageExpiration: true
    lockDuration: 'PT5M'
  }
}

// ─── Auth rule (read + write) ────────────────────────────────────────────────
resource sbAuthRule 'Microsoft.ServiceBus/namespaces/authorizationRules@2022-10-01-preview' = {
  parent: sbNamespace
  name: 'TUIcastWorker'
  properties: {
    rights: ['Send', 'Listen', 'Manage']
  }
}

// ─── Outputs ─────────────────────────────────────────────────────────────────
output namespaceName string = sbNamespace.name
output queueName string = sbQueue.name
output connectionString string = sbAuthRule.listKeys().primaryConnectionString
