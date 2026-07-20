import { parseArray, parseInteger, parseObject, parseString, postConnect } from './connect';

export const MODEL_STATUSES = { 1: 'Active', 2: 'Banned' } as const;
export const MODEL_TYPES = { 1: 'Text' } as const;

export type Model = {
  modelId: number;
  modelName: string;
  channelId: number;
  status: number;
  modelType: number;
};

function parseModel(value: unknown, field: string): Model {
  const model = parseObject(value, field);
  return {
    modelId: parseInteger(model.modelId, `${field}.modelId`, false),
    modelName: parseString(model.modelName, `${field}.modelName`),
    channelId: parseInteger(model.channelId, `${field}.channelId`, false),
    status: parseInteger(model.status, `${field}.status`),
    modelType: parseInteger(model.modelType, `${field}.modelType`),
  };
}

export async function listModels(channelId: number, signal?: AbortSignal): Promise<Model[]> {
  const response = await postConnect<{ models?: unknown }>('ModelService', 'ListModels', { channelId }, signal);
  return parseArray(response.models, 'models').map((model, index) => parseModel(model, `models[${index}]`));
}

export async function createModel(input: { modelName: string; channelId: number; status: number; modelType: number }): Promise<Model> {
  const response = await postConnect<{ model?: unknown }>('ModelService', 'CreateModel', input);
  return parseModel(response.model, 'model');
}

export async function getModelInfo(modelId: number, signal?: AbortSignal): Promise<Model> {
  const response = await postConnect<{ model?: unknown }>('ModelService', 'GetModelInfo', { modelId }, signal);
  return parseModel(response.model, 'model');
}
