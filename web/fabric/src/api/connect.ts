import axios from 'axios';

export async function postConnect<T>(service: string, method: string, body: object, signal?: AbortSignal): Promise<T> {
  try {
    const response = await axios.post<T>(`/admin-api/proto.${service}/${method}`, body, {
      signal,
      headers: { 'Content-Type': 'application/json' },
    });
    return response.data;
  } catch (error) {
    if (axios.isCancel(error)) {
      throw error;
    }
    if (axios.isAxiosError(error)) {
      const payload = error.response?.data;
      if (payload && typeof payload === 'object' && 'message' in payload && typeof payload.message === 'string') {
        throw new Error(payload.message);
      }
      throw new Error(error.response ? `Request failed (${error.response.status})` : 'Unable to reach the admin API');
    }
    throw error instanceof Error ? error : new Error('Unexpected admin API error');
  }
}

export function parseObject(value: unknown, field: string): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new Error(`Invalid response field: ${field}`);
  }
  return value as Record<string, unknown>;
}

export function parseArray(value: unknown, field: string): unknown[] {
  if (value === undefined) {
    return [];
  }
  if (!Array.isArray(value)) {
    throw new Error(`Invalid response field: ${field}`);
  }
  return value;
}

export function parseString(value: unknown, field: string, allowEmpty = false): string {
  if (typeof value !== 'string' || (!allowEmpty && value.trim() === '')) {
    throw new Error(`Invalid response field: ${field}`);
  }
  return value;
}

export function parseInteger(value: unknown, field: string, allowZero = true): number {
  if ((typeof value !== 'string' && typeof value !== 'number') || String(value).trim() === '') {
    throw new Error(`Invalid response field: ${field}`);
  }
  const parsed = Number(value);
  if (!Number.isSafeInteger(parsed) || parsed < (allowZero ? 0 : 1)) {
    throw new Error(`Invalid response field: ${field}`);
  }
  return parsed;
}

export function parseTimestamp(value: unknown, field: string): string {
  const timestamp = parseString(value, field);
  if (Number.isNaN(Date.parse(timestamp))) {
    throw new Error(`Invalid response field: ${field}`);
  }
  return timestamp;
}

export function enumLabel(value: number, labels: Readonly<Record<number, string>>): string {
  return labels[value] ?? `Unknown (${value})`;
}
