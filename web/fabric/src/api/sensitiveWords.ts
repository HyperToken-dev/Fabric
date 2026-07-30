import type {
    SensitiveDictionary as ProtoSensitiveDictionary,
    SensitiveDictionarySummary as ProtoSensitiveDictionarySummary,
    SensitiveWordSnapshot as ProtoSensitiveWordSnapshot,
} from '../gen/sensitive_pb';
import { sensitiveWordClient } from '../rpc/clients';
import { callAdminRpc } from '../rpc/errors';
import { requireString, safeInteger, timestampToIso } from '../rpc/values';

export type SensitiveWordSnapshot = {
    enabled: boolean;
    version: number;
    loadedAt: string;
    dictionaryCount: number;
};

export type SensitiveWordStatus = {
    enabled: boolean;
    storeInitialized: boolean;
    snapshot: SensitiveWordSnapshot | null;
};

export type SensitiveDictionarySummary = {
    name: string;
    effectModels: string[];
    enabled: boolean;
    wordCount: number;
};

export type SensitiveDictionary = SensitiveDictionarySummary & {
    words: string[];
};

function toSnapshot(
    snapshot: ProtoSensitiveWordSnapshot | undefined,
): SensitiveWordSnapshot | null {
    if (!snapshot) return null;
    return {
        enabled: snapshot.enabled,
        version: safeInteger(snapshot.version, 'snapshot.version'),
        loadedAt: timestampToIso(snapshot.loadedAt, 'snapshot.loadedAt'),
        dictionaryCount: safeInteger(snapshot.dictionaryCount, 'snapshot.dictionaryCount'),
    };
}

function toSummary(
    dictionary: ProtoSensitiveDictionarySummary,
    field: string,
): SensitiveDictionarySummary {
    return {
        name: requireString(dictionary.name, `${field}.name`),
        effectModels: dictionary.effectModels.map((model, index) =>
            requireString(model, `${field}.effectModels[${index}]`),
        ),
        enabled: dictionary.enabled,
        wordCount: safeInteger(dictionary.wordCount, `${field}.wordCount`),
    };
}

function toDictionary(dictionary: ProtoSensitiveDictionary, field: string): SensitiveDictionary {
    return {
        name: requireString(dictionary.name, `${field}.name`),
        effectModels: dictionary.effectModels.map((model, index) =>
            requireString(model, `${field}.effectModels[${index}]`),
        ),
        enabled: dictionary.enabled,
        wordCount: dictionary.words.length,
        words: dictionary.words.map((word, index) =>
            requireString(word, `${field}.words[${index}]`),
        ),
    };
}

function requireDictionary(dictionary: ProtoSensitiveDictionary | undefined): SensitiveDictionary {
    if (!dictionary) throw new Error('Invalid response field: dictionary');
    return toDictionary(dictionary, 'dictionary');
}

export async function getSensitiveWordStatus(signal?: AbortSignal): Promise<SensitiveWordStatus> {
    const response = await callAdminRpc(() =>
        sensitiveWordClient.getSensitiveWordStatus({}, { signal }),
    );
    return {
        enabled: response.enabled,
        storeInitialized: response.storeInitialized,
        snapshot: toSnapshot(response.snapshot),
    };
}

export async function updateSensitiveWordEnabled(enabled: boolean): Promise<SensitiveWordStatus> {
    const response = await callAdminRpc(() =>
        sensitiveWordClient.updateSensitiveWordEnabled({ enabled }),
    );
    return {
        enabled: response.enabled,
        storeInitialized: response.storeInitialized,
        snapshot: toSnapshot(response.snapshot),
    };
}

export async function listSensitiveDictionaries(
    signal?: AbortSignal,
): Promise<SensitiveDictionarySummary[]> {
    const response = await callAdminRpc(() =>
        sensitiveWordClient.listSensitiveDictionaries({}, { signal }),
    );
    return response.dictionaries.map((dictionary, index) =>
        toSummary(dictionary, `dictionaries[${index}]`),
    );
}

export async function getSensitiveDictionary(
    name: string,
    signal?: AbortSignal,
): Promise<SensitiveDictionary> {
    const response = await callAdminRpc(() =>
        sensitiveWordClient.getSensitiveDictionary({ name }, { signal }),
    );
    return requireDictionary(response.dictionary);
}

export async function createSensitiveDictionary(input: {
    name: string;
    effectModels: string[];
    enabled: boolean;
    words: string[];
}): Promise<SensitiveDictionary> {
    const response = await callAdminRpc(() => sensitiveWordClient.createSensitiveDictionary(input));
    return requireDictionary(response.dictionary);
}

export async function updateSensitiveDictionaryEffectModels(
    name: string,
    effectModels: string[],
): Promise<SensitiveDictionary> {
    const response = await callAdminRpc(() =>
        sensitiveWordClient.updateSensitiveDictionaryEffectModels({ name, effectModels }),
    );
    return requireDictionary(response.dictionary);
}

export async function updateSensitiveDictionaryEnabled(
    name: string,
    enabled: boolean,
): Promise<SensitiveDictionary> {
    const response = await callAdminRpc(() =>
        sensitiveWordClient.updateSensitiveDictionaryEnabled({ name, enabled }),
    );
    return requireDictionary(response.dictionary);
}

export async function addSensitiveWords(
    name: string,
    words: string[],
): Promise<SensitiveDictionary> {
    const response = await callAdminRpc(() =>
        sensitiveWordClient.addSensitiveWords({ name, words }),
    );
    return requireDictionary(response.dictionary);
}

export async function removeSensitiveWords(
    name: string,
    words: string[],
): Promise<SensitiveDictionary> {
    const response = await callAdminRpc(() =>
        sensitiveWordClient.removeSensitiveWords({ name, words }),
    );
    return requireDictionary(response.dictionary);
}

export async function deleteSensitiveDictionary(name: string): Promise<void> {
    await callAdminRpc(() => sensitiveWordClient.deleteSensitiveDictionary({ name }));
}
