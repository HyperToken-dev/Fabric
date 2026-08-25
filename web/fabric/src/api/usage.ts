import type { GetUsageResponse, UsageLog as ProtoUsageLog } from '../gen/usage_pb';
import { usageClient } from '../rpc/clients';
import { callAdminRpc } from '../rpc/errors';
import { safeInteger, timestampFromIso, timestampToIso } from '../rpc/values';

export type UsageLog = {
    usageId: string;
    keyId: number;
    modelId: number;
    channelId: number;
    ownerOpenid: string;
    promptTokens: number;
    completionTokens: number;
    createdAt: string | null;
};

export type UsageSummary = { promptTokens: number; completionTokens: number; totalTokens: number };

function toUsageLog(log: ProtoUsageLog, field: string): UsageLog {
    return {
        usageId: log.usageId,
        keyId: safeInteger(log.keyId, `${field}.keyId`),
        modelId: safeInteger(log.modelId, `${field}.modelId`),
        channelId: safeInteger(log.channelId, `${field}.channelId`),
        ownerOpenid: log.ownerOpenid,
        promptTokens: safeInteger(log.promptTokens || '0', `${field}.promptTokens`),
        completionTokens: safeInteger(log.completionTokens || '0', `${field}.completionTokens`),
        createdAt: log.createdAt ? timestampToIso(log.createdAt, `${field}.createdAt`) : null,
    };
}

function toUsageLogs(response: GetUsageResponse): UsageLog[] {
    return response.usageLog.map((log, index) => toUsageLog(log, `usageLog[${index}]`));
}

export async function getUsageSummary(signal?: AbortSignal): Promise<UsageSummary> {
    const response = await callAdminRpc(() => usageClient.getUsageSummary({}, { signal }));
    const logs = toUsageLogs(response);
    const summary = logs[0] ?? { promptTokens: 0, completionTokens: 0 };
    return {
        promptTokens: summary.promptTokens,
        completionTokens: summary.completionTokens,
        totalTokens: summary.promptTokens + summary.completionTokens,
    };
}

export async function getUsageByChannelId(
    channelId: number,
    signal?: AbortSignal,
): Promise<UsageLog[]> {
    return toUsageLogs(
        await callAdminRpc(() => usageClient.getUsageByChannelID({ channelId }, { signal })),
    );
}

export async function getUsageByModelId(
    modelId: number,
    signal?: AbortSignal,
): Promise<UsageLog[]> {
    return toUsageLogs(
        await callAdminRpc(() => usageClient.getUsageByModelID({ modelId }, { signal })),
    );
}

export async function getUsageByKeyHash(
    keyHash: string,
    signal?: AbortSignal,
): Promise<UsageLog[]> {
    return toUsageLogs(
        await callAdminRpc(() => usageClient.getUsageByKeyHash({ keyHash }, { signal })),
    );
}

export async function getUsageByDeadlineAndKeyHash(
    keyHash: string,
    deadline: string,
    signal?: AbortSignal,
): Promise<UsageLog[]> {
    const request = { keyHash, deadline: timestampFromIso(deadline, 'deadline') };
    return toUsageLogs(
        await callAdminRpc(() => usageClient.getUsageByDeadlineAndKeyHash(request, { signal })),
    );
}
