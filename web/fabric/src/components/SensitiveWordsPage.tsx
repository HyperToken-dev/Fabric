import { useEffect, useState, type FormEvent } from 'react';
import { ShieldAlert, Plus, Save, Trash2 } from 'lucide-react';
import {
    addSensitiveWords,
    createSensitiveDictionary,
    deleteSensitiveDictionary,
    getSensitiveDictionary,
    getSensitiveWordStatus,
    listSensitiveDictionaries,
    removeSensitiveWords,
    updateSensitiveDictionaryEffectModels,
    updateSensitiveDictionaryEnabled,
    updateSensitiveWordEnabled,
    type SensitiveDictionary,
    type SensitiveDictionarySummary,
    type SensitiveWordStatus,
} from '../api/sensitiveWords';
import {
    buttonClass,
    EmptyState,
    ErrorState,
    inputClass,
    LoadingCards,
    PageHeader,
    RefreshWarning,
} from './PageState';

export default function SensitiveWordsPage() {
    const [status, setStatus] = useState<SensitiveWordStatus | null>(null);
    const [dictionaries, setDictionaries] = useState<SensitiveDictionarySummary[] | null>(null);
    const [selectedName, setSelectedName] = useState('');
    const [selected, setSelected] = useState<SensitiveDictionary | null>(null);
    const [effectModelsText, setEffectModelsText] = useState('');
    const [newWordsText, setNewWordsText] = useState('');
    const [createOpen, setCreateOpen] = useState(false);
    const [newName, setNewName] = useState('');
    const [newEffectModelsText, setNewEffectModelsText] = useState('');
    const [newDictionaryWordsText, setNewDictionaryWordsText] = useState('');
    const [error, setError] = useState<string | null>(null);
    const [version, setVersion] = useState(0);
    const [saving, setSaving] = useState(false);

    useEffect(() => {
        const controller = new AbortController();
        setError(null);
        Promise.all([
            getSensitiveWordStatus(controller.signal),
            listSensitiveDictionaries(controller.signal),
        ])
            .then(([statusResult, dictionaryResult]) => {
                setStatus(statusResult);
                setDictionaries(dictionaryResult);
                setSelectedName((current) =>
                    current && dictionaryResult.some((dictionary) => dictionary.name === current)
                        ? current
                        : (dictionaryResult[0]?.name ?? ''),
                );
            })
            .catch((requestError: unknown) => {
                if (!controller.signal.aborted) {
                    setError(
                        requestError instanceof Error
                            ? requestError.message
                            : 'Unable to load sensitive words',
                    );
                }
            });
        return () => controller.abort();
    }, [version]);

    useEffect(() => {
        if (!selectedName) {
            setSelected(null);
            setEffectModelsText('');
            return;
        }
        const controller = new AbortController();
        setError(null);
        setSelected(null);
        getSensitiveDictionary(selectedName, controller.signal)
            .then((dictionary) => {
                setSelected(dictionary);
                setEffectModelsText(dictionary.effectModels.join('\n'));
            })
            .catch((requestError: unknown) => {
                if (!controller.signal.aborted) {
                    setError(
                        requestError instanceof Error
                            ? requestError.message
                            : 'Unable to load dictionary',
                    );
                }
            });
        return () => controller.abort();
    }, [selectedName, version]);

    async function toggleGlobalEnabled() {
        if (!status || saving) return;
        setSaving(true);
        setError(null);
        try {
            const next = await updateSensitiveWordEnabled(!status.enabled);
            setStatus(next);
        } catch (requestError) {
            setError(
                requestError instanceof Error
                    ? requestError.message
                    : 'Unable to update sensitive word detection',
            );
        } finally {
            setSaving(false);
        }
    }

    async function submitCreate(event: FormEvent) {
        event.preventDefault();
        if (saving) return;
        const name = newName.trim();
        if (!name) {
            setError('Dictionary name is required.');
            return;
        }
        setSaving(true);
        setError(null);
        try {
            const dictionary = await createSensitiveDictionary({
                name,
                effectModels: parseLines(newEffectModelsText),
                enabled: true,
                words: parseLines(newDictionaryWordsText),
            });
            setDictionaries((current) => [
                ...(current ?? []),
                {
                    name: dictionary.name,
                    effectModels: dictionary.effectModels,
                    enabled: dictionary.enabled,
                    wordCount: dictionary.words.length,
                },
            ]);
            setSelectedName(dictionary.name);
            setSelected(dictionary);
            setEffectModelsText(dictionary.effectModels.join('\n'));
            setNewName('');
            setNewEffectModelsText('');
            setNewDictionaryWordsText('');
            setCreateOpen(false);
            setVersion((value) => value + 1);
        } catch (requestError) {
            setError(
                requestError instanceof Error
                    ? requestError.message
                    : 'Unable to create dictionary',
            );
        } finally {
            setSaving(false);
        }
    }

    async function saveEffectModels() {
        if (!selected || saving) return;
        setSaving(true);
        setError(null);
        try {
            const dictionary = await updateSensitiveDictionaryEffectModels(
                selected.name,
                parseLines(effectModelsText),
            );
            updateSelected(dictionary);
        } catch (requestError) {
            setError(
                requestError instanceof Error
                    ? requestError.message
                    : 'Unable to update effect models',
            );
        } finally {
            setSaving(false);
        }
    }

    async function toggleDictionaryEnabled() {
        if (!selected || saving) return;
        setSaving(true);
        setError(null);
        try {
            const dictionary = await updateSensitiveDictionaryEnabled(
                selected.name,
                !selected.enabled,
            );
            updateSelected(dictionary);
        } catch (requestError) {
            setError(
                requestError instanceof Error
                    ? requestError.message
                    : 'Unable to update dictionary status',
            );
        } finally {
            setSaving(false);
        }
    }

    async function addWords() {
        if (!selected || saving) return;
        const words = parseLines(newWordsText);
        if (words.length === 0) return;
        setSaving(true);
        setError(null);
        try {
            const dictionary = await addSensitiveWords(selected.name, words);
            updateSelected(dictionary);
            setNewWordsText('');
        } catch (requestError) {
            setError(requestError instanceof Error ? requestError.message : 'Unable to add words');
        } finally {
            setSaving(false);
        }
    }

    async function removeWord(word: string) {
        if (!selected || saving) return;
        setSaving(true);
        setError(null);
        try {
            const dictionary = await removeSensitiveWords(selected.name, [word]);
            updateSelected(dictionary);
        } catch (requestError) {
            setError(
                requestError instanceof Error ? requestError.message : 'Unable to remove word',
            );
        } finally {
            setSaving(false);
        }
    }

    async function deleteDictionary() {
        if (!selected || saving) return;
        if (!window.confirm(`Delete sensitive dictionary "${selected.name}"?`)) return;
        setSaving(true);
        setError(null);
        try {
            await deleteSensitiveDictionary(selected.name);
            setDictionaries(
                (current) => current?.filter((item) => item.name !== selected.name) ?? [],
            );
            setSelectedName('');
            setSelected(null);
            setVersion((value) => value + 1);
        } catch (requestError) {
            setError(
                requestError instanceof Error
                    ? requestError.message
                    : 'Unable to delete dictionary',
            );
        } finally {
            setSaving(false);
        }
    }

    function updateSelected(dictionary: SensitiveDictionary) {
        setSelected(dictionary);
        setEffectModelsText(dictionary.effectModels.join('\n'));
        setDictionaries((current) =>
            (current ?? []).map((item) =>
                item.name === dictionary.name
                    ? {
                          name: dictionary.name,
                          effectModels: dictionary.effectModels,
                          enabled: dictionary.enabled,
                          wordCount: dictionary.words.length,
                      }
                    : item,
            ),
        );
        setVersion((value) => value + 1);
    }

    if (!status && error) {
        return (
            <div className="mx-auto max-w-7xl p-5 md:p-8">
                <ErrorState message={error} retry={() => setVersion((value) => value + 1)} />
            </div>
        );
    }

    return (
        <div className="mx-auto max-w-7xl space-y-6 p-5 md:p-8">
            <PageHeader
                eyebrow="Policy control"
                title="Fire Wall"
                description="Control detection, model scopes, and dictionary words from one place."
                action={
                    <button
                        type="button"
                        onClick={() => setCreateOpen((value) => !value)}
                        className={buttonClass}
                    >
                        <Plus size={16} /> New dictionary
                    </button>
                }
            />
            {error && status && (
                <RefreshWarning message={error} retry={() => setVersion((value) => value + 1)} />
            )}
            {!status && <LoadingCards />}
            {status && (
                <section className="rounded-3xl border border-slate-100 bg-white p-5 shadow-sm">
                    <div>
                        <div className="flex items-center justify-between gap-4">
                            <div>
                                <p className="text-sm font-semibold text-slate-900">
                                    Global detection
                                </p>
                                <p className="text-sm text-slate-500">
                                    {status.enabled
                                        ? 'Requests are checked against enabled dictionaries.'
                                        : 'Dictionaries are saved but request detection is disabled.'}
                                </p>
                            </div>
                            <button
                                type="button"
                                onClick={toggleGlobalEnabled}
                                disabled={saving}
                                className={`rounded-full px-4 py-2 text-sm font-semibold transition ${
                                    status.enabled
                                        ? 'bg-emerald-100 text-emerald-700'
                                        : 'bg-slate-100 text-slate-600'
                                }`}
                            >
                                {status.enabled ? 'Enabled' : 'Disabled'}
                            </button>
                        </div>
                    </div>
                </section>
            )}
            {createOpen && (
                <form
                    onSubmit={submitCreate}
                    className="rounded-3xl border border-emerald-100 bg-white p-5 shadow-sm"
                >
                    <div className="grid gap-4 lg:grid-cols-3">
                        <label className="text-sm font-medium text-slate-700">
                            Dictionary name
                            <input
                                className={inputClass}
                                value={newName}
                                onChange={(event) => setNewName(event.target.value)}
                                placeholder="Default dictionary"
                            />
                        </label>
                        <label className="text-sm font-medium text-slate-700">
                            Effect models
                            <textarea
                                className={`${inputClass} min-h-28`}
                                value={newEffectModelsText}
                                onChange={(event) => setNewEffectModelsText(event.target.value)}
                                placeholder="One model per line; empty means all models"
                            />
                        </label>
                        <label className="text-sm font-medium text-slate-700">
                            Initial words
                            <textarea
                                className={`${inputClass} min-h-28`}
                                value={newDictionaryWordsText}
                                onChange={(event) => setNewDictionaryWordsText(event.target.value)}
                                placeholder="One word per line"
                            />
                        </label>
                    </div>
                    <button
                        type="submit"
                        disabled={saving || !newName.trim()}
                        className={`${buttonClass} mt-4`}
                    >
                        Create dictionary
                    </button>
                </form>
            )}
            {dictionaries?.length === 0 && (
                <EmptyState
                    title="No sensitive dictionaries yet"
                    description="Create a dictionary to start adding UI-managed sensitive words."
                />
            )}
            {!!dictionaries?.length && (
                <section className="grid gap-6 lg:grid-cols-[320px_minmax(0,1fr)]">
                    <div className="space-y-3">
                        {dictionaries.map((dictionary) => (
                            <button
                                key={dictionary.name}
                                type="button"
                                onClick={() => setSelectedName(dictionary.name)}
                                className={`w-full rounded-2xl border p-4 text-left transition ${
                                    selectedName === dictionary.name
                                        ? 'border-emerald-300 bg-emerald-50 shadow-sm'
                                        : 'border-slate-100 bg-white hover:border-emerald-100'
                                }`}
                            >
                                <div className="flex items-center justify-between gap-3">
                                    <span className="font-semibold text-slate-900">
                                        {dictionary.name}
                                    </span>
                                    <span
                                        className={`rounded-full px-2.5 py-1 text-xs font-semibold ${
                                            dictionary.enabled
                                                ? 'bg-emerald-100 text-emerald-700'
                                                : 'bg-slate-100 text-slate-500'
                                        }`}
                                    >
                                        {dictionary.enabled ? 'On' : 'Off'}
                                    </span>
                                </div>
                                <p className="mt-2 text-sm text-slate-500">
                                    {dictionary.wordCount} words ·{' '}
                                    {dictionary.effectModels.length
                                        ? `${dictionary.effectModels.length} models`
                                        : 'All models'}
                                </p>
                            </button>
                        ))}
                    </div>
                    <DictionaryEditor
                        dictionary={selected}
                        effectModelsText={effectModelsText}
                        newWordsText={newWordsText}
                        saving={saving}
                        setEffectModelsText={setEffectModelsText}
                        setNewWordsText={setNewWordsText}
                        saveEffectModels={saveEffectModels}
                        toggleDictionaryEnabled={toggleDictionaryEnabled}
                        addWords={addWords}
                        removeWord={removeWord}
                        deleteDictionary={deleteDictionary}
                    />
                </section>
            )}
        </div>
    );
}

function DictionaryEditor({
    dictionary,
    effectModelsText,
    newWordsText,
    saving,
    setEffectModelsText,
    setNewWordsText,
    saveEffectModels,
    toggleDictionaryEnabled,
    addWords,
    removeWord,
    deleteDictionary,
}: {
    dictionary: SensitiveDictionary | null;
    effectModelsText: string;
    newWordsText: string;
    saving: boolean;
    setEffectModelsText: (value: string) => void;
    setNewWordsText: (value: string) => void;
    saveEffectModels: () => void;
    toggleDictionaryEnabled: () => void;
    addWords: () => void;
    removeWord: (word: string) => void;
    deleteDictionary: () => void;
}) {
    if (!dictionary) {
        return (
            <div className="rounded-3xl border border-slate-100 bg-white p-8 text-center text-slate-500 shadow-sm">
                <ShieldAlert className="mx-auto mb-3 text-slate-400" />
                Select a dictionary to edit its scope and words.
            </div>
        );
    }

    return (
        <div className="space-y-5 rounded-3xl border border-slate-100 bg-white p-5 shadow-sm">
            <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                <div>
                    <h2 className="text-xl font-bold text-slate-900">{dictionary.name}</h2>
                    <p className="text-sm text-slate-500">
                        {dictionary.effectModels.length
                            ? dictionary.effectModels.join(', ')
                            : 'Applies to all models'}
                    </p>
                </div>
                <div className="flex flex-wrap gap-2">
                    <button
                        type="button"
                        onClick={toggleDictionaryEnabled}
                        disabled={saving}
                        className="rounded-xl border border-slate-200 px-3 py-2 text-sm font-semibold text-slate-700"
                    >
                        {dictionary.enabled ? 'Disable' : 'Enable'}
                    </button>
                    <button
                        type="button"
                        onClick={deleteDictionary}
                        disabled={saving}
                        className="inline-flex items-center gap-2 rounded-xl border border-rose-200 px-3 py-2 text-sm font-semibold text-rose-600"
                    >
                        <Trash2 size={15} /> Delete
                    </button>
                </div>
            </div>
            <label className="block text-sm font-medium text-slate-700">
                Effect models
                <textarea
                    className={`${inputClass} mt-1 min-h-28`}
                    value={effectModelsText}
                    onChange={(event) => setEffectModelsText(event.target.value)}
                    placeholder="One model per line; empty means all models"
                />
            </label>
            <button
                type="button"
                onClick={saveEffectModels}
                disabled={saving}
                className="inline-flex items-center gap-2 rounded-xl bg-slate-900 px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-50"
            >
                <Save size={15} /> Save models
            </button>
            <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_260px]">
                <div>
                    <p className="mb-2 text-sm font-semibold text-slate-900">Words</p>
                    <div className="max-h-96 space-y-2 overflow-y-auto rounded-2xl border border-slate-100 bg-slate-50 p-3">
                        {dictionary.words.length === 0 && (
                            <p className="py-6 text-center text-sm text-slate-500">
                                No words in this dictionary.
                            </p>
                        )}
                        {dictionary.words.map((word) => (
                            <div
                                key={word}
                                className="flex items-center justify-between gap-3 rounded-xl bg-white px-3 py-2 text-sm text-slate-700"
                            >
                                <span className="break-all">{word}</span>
                                <button
                                    type="button"
                                    onClick={() => removeWord(word)}
                                    disabled={saving}
                                    className="text-rose-600 disabled:opacity-50"
                                >
                                    Remove
                                </button>
                            </div>
                        ))}
                    </div>
                </div>
                <div>
                    <label className="block text-sm font-medium text-slate-700">
                        Add words
                        <textarea
                            className={`${inputClass} mt-1 min-h-40`}
                            value={newWordsText}
                            onChange={(event) => setNewWordsText(event.target.value)}
                            placeholder="One word per line"
                        />
                    </label>
                    <button
                        type="button"
                        onClick={addWords}
                        disabled={saving || parseLines(newWordsText).length === 0}
                        className={`${buttonClass} mt-3 w-full`}
                    >
                        Add words
                    </button>
                </div>
            </div>
        </div>
    );
}

function parseLines(value: string): string[] {
    return Array.from(
        new Set(
            value
                .split(/\r?\n/)
                .map((line) => line.trim())
                .filter(Boolean),
        ),
    );
}
