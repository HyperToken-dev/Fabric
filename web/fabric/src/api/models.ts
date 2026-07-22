import type { CatalogModel as ProtoCatalogModel, Model as ProtoModel } from '../gen/model_pb';
import { modelClient } from '../rpc/clients';
import { callAdminRpc } from '../rpc/errors';
import { requireString, safeInteger } from '../rpc/values';

export const MODEL_STATUSES = { 1: 'Active', 2: 'Banned' } as const;
export const MODEL_TYPES = { 1: 'Text', 2: 'Video' } as const;

export type Model = {
    modelId: number;
    modelName: string;
    channelId: number;
    status: number;
    modelType: number;
};

export type CatalogModel = {
    modelName: string;
    modelType: number;
};

function toCatalogModel(model: ProtoCatalogModel, field: string): CatalogModel {
    return {
        modelName: requireString(model.modelName, `${field}.modelName`),
        modelType: safeInteger(model.modelType, `${field}.modelType`),
    };
}

function toModel(model: ProtoModel, field: string): Model {
    return {
        modelId: safeInteger(model.modelId, `${field}.modelId`, false),
        modelName: requireString(model.modelName, `${field}.modelName`),
        channelId: safeInteger(model.channelId, `${field}.channelId`, false),
        status: safeInteger(model.status, `${field}.status`),
        modelType: safeInteger(model.modelType, `${field}.modelType`),
    };
}

function requireModel(model: ProtoModel | undefined): Model {
    if (!model) throw new Error('Invalid response field: model');
    return toModel(model, 'model');
}

export async function listModels(channelId: number, signal?: AbortSignal): Promise<Model[]> {
    const response = await callAdminRpc(() => modelClient.listModels({ channelId }, { signal }));
    return response.models.map((model, index) => toModel(model, `models[${index}]`));
}

export async function listCatalogModels(
    apiFormat: number,
    signal?: AbortSignal,
): Promise<CatalogModel[]> {
    const response = await callAdminRpc(() =>
        modelClient.listCatalogModels({ apiFormat }, { signal }),
    );
    return response.models.map((model, index) => toCatalogModel(model, `models[${index}]`));
}

export async function createModel(input: {
    modelName: string;
    channelId: number;
    status: number;
    modelType: number;
}): Promise<Model> {
    const response = await callAdminRpc(() => modelClient.createModel(input));
    return requireModel(response.model);
}

export async function getModelInfo(modelId: number, signal?: AbortSignal): Promise<Model> {
    const response = await callAdminRpc(() => modelClient.getModelInfo({ modelId }, { signal }));
    return requireModel(response.model);
}
