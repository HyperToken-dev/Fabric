import type { AdminChannel as ProtoChannel } from '../gen/channel_admin_pb';
import type { ClientChannel as ProtoClientChannel } from '../gen/channel_client_pb';
import { channelAdminClient, channelClient } from '../rpc/clients';
import { callAdminRpc } from '../rpc/errors';
import { requireString, safeInteger, timestampToIso } from '../rpc/values';

export const CHANNEL_STATUSES = { 1: 'Active', 2: 'Banned', 3: 'Pending' } as const;
export const API_FORMATS = {
    1: 'OpenAI',
    2: 'Alibaba Bailian',
    3: 'Seedance',
    4: 'Google',
    5: 'Extrotec',
} as const;
export const CATALOG_API_FORMATS = new Set<number>([1, 2, 3, 4, 5]);
export const API_FORMAT_DEFAULT_BASE_URLS: Record<number, string> = {
    1: 'https://api.openai.com',
    2: 'https://dashscope.aliyuncs.com',
    3: 'https://ark.cn-beijing.volces.com',
    4: 'https://generativelanguage.googleapis.com',
    5: 'https://api.extrotec.com',
};

export type Channel = {
    channelId: number;
    channelName: string;
    createdAt: string;
    status: number;
    baseUrl: string;
    apiFormat: number;
};

export type ClientChannel = {
    channelName: string;
};

function toChannel(channel: ProtoChannel, field: string): Channel {
    return {
        channelId: safeInteger(channel.channelId, `${field}.channelId`, false),
        channelName: requireString(channel.channelName, `${field}.channelName`),
        createdAt: timestampToIso(channel.createdAt, `${field}.createdAt`),
        status: safeInteger(channel.status, `${field}.status`),
        baseUrl: requireString(channel.baseUrl, `${field}.baseUrl`, true),
        apiFormat: safeInteger(channel.apiFormat, `${field}.apiFormat`),
    };
}

function requireChannel(channel: ProtoChannel | undefined): Channel {
    if (!channel) throw new Error('Invalid response field: channel');
    return toChannel(channel, 'channel');
}

function toClientChannel(channel: ProtoClientChannel, field: string): ClientChannel {
    return {
        channelName: requireString(channel.channelName, `${field}.channelName`),
    };
}

export async function listChannels(signal?: AbortSignal): Promise<Channel[]> {
    const response = await callAdminRpc(() => channelAdminClient.listChannels({}, { signal }));
    return response.channels.map((channel, index) => toChannel(channel, `channels[${index}]`));
}

export async function listClientChannels(signal?: AbortSignal): Promise<ClientChannel[]> {
    const response = await callAdminRpc(() => channelClient.listChannels({}, { signal }));
    return response.channels.map((channel, index) =>
        toClientChannel(channel, `channels[${index}]`),
    );
}

export async function createChannel(input: {
    channelName: string;
    baseUrl: string;
    apiFormat: number;
    providerKey: string;
}): Promise<Channel> {
    const response = await callAdminRpc(() => channelAdminClient.createChannel(input));
    return requireChannel(response.channel);
}

export async function updateChannelName(channelId: number, channelName: string): Promise<Channel> {
    const response = await callAdminRpc(() =>
        channelAdminClient.updateChannelName({ channelId, channelName }),
    );
    return requireChannel(response.channel);
}

export async function updateChannelStatus(channelId: number, status: number): Promise<Channel> {
    const response = await callAdminRpc(() =>
        channelAdminClient.updateChannelStatus({ channelId, status }),
    );
    return requireChannel(response.channel);
}

export async function updateChannelBaseUrl(channelId: number, baseUrl: string): Promise<Channel> {
    const response = await callAdminRpc(() =>
        channelAdminClient.updateChannelBaseURL({ channelId, baseUrl }),
    );
    return requireChannel(response.channel);
}

export async function updateChannelApiFormat(
    channelId: number,
    apiFormat: number,
): Promise<Channel> {
    const response = await callAdminRpc(() =>
        channelAdminClient.updateChannelAPIFormat({ channelId, apiFormat }),
    );
    return requireChannel(response.channel);
}

export async function updateChannelProviderKey(
    channelId: number,
    providerKey: string,
): Promise<void> {
    await callAdminRpc(() =>
        channelAdminClient.updateChannelProviderKey({ channelId, providerKey }),
    );
}
