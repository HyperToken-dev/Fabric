import type { AdminApiKey } from '../gen/api_key_admin_pb';
import type { ClientApiKey } from '../gen/api_key_client_pb';
import { apiKeyAdminClient, apiKeyClient } from '../rpc/clients';
import { callAdminRpc } from '../rpc/errors';
import { requireString, safeInteger, timestampToIso } from '../rpc/values';

export type ApiKey = {
    keyId: number;
    keyName: string;
    keyHash: string;
    channelId?: number;
    channelName: string;
    ownerOpenid: string;
    createdAt: string;
};

export type CreatedApiKey = ApiKey & { rawKey: string };

export type ApiKeyChannel = {
    channelName: string;
    channelId?: number;
};

function toApiKey(key: AdminApiKey | ClientApiKey, field: string, channelName = ''): ApiKey {
    return {
        keyId: safeInteger(key.keyId, `${field}.keyId`, false),
        keyName: requireString(key.keyName, `${field}.keyName`),
        keyHash: requireString(key.keyHash, `${field}.keyHash`),
        channelId:
            'channelId' in key
                ? safeInteger(key.channelId, `${field}.channelId`, false)
                : undefined,
        channelName:
            'channelName' in key
                ? requireString(key.channelName, `${field}.channelName`, true)
                : channelName,
        ownerOpenid: requireString(key.ownerOpenid, `${field}.ownerOpenid`),
        createdAt: timestampToIso(key.createdAt, `${field}.createdAt`),
    };
}

function toCreatedApiKey(
    key: AdminApiKey | ClientApiKey | undefined,
    channelName = '',
): CreatedApiKey {
    if (!key) throw new Error('Invalid response field: apiKey');
    const parsed: ApiKey = {
        ...toApiKey(key, 'apiKey', channelName),
    };
    return { ...parsed, rawKey: requireString(key.rawKey, 'apiKey.rawKey') };
}

export async function listClientApiKeys(signal?: AbortSignal): Promise<ApiKey[]> {
    const response = await callAdminRpc(() => apiKeyClient.listApiKeys({}, { signal }));
    return response.apiKeys.map((key, index) => toApiKey(key, `apiKeys[${index}]`));
}

export async function listAdminApiKeys(
    channels: ApiKeyChannel[],
    signal?: AbortSignal,
): Promise<ApiKey[]> {
    const responses = await Promise.all(
        channels.map(async (channel) => {
            const response = await callAdminRpc(() =>
                apiKeyAdminClient.listApiKeysByChannelName(
                    { channelName: channel.channelName },
                    { signal },
                ),
            );
            return response.apiKeys.map((key, index) =>
                toApiKey(key, `apiKeys[${channel.channelName}][${index}]`, channel.channelName),
            );
        }),
    );
    return responses.flat();
}

export async function createClientApiKey(
    keyName: string,
    channelName: string,
): Promise<CreatedApiKey> {
    const response = await callAdminRpc(() => apiKeyClient.createApiKey({ keyName, channelName }));
    return toCreatedApiKey(response.apiKey);
}

export async function createAdminApiKey(
    keyName: string,
    channel: ApiKeyChannel,
): Promise<CreatedApiKey> {
    if (!channel.channelId) throw new Error('Invalid channel selection');
    const response = await callAdminRpc(() =>
        apiKeyAdminClient.createApiKey({ keyName, channelId: channel.channelId }),
    );
    return toCreatedApiKey(response.apiKey, channel.channelName);
}

export async function revokeClientApiKey(keyHash: string): Promise<void> {
    await callAdminRpc(() => apiKeyClient.revokeApiKey({ keyHash }));
}

export async function revokeAdminApiKey(keyHash: string): Promise<void> {
    await callAdminRpc(() => apiKeyAdminClient.revokeApiKey({ keyHash }));
}
