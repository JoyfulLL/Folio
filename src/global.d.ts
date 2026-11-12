declare module 'vuetify/styles';
declare module 'framework7-icons';

declare const __FOLIO_IS_PRODUCTION__: boolean;
declare const __FOLIO_VERSION__: string;
declare const __FOLIO_BUILD_UNIX_TIME__: string;
declare const __FOLIO_BUILD_COMMIT_HASH__: string;
declare const __FOLIO_LICENSE__: string;
declare const __FOLIO_CONTRIBUTORS__: ContributorInfo;
declare const __FOLIO_THIRD_PARTY_LICENSES__: LicenseInfo[];

declare interface ContributorInfo {
    code: string[];
    translators: Record<string, string[]>;
}

declare interface LicenseInfo {
    name: string;
    copyright?: string;
    url?: string;
    license?: string;
    licenseUrl?: string;
}

interface Window {
    FOLIO_SERVER_SETTINGS?: {
        [key: string]: string | number | boolean | undefined | null;
    };
}

interface Navigator {
    browserLanguage?: string;
}

interface Credential {
    rawId: ArrayBuffer;
    response: {
        clientDataJSON: ArrayBuffer;
        attestationObject: ArrayBuffer;
        userHandle: ArrayBuffer;
    };
}
