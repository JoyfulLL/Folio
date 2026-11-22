import type { ThemeInstance } from 'vuetify';

import { ThemeType } from '@/core/theme.ts';
import { getThemeColorScheme } from '@/core/theme_color_scheme.ts';

export function applyThemeColorSchemeCssVariables(schemeType: string): void {
    const scheme = getThemeColorScheme(schemeType);
    const htmlElement = document.querySelector('html');

    if (!htmlElement) {
        return;
    }

    htmlElement.style.setProperty('--ebk-primary-color', scheme.primaryRgb);
}

export function applyVuetifyThemeColorScheme(theme: ThemeInstance, schemeType: string): void {
    const scheme = getThemeColorScheme(schemeType);

    for (const themeName of [ThemeType.Light, ThemeType.Dark]) {
        const themeDefinition = theme.themes.value[themeName];

        if (!themeDefinition) {
            continue;
        }

        const neutral = themeName === ThemeType.Dark ? scheme.dark : scheme.light;

        themeDefinition.colors['primary'] = scheme.primary;
        themeDefinition.colors['primary-darken-1'] = scheme.primaryDarken1;
        themeDefinition.colors['on-primary'] = scheme.onPrimary;

        themeDefinition.colors['background'] = neutral.background;
        themeDefinition.colors['on-background'] = neutral.foreground;
        themeDefinition.colors['surface'] = neutral.surface;
        themeDefinition.colors['on-surface'] = neutral.foreground;
        themeDefinition.colors['surface-muted'] = neutral.surfaceMuted;
        themeDefinition.colors['surface-raised'] = neutral.surfaceRaised;
        themeDefinition.colors['auth-background'] = neutral.authBackground;
        themeDefinition.colors['border'] = neutral.border;
    }

    applyThemeColorSchemeCssVariables(schemeType);
}
