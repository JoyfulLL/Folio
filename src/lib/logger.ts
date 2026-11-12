import { isEnableDebug } from './settings.ts';

function logDebug(msg: string, obj?: unknown): void {
    if (isEnableDebug()) {
        if (obj) {
            console.debug('[Folio Debug] ' + msg, obj);
        } else {
            console.debug('[Folio Debug] ' + msg);
        }
    }
}

function logInfo(msg: string, obj?: unknown): void {
    if (obj) {
        console.info('[Folio Info] ' + msg, obj);
    } else {
        console.info('[Folio Info] ' + msg);
    }
}

function logWarn(msg: string, obj?: unknown): void {
    if (obj) {
        console.warn('[Folio Warn] ' + msg, obj);
    } else {
        console.warn('[Folio Warn] ' + msg);
    }
}

function logError(msg: string, obj?: unknown): void {
    if (obj) {
        console.error('[Folio Error] ' + msg, obj);
    } else {
        console.error('[Folio Error] ' + msg);
    }
}

export default {
    debug: logDebug,
    info: logInfo,
    warn: logWarn,
    error: logError
};
