import { parseArray, parseInteger, parseObject, parseString, parseTimestamp, postConnect } from './connect';

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

type ChannelResponse = { channel?: unknown };

function parseChannel(value: unknown, field: string): Channel {
  const channel = parseObject(value, field);
  return {
    channelId: parseInteger(channel.channelId, `${field}.channelId`, false),
    channelName: parseString(channel.channelName, `${field}.channelName`),
    createdAt: parseTimestamp(channel.createdAt, `${field}.createdAt`),
    status: parseInteger(channel.status, `${field}.status`),
    baseUrl: parseString(channel.baseUrl, `${field}.baseUrl`, true),
    apiFormat: parseInteger(channel.apiFormat, `${field}.apiFormat`),
  };
}

async function channelMutation(method: string, body: object): Promise<Channel> {
  const response = await postConnect<ChannelResponse>('ChannelService', method, body);
  return parseChannel(response.channel, 'channel');
}

export async function listChannels(signal?: AbortSignal): Promise<Channel[]> {
  const response = await postConnect<{ channels?: unknown }>('ChannelService', 'ListChannels', {}, signal);
  return parseArray(response.channels, 'channels').map((channel, index) => parseChannel(channel, `channels[${index}]`));
}

export function createChannel(input: { channelName: string; baseUrl: string; apiFormat: number; providerKey: string }): Promise<Channel> {
  return channelMutation('CreateChannel', input);
}

export function updateChannelName(channelId: number, channelName: string): Promise<Channel> {
  return channelMutation('UpdateChannelName', { channelId, channelName });
}

export function updateChannelStatus(channelId: number, status: number): Promise<Channel> {
  return channelMutation('UpdateChannelStatus', { channelId, status });
}

export function updateChannelBaseUrl(channelId: number, baseUrl: string): Promise<Channel> {
  return channelMutation('UpdateChannelBaseURL', { channelId, baseUrl });
}

export function updateChannelApiFormat(channelId: number, apiFormat: number): Promise<Channel> {
  return channelMutation('UpdateChannelAPIFormat', { channelId, apiFormat });
}

export async function updateChannelProviderKey(channelId: number, providerKey: string): Promise<void> {
  await channelMutation('UpdateChannelProviderKey', { channelId, providerKey });
}
