import { clientsClaim } from 'workbox-core';
import type {
    WorkboxPlugin,
    CacheDidUpdateCallbackParam,
    CacheKeyWillBeUsedCallbackParam,
    CacheWillUpdateCallbackParam,
    CachedResponseWillBeUsedCallbackParam
} from 'workbox-core/types';
import { cleanupOutdatedCaches, precacheAndRoute } from 'workbox-precaching';
import { registerRoute } from 'workbox-routing';
import { CacheFirst, NetworkFirst, StaleWhileRevalidate } from 'workbox-strategies';

interface CacheTimestampEntry {
    request: Request;
    time: number;
}

class DynamicExpirationPlugin implements WorkboxPlugin {
    private static readonly SW_CACHE_TIME_HEADER: string = 'folio-sw-cache-time';
    private maxEntries: number;
    private maxAgeMilliseconds: number;
    private cleaningCache: boolean = false;

    constructor(maxEntries: number, maxAgeMilliseconds: number) {
        this.maxEntries = maxEntries;
        this.maxAgeMilliseconds = maxAgeMilliseconds;
    }

    public getMaxEntries(): number {
        return this.maxEntries;
    }

    public setMaxEntries(maxEntries: number): void {
        this.maxEntries = maxEntries;
    }

    public getMaxAgeMilliseconds(): number {
        return this.maxAgeMilliseconds;
    }

    public setMaxAgeMilliseconds(maxAgeMilliseconds: number): void {
        this.maxAgeMilliseconds = maxAgeMilliseconds;
    }

    public async cacheWillUpdate(param: CacheWillUpdateCallbackParam): Promise<Response | null> {
        const response = param.response;

        if (!response || response.status < 200 || response.status >= 300 || response.type === 'opaque') {
            return null;
        }

        const body = await response.blob();
        const headers = new Headers(response.headers);
        headers.set(DynamicExpirationPlugin.SW_CACHE_TIME_HEADER, Date.now().toString());

        return new Response(body, {
            status: response.status,
            statusText: response.statusText,
            headers: headers
        });
    }

    public async cachedResponseWillBeUsed(param: CachedResponseWillBeUsedCallbackParam): Promise<Response | null> {
        const cachedResponse = param.cachedResponse;

        if (!cachedResponse) {
            return null;
        }

        const cacheTime: string | null = cachedResponse.headers.get(DynamicExpirationPlugin.SW_CACHE_TIME_HEADER);

        if (!cacheTime) {
            return cachedResponse;
        }

        const age: number = Date.now() - Number(cacheTime);

        if (this.maxAgeMilliseconds > 0 && age >= this.maxAgeMilliseconds) {
            if (param.cacheName) {
                const cache = await caches.open(param.cacheName);
                await cache.delete(param.request);
            }

            return null;
        }

        return cachedResponse;
    }

    public async cacheDidUpdate(param: CacheDidUpdateCallbackParam): Promise<void> {
        if (this.cleaningCache || !param.cacheName) {
            return;
        }

        this.cleaningCache = true;

        const cache: Cache = await caches.open(param.cacheName);
        const requests: readonly Request[] = await cache.keys();

        if (requests.length <= this.maxEntries) {
            this.cleaningCache = false;
            return;
        }

        const entries: CacheTimestampEntry[] = [];

        for (const request of requests) {
            const response: Response | undefined = await cache.match(request);

            if (!response) {
                continue;
            }

            const cacheTime: string | null = response.headers.get(DynamicExpirationPlugin.SW_CACHE_TIME_HEADER);
            let time: number = cacheTime ? Number(cacheTime) : 0;

            if (Number.isFinite(time)) {
                const age: number = Date.now() - time;

                if (this.maxAgeMilliseconds > 0 && age >= this.maxAgeMilliseconds) {
                    await cache.delete(request);
                    continue;
                }
            } else {
                time = 0;
            }

            entries.push({
                request: request,
                time: time
            });
        }

        if (entries.length <= this.maxEntries) {
            this.cleaningCache = false;
            return;
        }

        entries.sort((a, b) => a.time - b.time);

        const removeCount: number = entries.length - this.maxEntries;

        for (let i = 0; i < removeCount; i++) {
            const entry = entries[i];

            if (entry && entry.request) {
                await cache.delete(entry.request);
            }
        }

        this.cleaningCache = false;
    }
}

class AuthenticatedImageRequestStripTokenPlugin implements WorkboxPlugin {
    public async cacheKeyWillBeUsed(param: CacheKeyWillBeUsedCallbackParam): Promise<Request> {
        const url = new URL(param.request.url);

        if (url.searchParams.has('token')) {
            url.searchParams.delete('token');
            const request: Request = param.request.mode === 'navigate' ? new Request(param.request, { mode: 'same-origin' }) : param.request;
            return new Request(url.href, request);
        }

        return param.request;
    }
}

interface MapCacheConfig {
    enabled: boolean;
    patterns: RegExp[];
    mapDataRequestStripTokenPlugin: AuthenticatedImageRequestStripTokenPlugin;
    expirationPlugin: DynamicExpirationPlugin;
}

interface TransactionPictureCacheConfig extends AuthenticatedImageCacheConfig {
    enabled: boolean;
    expirationPlugin: DynamicExpirationPlugin;
}

interface AuthenticatedImageCacheConfig {
    imageRequestStripTokenPlugin: AuthenticatedImageRequestStripTokenPlugin;
}

declare const self: ServiceWorkerGlobalScope;

const SW_ASSETS_CACHE_NAME: string = 'folio-assets-cache';
const SW_CODE_CACHE_NAME: string = 'folio-code-cache';
const SW_MAP_CACHE_NAME: string = 'folio-map-cache';
const SW_USER_AVATAR_CACHE_NAME: string = 'folio-user-avatar-cache';
const SW_TRANSACTION_PICTURE_CACHE_NAME: string = 'folio-transaction-picture-cache';
const SW_CUSTOM_ICON_CACHE_NAME: string = 'folio-custom-icon-cache';
const SW_SHARE_CACHE_NAME: string = 'folio-share-cache';
const SW_SHARE_IMAGE_URL_PATHNAME: string = '__share__image__';
const SW_SHARE_IMAGE_PARAM_NAME: string = 'image';

const SW_MESSAGE_TYPE_UPDATE_MAP_CACHE_CONFIG: string = 'UPDATE_MAP_CACHE_CONFIG';
const SW_MESSAGE_TYPE_UPDATE_MAP_CACHE_CONFIG_RESPONSE: string = 'UPDATE_MAP_CACHE_CONFIG_RESPONSE';

const SW_MESSAGE_TYPE_UPDATE_TRANSACTION_PICTURE_CACHE_CONFIG: string = 'UPDATE_TRANSACTION_PICTURE_CACHE_CONFIG';
const SW_MESSAGE_TYPE_UPDATE_TRANSACTION_PICTURE_CACHE_CONFIG_RESPONSE: string = 'UPDATE_TRANSACTION_PICTURE_CACHE_CONFIG_RESPONSE';

const DEFAULT_TRANSACTION_PICTURE_CACHE_MAX_ENTRIES: number = 1000;
const DEFAULT_TRANSACTION_PICTURE_CACHE_MAX_AGE_MILLISECONDS: number = 30 * 24 * 60 * 60 * 1000;

const DEFAULT_MAP_CACHE_MAX_ENTRIES: number = 1000;
const DEFAULT_MAP_CACHE_MAX_AGE_MILLISECONDS: number = 30 * 24 * 60 * 60 * 1000;

const DEFAULT_CODE_CACHE_MAX_ENTRIES: number = 200;
const DEFAULT_CODE_CACHE_MAX_AGE_MILLISECONDS: number = 30 * 24 * 60 * 60 * 1000;
const DEFAULT_CODE_CACHE_NETWORK_TIMEOUT_SECONDS: number = 3;

const mapCacheConfig: MapCacheConfig = {
    enabled: false,
    patterns: [],
    mapDataRequestStripTokenPlugin: new AuthenticatedImageRequestStripTokenPlugin(),
    expirationPlugin: new DynamicExpirationPlugin(DEFAULT_MAP_CACHE_MAX_ENTRIES, DEFAULT_MAP_CACHE_MAX_AGE_MILLISECONDS)
};

const userAvatarCacheConfig: AuthenticatedImageCacheConfig = {
    imageRequestStripTokenPlugin: new AuthenticatedImageRequestStripTokenPlugin()
};

const transactionPictureCacheConfig: TransactionPictureCacheConfig = {
    enabled: true,
    imageRequestStripTokenPlugin: new AuthenticatedImageRequestStripTokenPlugin(),
    expirationPlugin: new DynamicExpirationPlugin(DEFAULT_TRANSACTION_PICTURE_CACHE_MAX_ENTRIES, DEFAULT_TRANSACTION_PICTURE_CACHE_MAX_AGE_MILLISECONDS)
};

const customIconCacheConfig: AuthenticatedImageCacheConfig = {
    imageRequestStripTokenPlugin: new AuthenticatedImageRequestStripTokenPlugin()
};

/** Evicts stale HTML/CSS/JS from the code cache by age and count, replacing the old "delete the whole cache on activate" approach. See the activate handler below. **/
const codeCacheExpirationPlugin = new DynamicExpirationPlugin(DEFAULT_CODE_CACHE_MAX_ENTRIES, DEFAULT_CODE_CACHE_MAX_AGE_MILLISECONDS);

self.skipWaiting();
clientsClaim();
precacheAndRoute(self.__WB_MANIFEST);
cleanupOutdatedCaches();

/**
 * HTML/CSS/JS are served from a runtime code cache with a fixed name.
 *
 * The previous implementation deleted this entire cache on every activate. Combined with
 * skipWaiting()/clientsClaim() that meant a freshly activated worker could wipe the cache
 * while the *current* page was still fetching hashed assets referenced by its HTML. The
 * browser then requested old, already-removed js/css chunk names -> 404 -> blank screen.
 * That race is exactly what made a fresh deploy white-screen only sometimes.
 *
 * New approach: never wipe the code cache on activate. Hashed asset names are immutable, so
 * keeping old entries is safe; stale entries are reclaimed gradually by codeCacheExpirationPlugin
 * (age + count bound) instead of all at once. Document navigations use NetworkFirst with a
 * short timeout so an offline/slow server falls back to cache quickly instead of hanging.
 * cleanupOutdatedCaches() above still handles genuine old precache bundles.
 **/
self.addEventListener('activate', (event: ExtendableEvent) => {
    event.waitUntil(Promise.resolve());
});

/** Document navigations (including standalone PWA start_url './') revalidate against the network with a short timeout, then fall back to cache. The timeout prevents a slow/unreachable server from leaving the page blank. **/
registerRoute(
    ({ request }) => request.mode === 'navigate',
    new NetworkFirst({
        cacheName: SW_CODE_CACHE_NAME,
        networkTimeoutSeconds: DEFAULT_CODE_CACHE_NETWORK_TIMEOUT_SECONDS,
        plugins: [codeCacheExpirationPlugin]
    })
);

registerRoute(
    /.*\/img\/desktop\/.*\.(png|jpg|jpeg|gif|tiff|bmp|svg)/,
    new StaleWhileRevalidate({
        cacheName: SW_ASSETS_CACHE_NAME,
    })
);

registerRoute(
    /.*\/fonts\/.*\.(eot|ttf|svg|woff)/,
    new CacheFirst({
        cacheName: SW_ASSETS_CACHE_NAME,
    })
);

registerRoute(
    /.*\/(mobile|mobile\/|desktop|desktop\/)$/,
    new NetworkFirst({
        cacheName: SW_CODE_CACHE_NAME,
        networkTimeoutSeconds: DEFAULT_CODE_CACHE_NETWORK_TIMEOUT_SECONDS,
        plugins: [codeCacheExpirationPlugin]
    })
);

registerRoute(
    /.*\/(mobile|mobile\/)#!\//,
    new NetworkFirst({
        cacheName: SW_CODE_CACHE_NAME,
        networkTimeoutSeconds: DEFAULT_CODE_CACHE_NETWORK_TIMEOUT_SECONDS,
        plugins: [codeCacheExpirationPlugin]
    })
);

registerRoute(
    /.*\/(desktop|desktop\/)#\//,
    new NetworkFirst({
        cacheName: SW_CODE_CACHE_NAME,
        networkTimeoutSeconds: DEFAULT_CODE_CACHE_NETWORK_TIMEOUT_SECONDS,
        plugins: [codeCacheExpirationPlugin]
    })
);

registerRoute(
    /.*\/(index\.html|mobile\.html|desktop\.html)/,
    new NetworkFirst({
        cacheName: SW_CODE_CACHE_NAME,
        networkTimeoutSeconds: DEFAULT_CODE_CACHE_NETWORK_TIMEOUT_SECONDS,
        plugins: [codeCacheExpirationPlugin]
    })
);

registerRoute(
    /.*\/css\/.*\.css/,
    new CacheFirst({
        cacheName: SW_CODE_CACHE_NAME,
        plugins: [codeCacheExpirationPlugin]
    })
);

registerRoute(
    /.*\/js\/.*\.js/,
    new CacheFirst({
        cacheName: SW_CODE_CACHE_NAME,
        plugins: [codeCacheExpirationPlugin]
    })
);

registerRoute(
    ({ url }) => {
        if (!mapCacheConfig.enabled || mapCacheConfig.patterns.length < 1) {
            return false;
        }

        for (const pattern of mapCacheConfig.patterns) {
            if (pattern.test && pattern.test(url.href)) {
                return true;
            }
        }

        return false;
    },
    new CacheFirst({
        cacheName: SW_MAP_CACHE_NAME,
        plugins: [
            mapCacheConfig.mapDataRequestStripTokenPlugin,
            mapCacheConfig.expirationPlugin
        ]
    })
);

registerRoute(
    /.*\/avatar\/\d+\.[a-zA-Z0-9]+/,
    new StaleWhileRevalidate({
        cacheName: SW_USER_AVATAR_CACHE_NAME,
        plugins: [
            userAvatarCacheConfig.imageRequestStripTokenPlugin
        ]
    })
);

registerRoute(
    ({ url }) => transactionPictureCacheConfig.enabled && /.*\/pictures\/\d+\.[a-zA-Z0-9]+/.test(url.href),
    new CacheFirst({
        cacheName: SW_TRANSACTION_PICTURE_CACHE_NAME,
        plugins: [
            transactionPictureCacheConfig.imageRequestStripTokenPlugin,
            transactionPictureCacheConfig.expirationPlugin
        ]
    })
);

registerRoute(
    /.*\/icons\/\d+\.png/,
    new CacheFirst({
        cacheName: SW_CUSTOM_ICON_CACHE_NAME,
        plugins: [
            customIconCacheConfig.imageRequestStripTokenPlugin
        ]
    })
);

self.addEventListener('fetch', (event: FetchEvent) => {
    const request: Request = event.request;

    if (request.method !== 'POST' || !request.url.endsWith(SW_SHARE_IMAGE_URL_PATHNAME)) {
        return;
    }

    event.respondWith((async (): Promise<Response> => {
        let redirectUrl = request.url;
        const lastShareIndex = redirectUrl.lastIndexOf(SW_SHARE_IMAGE_URL_PATHNAME);
        redirectUrl = redirectUrl.substring(0, lastShareIndex);

        try {
            const formData = await request.formData();
            const image = formData.get(SW_SHARE_IMAGE_PARAM_NAME);

            if (image instanceof Blob) {
                const cache: Cache = await caches.open(SW_SHARE_CACHE_NAME);
                const response = new Response(image, {
                    headers: {
                        'Content-Type': image.type
                    }
                });

                const putPromise = cache.put(SW_SHARE_CACHE_NAME, response.clone());
                event.waitUntil(putPromise);
                await putPromise;
            }

            return Response.redirect(redirectUrl, 303);
        } catch (ex) {
            console.error('failed to handle share image upload in service worker', ex);
            return Response.redirect(redirectUrl, 303);
        }
    })());
});

self.addEventListener('message', (event: ExtendableMessageEvent) => {
    try {
        if (event.data && event.data.type === SW_MESSAGE_TYPE_UPDATE_MAP_CACHE_CONFIG && 'payload' in event.data) {
            mapCacheConfig.enabled = !!event.data.payload['enabled'];
            mapCacheConfig.patterns = [];
            mapCacheConfig.expirationPlugin.setMaxEntries(event.data.payload['maxEntries'] ?? DEFAULT_MAP_CACHE_MAX_ENTRIES);
            mapCacheConfig.expirationPlugin.setMaxAgeMilliseconds(event.data.payload['maxAgeMilliseconds'] ?? DEFAULT_MAP_CACHE_MAX_AGE_MILLISECONDS);

            if (event.data.payload['patterns'] && Array.isArray(event.data.payload['patterns'])) {
                for (const pattern of event.data.payload['patterns']) {
                    if (pattern) {
                        mapCacheConfig.patterns.push(new RegExp(pattern as string));
                    }
                }
            }

            if (event.ports && event.ports[0] && typeof event.ports[0].postMessage === 'function') {
                event.ports[0].postMessage({
                    type: SW_MESSAGE_TYPE_UPDATE_MAP_CACHE_CONFIG_RESPONSE,
                    payload: {
                        enabled: mapCacheConfig.enabled,
                        patterns: event.data.payload['patterns'],
                        maxEntries: mapCacheConfig.expirationPlugin.getMaxEntries(),
                        maxAgeMilliseconds: mapCacheConfig.expirationPlugin.getMaxAgeMilliseconds()
                    }
                });
            }
        } else if (event.data && event.data.type === SW_MESSAGE_TYPE_UPDATE_TRANSACTION_PICTURE_CACHE_CONFIG && 'payload' in event.data) {
            transactionPictureCacheConfig.enabled = !!event.data.payload['enabled'];
            transactionPictureCacheConfig.expirationPlugin.setMaxEntries(event.data.payload['maxEntries'] ?? DEFAULT_TRANSACTION_PICTURE_CACHE_MAX_ENTRIES);
            transactionPictureCacheConfig.expirationPlugin.setMaxAgeMilliseconds(event.data.payload['maxAgeMilliseconds'] ?? DEFAULT_TRANSACTION_PICTURE_CACHE_MAX_AGE_MILLISECONDS);

            if (event.ports && event.ports[0] && typeof event.ports[0].postMessage === 'function') {
                event.ports[0].postMessage({
                    type: SW_MESSAGE_TYPE_UPDATE_TRANSACTION_PICTURE_CACHE_CONFIG_RESPONSE,
                    payload: {
                        enabled: transactionPictureCacheConfig.enabled,
                        maxEntries: transactionPictureCacheConfig.expirationPlugin.getMaxEntries(),
                        maxAgeMilliseconds: transactionPictureCacheConfig.expirationPlugin.getMaxAgeMilliseconds()
                    }
                });
            }
        }
    } catch (ex) {
        console.error('failed to process message in service worker', ex);
    }
});
