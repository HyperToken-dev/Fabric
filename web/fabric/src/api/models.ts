import type {
    AdminModel as ProtoModel,
    CatalogModel as ProtoCatalogModel,
} from '../gen/model_admin_pb';
import type { ClientModel as ProtoClientModel } from '../gen/model_client_pb';
import { modelAdminClient, modelClient } from '../rpc/clients';
import { callAdminRpc } from '../rpc/errors';
import { requireString, safeInteger } from '../rpc/values';

export const MODEL_STATUSES = { 1: 'Active', 2: 'Banned' } as const;
export const MODEL_TYPES = { 1: 'Text', 2: 'Video', 3: 'Image' } as const;

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

export type ClientModel = {
    modelName: string;
    modelType: number;
    channelName: string;
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

function toClientModel(model: ProtoClientModel, field: string): ClientModel {
    return {
        modelName: requireString(model.modelName, `${field}.modelName`),
        modelType: safeInteger(model.modelType, `${field}.modelType`),
        channelName: requireString(model.channelName, `${field}.channelName`),
    };
}

export async function listModels(channelId: number, signal?: AbortSignal): Promise<Model[]> {
    const response = await callAdminRpc(() =>
        modelAdminClient.listModels({ channelId }, { signal }),
    );
    return response.models.map((model, index) => toModel(model, `models[${index}]`));
}

export async function listClientModels(
    channelName = '',
    signal?: AbortSignal,
): Promise<ClientModel[]> {
    const response = await callAdminRpc(() => modelClient.listModels({ channelName }, { signal }));
    return response.models.map((model, index) => toClientModel(model, `models[${index}]`));
}

export async function listCatalogModels(
    apiFormat: number,
    signal?: AbortSignal,
): Promise<CatalogModel[]> {
    const response = await callAdminRpc(() =>
        modelAdminClient.listCatalogModels({ apiFormat }, { signal }),
    );
    return response.models.map((model, index) => toCatalogModel(model, `models[${index}]`));
}

export async function createModel(input: {
    modelName: string;
    channelId: number;
    status: number;
    modelType: number;
}): Promise<Model> {
    const response = await callAdminRpc(() => modelAdminClient.createModel(input));
    return requireModel(response.model);
}

export async function getModelInfo(modelId: number, signal?: AbortSignal): Promise<Model> {
    const response = await callAdminRpc(() =>
        modelAdminClient.getModelInfo({ modelId }, { signal }),
    );
    return requireModel(response.model);
}
