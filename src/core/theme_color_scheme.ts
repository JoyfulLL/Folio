export enum ThemeColorSchemeType {
    Amber = 'amber',
    Terracotta = 'terracotta',
    Indigo = 'indigo',
    Cobalt = 'cobalt',
    Violet = 'violet',
    Slate = 'slate'
}

export interface ThemeColorSchemeNeutralColors {
    readonly background: string;
    readonly foreground: string;
    readonly surface: string;
    readonly surfaceMuted: string;
    readonly surfaceRaised: string;
    readonly authBackground: string;
    readonly border: string;
}

export interface ThemeColorScheme {
    readonly type: ThemeColorSchemeType;
    readonly name: string;
    readonly primary: string;
    readonly primaryDarken1: string;
    readonly primaryRgb: string;
    readonly onPrimary: string;
    readonly light: ThemeColorSchemeNeutralColors;
    readonly dark: ThemeColorSchemeNeutralColors;
}

export const DEFAULT_THEME_COLOR_SCHEME: ThemeColorSchemeType = ThemeColorSchemeType.Amber;

export const ALL_THEME_COLOR_SCHEMES: readonly ThemeColorScheme[] = [
    {
        type: ThemeColorSchemeType.Amber,
        name: 'Amber',
        primary: '#c67e48',
        primaryDarken1: '#b67443',
        primaryRgb: '198, 126, 72',
        onPrimary: '#ffffff',
        light: {
            background: '#faf8f4',
            foreground: '#413935',
            surface: '#ffffff',
            surfaceMuted: '#f8f7f5',
            surfaceRaised: '#ffffff',
            authBackground: '#faf8f4',
            border: '#362f2a'
        },
        dark: {
            background: '#060504',
            foreground: '#fcf0e3',
            surface: '#1a1a1a',
            surfaceMuted: '#23201d',
            surfaceRaised: '#272421',
            authBackground: '#202122',
            border: '#ece2d9'
        }
    },
    {
        type: ThemeColorSchemeType.Terracotta,
        name: 'Terracotta',
        primary: '#c25c38',
        primaryDarken1: '#ad502f',
        primaryRgb: '194, 92, 56',
        onPrimary: '#ffffff',
        light: {
            background: '#fbf7f4',
            foreground: '#3c332e',
            surface: '#ffffff',
            surfaceMuted: '#f7f2ee',
            surfaceRaised: '#ffffff',
            authBackground: '#fbf7f4',
            border: '#4a3a30'
        },
        dark: {
            background: '#080605',
            foreground: '#faeee7',
            surface: '#1c1917',
            surfaceMuted: '#26211e',
            surfaceRaised: '#2b2521',
            authBackground: '#221f1d',
            border: '#e8dcd2'
        }
    },
    {
        type: ThemeColorSchemeType.Indigo,
        name: 'Indigo',
        primary: '#4f5bb8',
        primaryDarken1: '#43509f',
        primaryRgb: '79, 91, 184',
        onPrimary: '#ffffff',
        light: {
            background: '#f6f7fc',
            foreground: '#2e3348',
            surface: '#ffffff',
            surfaceMuted: '#f0f2f9',
            surfaceRaised: '#ffffff',
            authBackground: '#f6f7fc',
            border: '#333a52'
        },
        dark: {
            background: '#05060b',
            foreground: '#e9ebf7',
            surface: '#191b26',
            surfaceMuted: '#222535',
            surfaceRaised: '#282b3c',
            authBackground: '#1d202d',
            border: '#dfe3f0'
        }
    },
    {
        type: ThemeColorSchemeType.Cobalt,
        name: 'Cobalt',
        primary: '#2f6bb0',
        primaryDarken1: '#295e9c',
        primaryRgb: '47, 107, 176',
        onPrimary: '#ffffff',
        light: {
            background: '#f5f8fc',
            foreground: '#2b3644',
            surface: '#ffffff',
            surfaceMuted: '#eef3f9',
            surfaceRaised: '#ffffff',
            authBackground: '#f5f8fc',
            border: '#2f3c4d'
        },
        dark: {
            background: '#04070b',
            foreground: '#e7eff9',
            surface: '#171c23',
            surfaceMuted: '#1f2731',
            surfaceRaised: '#252f3b',
            authBackground: '#1b212a',
            border: '#dce6f1'
        }
    },
    {
        type: ThemeColorSchemeType.Violet,
        name: 'Violet',
        primary: '#7b5ea7',
        primaryDarken1: '#6b5193',
        primaryRgb: '123, 94, 167',
        onPrimary: '#ffffff',
        light: {
            background: '#f8f6fb',
            foreground: '#383044',
            surface: '#ffffff',
            surfaceMuted: '#f2eef7',
            surfaceRaised: '#ffffff',
            authBackground: '#f8f6fb',
            border: '#41354f'
        },
        dark: {
            background: '#07050b',
            foreground: '#f0eaf8',
            surface: '#1b1822',
            surfaceMuted: '#24202d',
            surfaceRaised: '#2a2536',
            authBackground: '#201c29',
            border: '#e5dcf0'
        }
    },
    {
        type: ThemeColorSchemeType.Slate,
        name: 'Slate',
        primary: '#566573',
        primaryDarken1: '#495662',
        primaryRgb: '86, 101, 115',
        onPrimary: '#ffffff',
        light: {
            background: '#f6f8fa',
            foreground: '#303842',
            surface: '#ffffff',
            surfaceMuted: '#eff3f6',
            surfaceRaised: '#ffffff',
            authBackground: '#f6f8fa',
            border: '#37424e'
        },
        dark: {
            background: '#050709',
            foreground: '#e9eef3',
            surface: '#181c21',
            surfaceMuted: '#20262d',
            surfaceRaised: '#262d35',
            authBackground: '#1c2228',
            border: '#dfe6ed'
        }
    }
];

export function getThemeColorScheme(type: string): ThemeColorScheme {
    for (const scheme of ALL_THEME_COLOR_SCHEMES) {
        if (scheme.type === type) {
            return scheme;
        }
    }

    return getDefaultThemeColorScheme();
}

export function getDefaultThemeColorScheme(): ThemeColorScheme {
    return getThemeColorScheme(DEFAULT_THEME_COLOR_SCHEME);
}

export function getThemeColorSchemeChartBaseColor(type: string, dark: boolean): string {
    const scheme = getThemeColorScheme(type);

    return dark ? scheme.dark.surface : scheme.light.background;
}
