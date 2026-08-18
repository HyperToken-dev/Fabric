import type { ClientApiKey as ProtoApiKey } from '../gen/api_key_client_pb';
import { apiKeyClient } from '../rpc/clients';
import { callAdminRpc } from '../rpc/errors';
import { requireString, timestampToIso } from '../rpc/values';

export type ApiKey = {
    keyName: string;
    keyHash: string;
    channelName: string;
    createdAt: string;
};

export type CreatedApiKey = ApiKey & { rawKey: string };

function toApiKey(key: ProtoApiKey, field: string): ApiKey {
    return {
        keyName: requireString(key.keyName, `${field}.keyName`),
        keyHash: requireString(key.keyHash, `${field}.keyHash`),
        channelName: requireString(key.channelName, `${field}.channelName`, true),
        createdAt: timestampToIso(key.createdAt, `${field}.createdAt`),
    };
}

function toCreatedApiKey(key: ProtoApiKey | undefined): CreatedApiKey {
    if (!key) throw new Error('Invalid response field: apiKey');
    const parsed: ApiKey = {
        ...toApiKey(key, 'apiKey'),
    };
    return { ...parsed, rawKey: requireString(key.rawKey, 'apiKey.rawKey') };
}

export async function listApiKeys(signal?: AbortSignal): Promise<ApiKey[]> {
    const response = await callAdminRpc(() => apiKeyClient.listApiKeys({}, { signal }));
    return response.apiKeys.map((key, index) => toApiKey(key, `apiKeys[${index}]`));
}

export async function createApiKey(keyName: string, channelName: string): Promise<CreatedApiKey> {
    const response = await callAdminRpc(() => apiKeyClient.createApiKey({ keyName, channelName }));
    return toCreatedApiKey(response.apiKey);
}

export async function revokeApiKey(keyHash: string): Promise<void> {
    await callAdminRpc(() => apiKeyClient.revokeApiKey({ keyHash }));
}
