import type { Timestamp } from '@bufbuild/protobuf/wkt';
import { timestampDate, timestampFromDate } from '@bufbuild/protobuf/wkt';

export function requireString(value: string, field: string, allowEmpty = false): string {
    if (!allowEmpty && value.trim() === '') {
        throw new Error(`Invalid response field: ${field}`);
    }
    return value;
}

export function safeInteger(
    value: bigint | number | string,
    field: string,
    allowZero = true,
): number {
    let integer: bigint;
    try {
        if (typeof value === 'number') {
            if (!Number.isSafeInteger(value)) throw new Error();
            integer = BigInt(value);
        } else {
            if (typeof value === 'string' && !/^\d+$/.test(value)) throw new Error();
            integer = BigInt(value);
        }
    } catch {
        throw new Error(`Invalid response field: ${field}`);
    }

    const minimum = allowZero ? 0n : 1n;
    if (integer < minimum || integer > BigInt(Number.MAX_SAFE_INTEGER)) {
        throw new Error(`Invalid response field: ${field}`);
    }
    return Number(integer);
}

export function timestampToIso(value: Timestamp | undefined, field: string): string {
    if (!value) {
        throw new Error(`Invalid response field: ${field}`);
    }
    try {
        return timestampDate(value).toISOString();
    } catch {
        throw new Error(`Invalid response field: ${field}`);
    }
}

export function timestampFromIso(value: string, field: string): Timestamp {
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) {
        throw new Error(`Invalid request field: ${field}`);
    }
    return timestampFromDate(date);
}
