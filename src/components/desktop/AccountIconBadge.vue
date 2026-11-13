<template>
    <v-badge class="right-bottom-icon account-icon-badge-wrapper"
             color="secondary" offset-y="4"
             :location="`bottom ${textDirection === TextDirection.LTR ? 'right' : 'left'}`"
             :icon="mdiEyeOffOutline" :model-value="!!hiddenStatus">
        <span class="account-icon-badge" :style="badgeStyle">
            <img class="account-icon-badge__img" :src="customIconUrl" alt="" draggable="false"
                 v-if="showCustomImage" @error="onCustomImageError" />
            <i class="account-icon-badge__glyph" :class="systemIconClass" v-else></i>
        </span>
    </v-badge>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';

import { type CommonIconProps, useItemIconBase } from '@/components/base/ItemIconBase.ts';

import { useI18n } from '@/locales/helpers.ts';

import { TextDirection } from '@/core/text.ts';

import { DEFAULT_ACCOUNT_COLOR } from '@/consts/color.ts';

import {
    mdiEyeOffOutline
} from '@mdi/js';

interface AccountIconBadgeProps extends CommonIconProps {
    defaultIconId?: string | number;
    hiddenStatus?: boolean;
}

const props = defineProps<AccountIconBadgeProps>();

const { getCurrentLanguageTextDirection } = useI18n();
const { customIconUrl, getAccountIcon } = useItemIconBase(props);

const DEFAULT_BADGE_RGB = 'var(--v-theme-primary, var(--ebk-primary-color))';

const textDirection = computed<TextDirection>(() => getCurrentLanguageTextDirection());
const customImageFailed = ref<boolean>(false);

const showCustomImage = computed<boolean>(() => !!customIconUrl.value && !customImageFailed.value);

const systemIconClass = computed<string>(() => {
    if (props.iconType !== 'user-custom') {
        return getAccountIcon(props.iconId);
    }

    // user-custom icons have no system icon id, fall back to the account category default icon
    return getAccountIcon(props.defaultIconId ?? props.iconId);
});

const badgeStyle = computed<Record<string, string>>(() => ({
    '--ebk-account-icon-badge-rgb': getBadgeColorRgb(props.color)
}));

function getBadgeColorRgb(color?: string): string {
    if (!color) {
        return DEFAULT_BADGE_RGB;
    }

    let hex = color.trim().replace(/^#/, '');

    if (/^[0-9a-f]{3}$/i.test(hex)) {
        hex = hex.replace(/./g, char => char + char);
    } else if (/^[0-9a-f]{8}$/i.test(hex)) {
        hex = hex.substring(0, 6);
    }

    // the default (black) account color means "use the brand primary color"
    if (!/^[0-9a-f]{6}$/i.test(hex) || hex.toLowerCase() === DEFAULT_ACCOUNT_COLOR.toLowerCase()) {
        return DEFAULT_BADGE_RGB;
    }

    const red = parseInt(hex.substring(0, 2), 16);
    const green = parseInt(hex.substring(2, 4), 16);
    const blue = parseInt(hex.substring(4, 6), 16);

    return `${red}, ${green}, ${blue}`;
}

function onCustomImageError(): void {
    customImageFailed.value = true;
}

watch(customIconUrl, () => {
    customImageFailed.value = false;
});
</script>

<style>
.account-icon-badge-wrapper {
    display: inline-flex;
    flex: 0 0 auto;
    align-items: center;
    vertical-align: middle;
}

.account-icon-badge {
    display: inline-flex;
    flex: 0 0 auto;
    align-items: center;
    justify-content: center;
    box-sizing: border-box;
    width: var(--ebk-account-icon-badge-size);
    height: var(--ebk-account-icon-badge-size);
    min-width: var(--ebk-account-icon-badge-size);
    min-height: var(--ebk-account-icon-badge-size);
    border-radius: 50%;
    overflow: hidden;
    vertical-align: middle;
    line-height: 1;
    background-color: rgba(var(--ebk-account-icon-badge-rgb), var(--ebk-account-icon-badge-bg-opacity));
    color: color-mix(in srgb,
                     rgb(var(--ebk-account-icon-badge-rgb)),
                     rgb(var(--v-theme-on-surface)) var(--ebk-account-icon-badge-fg-mix));
}

.account-icon-badge__img {
    display: block;
    box-sizing: border-box;
    width: 100%;
    height: 100%;
    padding: calc((var(--ebk-account-icon-badge-size) - var(--ebk-account-icon-badge-icon-size)) / 2);
    object-fit: contain;
    object-position: center;
    border: none;
}

.account-icon-badge__glyph {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: var(--ebk-account-icon-badge-icon-size);
    height: var(--ebk-account-icon-badge-icon-size);
    font-size: var(--ebk-account-icon-badge-icon-size);
    line-height: 1;
    color: inherit;
    font-style: normal;
}

.account-icon-badge--placeholder {
    background-color: rgba(var(--v-theme-on-surface), 0.08);
}
</style>
