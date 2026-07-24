import type { IntegralLog as ProtoIntegralLog } from '../gen/integral_pb';
import { integralLogClient } from '../rpc/clients';
import { callAdminRpc } from '../rpc/errors';
import { requireString, safeInteger, timestampToIso } from '../rpc/values';

export type IntegralLog = {
    id: number;
    context: string;
    response: string;
    keyId: number;
    createdAt: string;
};

export type IntegralLogList = {
    logs: IntegralLog[];
    total: number;
};

function toIntegralLog(log: ProtoIntegralLog, field: string): IntegralLog {
    return {
        id: safeInteger(log.id, `${field}.id`, false),
        context: requireString(log.context, `${field}.context`),
        response: requireString(log.response, `${field}.response`, true),
        keyId: safeInteger(log.keyId, `${field}.keyId`, false),
        createdAt: timestampToIso(log.createdAt, `${field}.createdAt`),
    };
}

function requireIntegralLog(log: ProtoIntegralLog | undefined): IntegralLog {
    if (!log) throw new Error('Invalid response field: log');
    return toIntegralLog(log, 'log');
}

export async function listIntegralLogs(
    input: { keyId?: number; limit?: number; offset?: number } = {},
    signal?: AbortSignal,
): Promise<IntegralLogList> {
    const response = await callAdminRpc(() =>
        integralLogClient.listIntegralLogs(
            {
                keyId: input.keyId ?? 0,
                limit: input.limit ?? 100,
                offset: input.offset ?? 0,
            },
            { signal },
        ),
    );
    return {
        logs: response.logs.map((log, index) => toIntegralLog(log, `logs[${index}]`)),
        total: safeInteger(response.total, 'total'),
    };
}

export async function createIntegralLog(input: {
    context: string;
    response: string;
    keyId: number;
}): Promise<IntegralLog> {
    const response = await callAdminRpc(() => integralLogClient.createIntegralLog(input));
    return requireIntegralLog(response.log);
}

export async function updateIntegralLog(input: {
    id: number;
    context: string;
    response: string;
}): Promise<IntegralLog> {
    const response = await callAdminRpc(() => integralLogClient.updateIntegralLog(input));
    return requireIntegralLog(response.log);
}

export async function deleteIntegralLog(id: number): Promise<void> {
    await callAdminRpc(() => integralLogClient.deleteIntegralLog({ id }));
}
