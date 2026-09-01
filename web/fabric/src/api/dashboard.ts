import { usageClient } from '../rpc/clients';
import { callAdminRpc } from '../rpc/errors';
import { safeInteger } from '../rpc/values';

type ProtoTotals = {
    promptTokens: bigint;
    completionTokens: bigint;
    totalTokens: bigint;
    requestCount: bigint;
};

export type UsageTotals = {
    promptTokens: number;
    completionTokens: number;
    totalTokens: number;
    requestCount: number;
};

export type UsageTimelinePoint = UsageTotals & {
    date: string;
};

export type UsageDashboard = {
    timeZone: string;
    today: UsageTotals;
    recentDays: UsageTimelinePoint[];
};

function toTotals(value: ProtoTotals | undefined, prefix: string): UsageTotals {
    if (!value) {
        throw new Error(`Invalid dashboard field: ${prefix}`);
    }
    return {
        promptTokens: safeInteger(value.promptTokens, `${prefix}.promptTokens`),
        completionTokens: safeInteger(value.completionTokens, `${prefix}.completionTokens`),
        totalTokens: safeInteger(value.totalTokens, `${prefix}.totalTokens`),
        requestCount: safeInteger(value.requestCount, `${prefix}.requestCount`),
    };
}

export async function getUsageDashboard(
    signal?: AbortSignal,
    ownerOpenid = '',
): Promise<UsageDashboard> {
    const payload = await callAdminRpc(() =>
        usageClient.getUsageDashboard({ ownerOpenid }, { signal }),
    );
    if (!payload.timeZone) {
        throw new Error('Invalid dashboard response');
    }

    return {
        timeZone: payload.timeZone,
        today: toTotals(payload.today, 'today'),
        recentDays: payload.recentDays.map((point, index) => {
            if (!/^\d{4}-\d{2}-\d{2}$/.test(point.date)) {
                throw new Error(`Invalid dashboard field: recentDays[${index}].date`);
            }
            return {
                date: point.date,
                ...toTotals(point, `recentDays[${index}]`),
            };
        }),
    };
}
