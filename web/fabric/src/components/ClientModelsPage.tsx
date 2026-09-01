import { useEffect, useState } from 'react';
import { listClientModels, MODEL_TYPES, type ClientModel } from '../api/models';
import { useI18n } from '../i18n';
import { EmptyState, ErrorState, LoadingCards, PageHeader, RefreshWarning } from './PageState';

export default function ClientModelsPage() {
    const { t } = useI18n();
    const [models, setModels] = useState<ClientModel[] | null>(null);
    const [error, setError] = useState<string | null>(null);
    const [version, setVersion] = useState(0);

    useEffect(() => {
        const controller = new AbortController();
        setError(null);
        setModels(null);
        listClientModels('', controller.signal)
            .then(setModels)
            .catch((requestError: unknown) => {
                if (!controller.signal.aborted) {
                    setError(
                        requestError instanceof Error
                            ? requestError.message
                            : 'i18n:models.loadModelsError',
                    );
                }
            });
        return () => controller.abort();
    }, [version]);

    function modelTypeLabel(modelType: number): string {
        return (
            MODEL_TYPES[modelType as keyof typeof MODEL_TYPES] ??
            t('common.unknown', { value: modelType })
        );
    }

    if (!models && error) {
        return (
            <div className="mx-auto max-w-7xl p-5 md:p-8">
                <ErrorState message={error} retry={() => setVersion((value) => value + 1)} />
            </div>
        );
    }

    return (
        <div className="mx-auto max-w-7xl space-y-6 p-5 md:p-8">
            <PageHeader
                eyebrow={t('models.eyebrow')}
                title={t('models.title')}
                description={t('models.description')}
            />
            {error && models && (
                <RefreshWarning message={error} retry={() => setVersion((value) => value + 1)} />
            )}
            {!models && <LoadingCards />}
            {models?.length === 0 && (
                <EmptyState
                    title={t('models.emptyTitle')}
                    description={t('models.emptyDescription')}
                />
            )}
            {!!models?.length && (
                <div className="grid gap-4 lg:grid-cols-2">
                    {models.map((model) => (
                        <article
                            key={`${model.channelName}:${model.modelName}`}
                            className="rounded-2xl border border-slate-100 bg-white p-5 shadow-sm"
                        >
                            <h2 className="font-bold text-slate-900">{model.modelName}</h2>
                            <div className="mt-4 flex flex-wrap gap-2 text-xs font-semibold">
                                <span className="rounded-full bg-emerald-50 px-3 py-1 text-emerald-700">
                                    {model.channelName}
                                </span>
                                <span className="rounded-full bg-slate-100 px-3 py-1 text-slate-600">
                                    {modelTypeLabel(model.modelType)}
                                </span>
                            </div>
                        </article>
                    ))}
                </div>
            )}
        </div>
    );
}
