import type { ApiKey as ProtoApiKey } from '../gen/apiKey_pb';
import { apiKeyClient } from '../rpc/clients';
import { callAdminRpc } from '../rpc/errors';
import { requireString, timestampToIso } from '../rpc/values';

export type ApiKey = {
    keyName: string;
    keyHash: string;
    createdAt: string;
};

export type CreatedApiKey = ApiKey & { rawKey: string };

function toApiKey(key: ProtoApiKey, field: string): ApiKey {
    return {
        keyName: requireString(key.keyName, `${field}.keyName`),
        keyHash: requireString(key.keyHash, `${field}.keyHash`),
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

export async function listApiKeys(channelId: number, signal?: AbortSignal): Promise<ApiKey[]> {
    const response = await callAdminRpc(() =>
        apiKeyClient.listApiKeysByChannelID({ channelId }, { signal }),
    );
    return response.apiKeys.map((key, index) => toApiKey(key, `apiKeys[${index}]`));
}

export async function createApiKey(keyName: string, channelId: number): Promise<CreatedApiKey> {
    const response = await callAdminRpc(() => apiKeyClient.createApiKey({ keyName, channelId }));
    return toCreatedApiKey(response.apiKey);
}

export async function revokeApiKey(keyHash: string): Promise<void> {
    await callAdminRpc(() => apiKeyClient.revokeApiKey({ keyHash }));
}
