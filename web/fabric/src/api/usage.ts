import { parseArray, parseInteger, parseObject, parseString, parseTimestamp, postConnect } from './connect';

export type UsageLog = {
  usageId: string;
  keyId: number;
  modelId: number;
  channelId: number;
  promptTokens: number;
  completionTokens: number;
  createdAt: string | null;
};

export type UsageSummary = { promptTokens: number; completionTokens: number; totalTokens: number };

function optionalInteger(value: unknown, field: string): number {
  return value === undefined ? 0 : parseInteger(value, field);
}

function parseUsageLog(value: unknown, field: string): UsageLog {
  const log = parseObject(value, field);
  return {
    usageId: log.usageId === undefined ? '' : parseString(log.usageId, `${field}.usageId`, true),
    keyId: optionalInteger(log.keyId, `${field}.keyId`),
    modelId: optionalInteger(log.modelId, `${field}.modelId`),
    channelId: optionalInteger(log.channelId, `${field}.channelId`),
    promptTokens: optionalInteger(log.promptTokens, `${field}.promptTokens`),
    completionTokens: optionalInteger(log.completionTokens, `${field}.completionTokens`),
    createdAt: log.createdAt === undefined ? null : parseTimestamp(log.createdAt, `${field}.createdAt`),
  };
}

async function queryUsage(method: string, body: object, signal?: AbortSignal): Promise<UsageLog[]> {
  const response = await postConnect<{ usageLog?: unknown }>('UsageService', method, body, signal);
  return parseArray(response.usageLog, 'usageLog').map((log, index) => parseUsageLog(log, `usageLog[${index}]`));
}

export async function getUsageSummary(signal?: AbortSignal): Promise<UsageSummary> {
  const logs = await queryUsage('GetUsageSummary', {}, signal);
  const summary = logs[0] ?? { promptTokens: 0, completionTokens: 0 };
  return { promptTokens: summary.promptTokens, completionTokens: summary.completionTokens, totalTokens: summary.promptTokens + summary.completionTokens };
}

export const getUsageByChannelId = (channelId: number, signal?: AbortSignal) => queryUsage('GetUsageByChannelID', { channelId }, signal);
export const getUsageByModelId = (modelId: number, signal?: AbortSignal) => queryUsage('GetUsageByModelID', { modelId }, signal);
export const getUsageByKeyHash = (keyHash: string, signal?: AbortSignal) => queryUsage('GetUsageByKeyHash', { keyHash }, signal);
export const getUsageByDeadlineAndKeyHash = (keyHash: string, deadline: string, signal?: AbortSignal) => queryUsage('GetUsageByDeadlineAndKeyHash', { keyHash, deadline }, signal);
