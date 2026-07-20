import axios from 'axios';

type IntegerValue = string | number;

type UsageTotalsResponse = {
  promptTokens?: IntegerValue;
  completionTokens?: IntegerValue;
  totalTokens?: IntegerValue;
  requestCount?: IntegerValue;
};

type UsageTimelinePointResponse = UsageTotalsResponse & {
  date?: unknown;
};

type DashboardResponse = {
  timeZone?: unknown;
  today?: UsageTotalsResponse;
  recentDays?: UsageTimelinePointResponse[];
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

function parseInteger(value: IntegerValue | undefined, field: string): number {
  if (value === undefined) {
    return 0;
  }
  if ((typeof value !== 'string' && typeof value !== 'number') || String(value).trim() === '') {
    throw new Error(`Invalid dashboard field: ${field}`);
  }
  const parsed = Number(value);
  if (!Number.isSafeInteger(parsed) || parsed < 0) {
    throw new Error(`Invalid dashboard field: ${field}`);
  }
  return parsed;
}

function parseTotals(value: UsageTotalsResponse | undefined, prefix: string): UsageTotals {
  if (!value || typeof value !== 'object') {
    throw new Error(`Invalid dashboard field: ${prefix}`);
  }
  return {
    promptTokens: parseInteger(value.promptTokens, `${prefix}.promptTokens`),
    completionTokens: parseInteger(value.completionTokens, `${prefix}.completionTokens`),
    totalTokens: parseInteger(value.totalTokens, `${prefix}.totalTokens`),
    requestCount: parseInteger(value.requestCount, `${prefix}.requestCount`),
  };
}

export async function getUsageDashboard(signal?: AbortSignal): Promise<UsageDashboard> {
  const response = await axios.post<DashboardResponse>(
    '/admin-api/proto.UsageService/GetUsageDashboard',
    {},
    {
      signal,
      headers: { 'Content-Type': 'application/json' },
    },
  );
  const payload = response.data;
  if (!payload || typeof payload.timeZone !== 'string' || !Array.isArray(payload.recentDays)) {
    throw new Error('Invalid dashboard response');
  }

  return {
    timeZone: payload.timeZone,
    today: parseTotals(payload.today, 'today'),
    recentDays: payload.recentDays.map((point, index) => {
      if (typeof point.date !== 'string' || !/^\d{4}-\d{2}-\d{2}$/.test(point.date)) {
        throw new Error(`Invalid dashboard field: recentDays[${index}].date`);
      }
      return {
        date: point.date,
        ...parseTotals(point, `recentDays[${index}]`),
      };
    }),
  };
}
