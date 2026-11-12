export function getLicense(): string {
    return __FOLIO_LICENSE__;
}

export function getThirdPartyLicenses(): LicenseInfo[] {
    return __FOLIO_THIRD_PARTY_LICENSES__ || [];
}
