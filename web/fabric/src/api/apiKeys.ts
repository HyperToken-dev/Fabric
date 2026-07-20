import { parseArray, parseObject, parseString, parseTimestamp, postConnect } from './connect';

export type ApiKey = {
  keyName: string;
  keyHash: string;
  createdAt: string;
};

export type CreatedApiKey = ApiKey & { rawKey: string };

function parseApiKey(value: unknown, field: string, requireRawKey = false): ApiKey | CreatedApiKey {
  const key = parseObject(value, field);
  const parsed: ApiKey = {
    keyName: parseString(key.keyName, `${field}.keyName`),
    keyHash: parseString(key.keyHash, `${field}.keyHash`),
    createdAt: parseTimestamp(key.createdAt, `${field}.createdAt`),
  };
  return requireRawKey ? { ...parsed, rawKey: parseString(key.rawKey, `${field}.rawKey`) } : parsed;
}

export async function listApiKeys(channelId: number, signal?: AbortSignal): Promise<ApiKey[]> {
  const response = await postConnect<{ apiKeys?: unknown }>('ManageApiKeyService', 'ListApiKeysByChannelID', { channelId }, signal);
  return parseArray(response.apiKeys, 'apiKeys').map((key, index) => parseApiKey(key, `apiKeys[${index}]`) as ApiKey);
}

export async function createApiKey(keyName: string, channelId: number): Promise<CreatedApiKey> {
  const response = await postConnect<{ apiKey?: unknown }>('ManageApiKeyService', 'CreateApiKey', { keyName, channelId });
  return parseApiKey(response.apiKey, 'apiKey', true) as CreatedApiKey;
}

export async function revokeApiKey(keyHash: string): Promise<void> {
  await postConnect('ManageApiKeyService', 'RevokeApiKey', { keyHash });
}
