import {
  BlobServiceClient,
  StorageSharedKeyCredential,
  generateBlobSASQueryParameters,
  BlobSASPermissions,
  ContainerClient,
  BlobClient,
  BlockBlobClient,
} from "@azure/storage-blob";
import { Readable } from "stream";

const CONTAINER_GIFS = process.env.STORAGE_CONTAINER_GIFS ?? "gifs";
const CONTAINER_CASTS = process.env.STORAGE_CONTAINER_CASTS ?? "casts";
const CONTAINER_BINARIES =
  process.env.STORAGE_CONTAINER_BINARIES ?? "binaries";
/** Presigned URL TTL in seconds. Default: 1 hour. */
const SAS_TTL_SECONDS = parseInt(
  process.env.STORAGE_SAS_TTL_SECONDS ?? "3600",
  10
);

let _client: BlobServiceClient | null = null;

function getClient(): BlobServiceClient {
  if (!_client) {
    const connectionString = process.env.STORAGE_CONNECTION_STRING;
    if (!connectionString) {
      throw new Error(
        "STORAGE_CONNECTION_STRING environment variable is not set."
      );
    }
    _client = BlobServiceClient.fromConnectionString(connectionString);
  }
  return _client;
}

async function ensureContainer(name: string): Promise<ContainerClient> {
  const container = getClient().getContainerClient(name);
  await container.createIfNotExists();
  return container;
}

/** Upload a GIF file and return a short-lived download URL. */
export async function uploadGif(
  jobId: string,
  data: Buffer | Readable
): Promise<string> {
  const container = await ensureContainer(CONTAINER_GIFS);
  const blobName = `${jobId}.gif`;
  const blockBlob: BlockBlobClient = container.getBlockBlobClient(blobName);
  if (Buffer.isBuffer(data)) {
    await blockBlob.uploadData(data, {
      blobHTTPHeaders: { blobContentType: "image/gif" },
    });
  } else {
    await blockBlob.uploadStream(data as Readable, undefined, undefined, {
      blobHTTPHeaders: { blobContentType: "image/gif" },
    });
  }
  return generateSasUrl(container.accountName, CONTAINER_GIFS, blobName);
}

/** Upload an asciinema cast file and return a short-lived download URL. */
export async function uploadCast(
  jobId: string,
  data: Buffer | Readable
): Promise<string> {
  const container = await ensureContainer(CONTAINER_CASTS);
  const blobName = `${jobId}.cast`;
  const blockBlob: BlockBlobClient = container.getBlockBlobClient(blobName);
  if (Buffer.isBuffer(data)) {
    await blockBlob.uploadData(data, {
      blobHTTPHeaders: { blobContentType: "text/plain" },
    });
  } else {
    await blockBlob.uploadStream(data as Readable, undefined, undefined, {
      blobHTTPHeaders: { blobContentType: "text/plain" },
    });
  }
  return generateSasUrl(container.accountName, CONTAINER_CASTS, blobName);
}

/** Upload a user-supplied binary and return its blob name. */
export async function uploadBinary(
  blobName: string,
  data: Buffer | Readable
): Promise<string> {
  const container = await ensureContainer(CONTAINER_BINARIES);
  const blockBlob: BlockBlobClient =
    container.getBlockBlobClient(blobName);
  if (Buffer.isBuffer(data)) {
    await blockBlob.uploadData(data);
  } else {
    await blockBlob.uploadStream(data as Readable);
  }
  return blobName;
}

/** Download a binary blob to a local path. */
export async function downloadBinary(
  blobName: string,
  localPath: string
): Promise<void> {
  const container = await ensureContainer(CONTAINER_BINARIES);
  const blobClient: BlobClient = container.getBlobClient(blobName);
  await blobClient.downloadToFile(localPath);
}

/**
 * Generate a short-lived SAS URL for a blob.
 * Falls back to a plain blob URL if the client uses connection-string-based
 * anonymous access (development emulator).
 */
function generateSasUrl(
  accountName: string,
  containerName: string,
  blobName: string
): string {
  const connectionString = process.env.STORAGE_CONNECTION_STRING ?? "";

  // Extract account key from connection string (if present).
  const keyMatch = connectionString.match(/AccountKey=([^;]+)/);
  if (!keyMatch) {
    // Azurite / emulator without a key – return plain URL.
    return `http://127.0.0.1:10000/${accountName}/${containerName}/${blobName}`;
  }

  const credential = new StorageSharedKeyCredential(
    accountName,
    keyMatch[1]
  );
  const expiresOn = new Date(Date.now() + SAS_TTL_SECONDS * 1000);
  const sasToken = generateBlobSASQueryParameters(
    {
      containerName,
      blobName,
      permissions: BlobSASPermissions.parse("r"),
      expiresOn,
    },
    credential
  ).toString();

  return `https://${accountName}.blob.core.windows.net/${containerName}/${blobName}?${sasToken}`;
}
