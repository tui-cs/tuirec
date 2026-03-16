import {
  ServiceBusClient,
  ServiceBusSender,
  ServiceBusReceiver,
  ProcessErrorArgs,
  ServiceBusReceivedMessage,
} from "@azure/service-bus";
import { JobMessage } from "../types";

const QUEUE_NAME = process.env.SERVICE_BUS_QUEUE_NAME ?? "tuicast-jobs";

/** Lazily-initialised Service Bus client shared across the process. */
let _client: ServiceBusClient | null = null;

function getClient(): ServiceBusClient {
  if (!_client) {
    const connectionString = process.env.SERVICE_BUS_CONNECTION_STRING;
    if (!connectionString) {
      throw new Error(
        "SERVICE_BUS_CONNECTION_STRING environment variable is not set."
      );
    }
    _client = new ServiceBusClient(connectionString);
  }
  return _client;
}

/** Enqueue a new job message. */
export async function enqueueJob(message: JobMessage): Promise<void> {
  const sender: ServiceBusSender = getClient().createSender(QUEUE_NAME);
  try {
    await sender.sendMessages({
      body: message,
      messageId: message.jobId,
      contentType: "application/json",
    });
  } finally {
    await sender.close();
  }
}

/**
 * Start consuming job messages from the queue.
 *
 * @param handler - Called for each received message.  Must resolve/reject to
 *                  acknowledge or abandon the message.
 * @returns A function that, when called, stops the receiver.
 */
export function startWorkerReceiver(
  handler: (message: JobMessage) => Promise<void>
): () => Promise<void> {
  const receiver: ServiceBusReceiver = getClient().createReceiver(QUEUE_NAME, {
    receiveMode: "peekLock",
  });

  receiver.subscribe({
    async processMessage(msg: ServiceBusReceivedMessage) {
      try {
        const payload = msg.body as JobMessage;
        await handler(payload);
        await receiver.completeMessage(msg);
      } catch (err) {
        console.error("[queue] Job processing failed – abandoning message", err);
        await receiver.abandonMessage(msg);
      }
    },
    async processError(args: ProcessErrorArgs) {
      console.error("[queue] Service Bus error", args.error);
    },
  });

  return async () => {
    await receiver.close();
  };
}

/** Close the shared Service Bus client (call on process exit). */
export async function closeQueue(): Promise<void> {
  if (_client) {
    await _client.close();
    _client = null;
  }
}
