import { parseInteger, postConnect } from './connect';

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

function parseTotals(value: UsageTotalsResponse | undefined, prefix: string): UsageTotals {
  if (!value || typeof value !== 'object') {
    throw new Error(`Invalid dashboard field: ${prefix}`);
  }
  return {
    promptTokens: value.promptTokens === undefined ? 0 : parseInteger(value.promptTokens, `${prefix}.promptTokens`),
    completionTokens: value.completionTokens === undefined ? 0 : parseInteger(value.completionTokens, `${prefix}.completionTokens`),
    totalTokens: value.totalTokens === undefined ? 0 : parseInteger(value.totalTokens, `${prefix}.totalTokens`),
    requestCount: value.requestCount === undefined ? 0 : parseInteger(value.requestCount, `${prefix}.requestCount`),
  };
}

export async function getUsageDashboard(signal?: AbortSignal): Promise<UsageDashboard> {
  const payload = await postConnect<DashboardResponse>('UsageService', 'GetUsageDashboard', {}, signal);
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
