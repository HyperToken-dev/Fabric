import type { Channel as ProtoChannel } from '../gen/channel_pb';
import { channelClient } from '../rpc/clients';
import { callAdminRpc } from '../rpc/errors';
import { requireString, safeInteger, timestampToIso } from '../rpc/values';

export const CHANNEL_STATUSES = { 1: 'Active', 2: 'Banned', 3: 'Pending' } as const;
export const API_FORMATS = { 1: 'OpenAI' } as const;

export type Channel = {
    channelId: number;
    channelName: string;
    createdAt: string;
    status: number;
    baseUrl: string;
    apiFormat: number;
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

export async function listChannels(signal?: AbortSignal): Promise<Channel[]> {
    const response = await callAdminRpc(() => channelClient.listChannels({}, { signal }));
    return response.channels.map((channel, index) => toChannel(channel, `channels[${index}]`));
}

export async function createChannel(input: {
    channelName: string;
    baseUrl: string;
    apiFormat: number;
    providerKey: string;
}): Promise<Channel> {
    const response = await callAdminRpc(() => channelClient.createChannel(input));
    return requireChannel(response.channel);
}

export async function updateChannelName(channelId: number, channelName: string): Promise<Channel> {
    const response = await callAdminRpc(() =>
        channelClient.updateChannelName({ channelId, channelName }),
    );
    return requireChannel(response.channel);
}

export async function updateChannelStatus(channelId: number, status: number): Promise<Channel> {
    const response = await callAdminRpc(() =>
        channelClient.updateChannelStatus({ channelId, status }),
    );
    return requireChannel(response.channel);
}

export async function updateChannelBaseUrl(channelId: number, baseUrl: string): Promise<Channel> {
    const response = await callAdminRpc(() =>
        channelClient.updateChannelBaseURL({ channelId, baseUrl }),
    );
    return requireChannel(response.channel);
}

export async function updateChannelApiFormat(
    channelId: number,
    apiFormat: number,
): Promise<Channel> {
    const response = await callAdminRpc(() =>
        channelClient.updateChannelAPIFormat({ channelId, apiFormat }),
    );
    return requireChannel(response.channel);
}

export async function updateChannelProviderKey(
    channelId: number,
    providerKey: string,
): Promise<void> {
    await callAdminRpc(() => channelClient.updateChannelProviderKey({ channelId, providerKey }));
}
