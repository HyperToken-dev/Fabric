import {
    createContext,
    useCallback,
    useContext,
    useEffect,
    useMemo,
    useState,
    type ReactNode,
} from 'react';
import {
    defaultLanguage,
    languageStorageKey,
    messages,
    supportedLanguages,
    type Language,
} from './messages';

type TranslationValues = Record<string, string | number>;

type I18nContextValue = {
    language: Language;
    setLanguage: (language: Language) => void;
    t: (key: string, values?: TranslationValues) => string;
    formatNumber: (value: number) => string;
    formatCompactNumber: (value: number) => string;
    formatDate: (value: string | number | Date) => string;
    formatErrorMessage: (message: string) => string;
};

const I18nContext = createContext<I18nContextValue | null>(null);
const translationErrorPrefix = 'i18n:';

function isLanguage(value: string | null): value is Language {
    return supportedLanguages.includes(value as Language);
}

function initialLanguage(): Language {
    if (typeof window === 'undefined') return defaultLanguage;
    const stored = window.localStorage.getItem(languageStorageKey);
    return isLanguage(stored) ? stored : defaultLanguage;
}

function interpolate(message: string, values?: TranslationValues): string {
    if (!values) return message;
    return message.replace(/\{(\w+)\}/g, (match, key) =>
        Object.prototype.hasOwnProperty.call(values, key) ? String(values[key]) : match,
    );
}

export function I18nProvider({ children }: { children: ReactNode }) {
    const [language, setLanguageState] = useState<Language>(initialLanguage);

    useEffect(() => {
        window.localStorage.setItem(languageStorageKey, language);
        document.documentElement.lang = language;
    }, [language]);

    const setLanguage = useCallback(function setLanguage(nextLanguage: Language) {
        setLanguageState(nextLanguage);
    }, []);

    const t = useCallback(
        function t(key: string, values?: TranslationValues): string {
            const message = messages[language][key] ?? messages[defaultLanguage][key] ?? key;
            return interpolate(message, values);
        },
        [language],
    );

    const formatNumber = useCallback(
        function formatNumber(value: number): string {
            return new Intl.NumberFormat(language).format(value);
        },
        [language],
    );

    const formatCompactNumber = useCallback(
        function formatCompactNumber(value: number): string {
            return new Intl.NumberFormat(language, {
                notation: 'compact',
                maximumFractionDigits: 1,
            }).format(value);
        },
        [language],
    );

    const formatDate = useCallback(
        function formatDate(value: string | number | Date): string {
            return new Date(value).toLocaleString(language);
        },
        [language],
    );

    const formatErrorMessage = useCallback(
        function formatErrorMessage(message: string): string {
            if (message.startsWith(translationErrorPrefix)) {
                return t(message.slice(translationErrorPrefix.length));
            }
            if (message === messages[defaultLanguage]['rpc.unreachable'])
                return t('rpc.unreachable');
            if (message === messages[defaultLanguage]['rpc.failed']) return t('rpc.failed');
            return message;
        },
        [t],
    );

    const value = useMemo(
        () => ({
            language,
            setLanguage,
            t,
            formatNumber,
            formatCompactNumber,
            formatDate,
            formatErrorMessage,
        }),
        [
            language,
            setLanguage,
            t,
            formatNumber,
            formatCompactNumber,
            formatDate,
            formatErrorMessage,
        ],
    );

    return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n(): I18nContextValue {
    const context = useContext(I18nContext);
    if (!context) throw new Error('useI18n must be used within I18nProvider');
    return context;
}

export type { Language };
