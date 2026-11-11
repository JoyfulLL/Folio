<template>
    <v-row>
        <v-col cols="12">
            <v-card class="setting-items" :class="{ 'disabled': loading || saving }">
                <template #title>
                    <span>{{ tt('Basic Settings') }}</span>
                    <v-progress-circular indeterminate size="20" class="ms-3" v-if="loading"></v-progress-circular>
                </template>

                <v-card-text class="d-flex mt-2">
                    <v-avatar rounded="lg" variant="tonal" size="100" class="me-4 user-profile-avatar-icon"
                              :class="{ 'cursor-pointer': avatarProvider === 'internal', 'user-profile-avatar-icon-modifiable': avatarProvider === 'internal' }"
                              :color="currentUserAvatar ? 'rgba(0,0,0,0)' : 'primary'">
                        <v-img :src="currentUserAvatar" v-if="currentUserAvatar">
                            <template #placeholder>
                                <div class="d-flex align-center justify-center bg-light-primary">
                                    <v-icon color="primary" size="48" class="user-profile-avatar-placeholder" :icon="mdiAccount"/>
                                </div>
                            </template>
                        </v-img>
                        <v-icon size="48" class="user-profile-avatar-placeholder" :icon="mdiAccount" v-else-if="!currentUserAvatar"/>
                        <div class="avatar-edit-icon" v-if="avatarProvider === 'internal'">
                            <v-icon size="48" :icon="mdiAccountEditOutline"/>
                        </div>
                        <v-menu activator="parent" width="200" location="bottom" offset="14px" v-if="avatarProvider === 'internal'">
                            <v-list>
                                <v-list-item :disabled="saving" :title="tt('Update Avatar')" @click="showOpenAvatarDialog"></v-list-item>
                                <v-list-item :disabled="!currentUserAvatar || saving" :title="tt('Remove Avatar')" @click="removeAvatar"></v-list-item>
                            </v-list>
                        </v-menu>
                    </v-avatar>
                    <div class="d-flex flex-column justify-center gap-3">
                        <div class="d-flex text-body-large">
                            <span class="me-1">{{ tt('Username:') }}</span>
                            <v-skeleton-loader class="skeleton-no-margin" type="text" style="width: 100px" :loading="true" v-if="loading"></v-skeleton-loader>
                            <span v-if="!loading">{{ oldProfile.username }}</span>
                        </div>
                        <div class="d-flex text-body-large align-center" style="height: 40px;">
                            <span v-if="!loading && emailVerified">{{ tt('Email address is verified') }}</span>
                            <span v-if="!loading && !emailVerified">{{ tt('Email address is not verified') }}</span>
                            <v-btn class="ms-2 px-2" size="small" variant="text" :disabled="loading || resending"
                                   @click="resendVerifyEmail" v-if="isUserVerifyEmailEnabled() && !loading && !emailVerified">
                                {{ tt('Resend Validation Email') }}
                                <v-progress-circular indeterminate size="18" class="ms-2" v-if="resending"></v-progress-circular>
                            </v-btn>
                            <v-skeleton-loader class="skeleton-no-margin mt-2 mb-1" type="text" style="width: 160px" :loading="true" v-if="loading"></v-skeleton-loader>
                        </div>
                    </div>
                </v-card-text>

                <v-divider />

                <v-form>
                    <v-card-text class="my-1 pa-0 text-body-medium">
                        <div class="setting-item">
                            <span>{{ tt('Nickname') }}</span>
                            <v-spacer/>
                            <v-text-field
                                class="ms-4"
                                density="compact"
                                type="text"
                                autocomplete="nickname"
                                persistent-placeholder
                                max-width="400px"
                                :disabled="loading || saving"
                                :placeholder="tt('Your nickname')"
                                v-model="newProfile.nickname"
                            />
                        </div>

                        <div class="setting-item">
                            <span>{{ tt('E-mail') }}</span>
                            <v-spacer/>
                            <v-text-field
                                class="ms-4"
                                density="compact"
                                type="email"
                                autocomplete="email"
                                persistent-placeholder
                                max-width="400px"
                                :disabled="loading || saving"
                                :placeholder="tt('Your email address')"
                                v-model="newProfile.email"
                            />
                        </div>

                        <div class="setting-item">
                            <span>{{ tt('Default Account') }}</span>
                            <v-spacer/>
                            <two-column-select class="ms-4" density="compact" max-width="600px"
                                               primary-key-field="id" primary-value-field="category"
                                               primary-title-field="name"
                                               primary-icon-field="icon" primary-icon-type-field="iconType" primary-icon-type="account"
                                               primary-sub-items-field="accounts"
                                               :primary-title-i18n="true"
                                               secondary-key-field="id" secondary-value-field="id"
                                               secondary-title-field="name"
                                               secondary-icon-field="icon" secondary-icon-type-field="iconType" secondary-icon-type="account" secondary-color-field="color"
                                               :disabled="loading || saving || !allVisibleAccounts.length"
                                               :enable-filter="true" :filter-placeholder="tt('Find account')" :filter-no-items-text="tt('No available account')"
                                               :placeholder="tt('Default Account')"
                                               :items="allVisibleCategorizedAccounts"
                                               :no-item-text="Account.findAccountNameById(allAccounts, newProfile.defaultAccountId, tt('Unspecified'))"
                                               v-model="newProfile.defaultAccountId">
                            </two-column-select>
                        </div>

                        <div class="setting-item">
                            <span>{{ tt('Use Last Reconciled Time') }}</span>
                            <v-spacer/>
                            <v-select
                                class="ms-4"
                                density="compact"
                                item-title="displayName"
                                item-value="value"
                                persistent-placeholder
                                :disabled="loading || saving"
                                max-width="400px"
                                :placeholder="tt('Use Last Reconciled Time')"
                                :items="enableDisableOptions"
                                v-model="newProfile.useLastReconciledTime"
                            >
                                <template #item="{ props, internalItem }">
                                    <v-list-item :disabled="!internalItem.raw.value && (TransactionEditScopeType.valueOf(newProfile.transactionEditScope)?.needLastReconciledTime ?? false)" v-bind="props">
                                        <template #title>
                                            <div class="text-truncate">{{ internalItem.raw.displayName }}</div>
                                        </template>
                                    </v-list-item>
                                </template>
                            </v-select>
                        </div>

                        <div class="setting-item">
                            <span>{{ tt('Editable Transaction Range') }}</span>
                            <v-spacer/>
                            <v-select
                                class="ms-4"
                                density="compact"
                                item-title="displayName"
                                item-value="type"
                                persistent-placeholder
                                max-width="400px"
                                :disabled="loading || saving"
                                :placeholder="tt('Editable Transaction Range')"
                                :items="allTransactionEditScopeTypes"
                                v-model="newProfile.transactionEditScope"
                            />
                        </div>
                    </v-card-text>

                    <v-divider />

                    <v-card-text class="my-1 pa-0 text-body-medium">
                        <div class="setting-item">
                            <span>{{ languageTitle }}</span>
                            <v-spacer/>
                            <language-select class="ms-4" density="compact" max-width="400px"
                                             :disabled="loading || saving"
                                             :placeholder="languageTitle"
                                             :include-system-default="true"
                                             :use-model-value="true" v-model="newProfile.language" />
                        </div>

                        <div class="setting-item">
                            <span>{{ tt('Default Currency') }}</span>
                            <v-spacer/>
                            <currency-select class="ms-4" density="compact" max-width="400px"
                                             :disabled="loading || saving"
                                             :placeholder="tt('Default Currency')"
                                             v-model="newProfile.defaultCurrency" />
                        </div>

                        <div class="setting-item">
                            <span>{{ tt('First Day of Week') }}</span>
                            <v-spacer/>
                            <v-select
                                class="ms-4"
                                density="compact"
                                item-title="displayName"
                                item-value="type"
                                persistent-placeholder
                                max-width="400px"
                                :disabled="loading || saving"
                                :placeholder="tt('First Day of Week')"
                                :items="allWeekDays"
                                v-model="newProfile.firstDayOfWeek"
                            />
                        </div>

                        <div class="setting-item">
                            <span>{{ tt('Fiscal Year Start Date') }}</span>
                            <v-spacer/>
                            <fiscal-year-start-select
                                class="ms-4"
                                density="compact"
                                persistent-placeholder
                                max-width="400px"
                                :disabled="loading || saving"
                                :placeholder="tt('Fiscal Year Start Date')"
                                :numeral-system="newProfile.numeralSystem"
                                v-model="newProfile.fiscalYearStart"
                            />
                        </div>
                    </v-card-text>

                    <v-divider />

                    <v-card-text class="my-1 pa-0 text-body-medium">
                        <div class="setting-item">
                            <span>{{ tt('Calendar Display Type') }}</span>
                            <v-spacer/>
                            <v-select
                                class="ms-4"
                                density="compact"
                                item-title="displayName"
                                item-value="type"
                                persistent-placeholder
                                max-width="400px"
                                :disabled="loading || saving"
                                :placeholder="tt('Calendar Display Type')"
                                :items="allCalendarDisplayTypes"
                                v-model="newProfile.calendarDisplayType"
                            />
                        </div>

                        <div class="setting-item">
                            <span>{{ tt('Date Display Type') }}</span>
                            <v-spacer/>
                            <v-select
                                class="ms-4"
                                density="compact"
                                item-title="displayName"
                                item-value="type"
                                persistent-placeholder
                                max-width="400px"
                                :disabled="loading || saving"
                                :placeholder="tt('Date Display Type')"
                                :items="allDateDisplayTypes"
                                v-model="newProfile.dateDisplayType"
                            />
                        </div>

                        <div class="setting-item">
                            <span>{{ tt('Long Date Format') }}</span>
                            <v-spacer/>
                            <v-select
                                class="ms-4"
                                density="compact"
                                item-title="displayName"
                                item-value="type"
                                persistent-placeholder
                                max-width="400px"
                                :disabled="loading || saving"
                                :placeholder="tt('Long Date Format')"
                                :items="allLongDateFormats"
                                v-model="newProfile.longDateFormat"
                            />
                        </div>

                        <div class="setting-item">
                            <span>{{ tt('Short Date Format') }}</span>
                            <v-spacer/>
                            <v-select
                                class="ms-4"
                                density="compact"
                                item-title="displayName"
                                item-value="type"
                                persistent-placeholder
                                max-width="400px"
                                :disabled="loading || saving"
                                :placeholder="tt('Short Date Format')"
                                :items="allShortDateFormats"
                                v-model="newProfile.shortDateFormat"
                            />
                        </div>

                        <div class="setting-item">
                            <span>{{ tt('Long Time Format') }}</span>
                            <v-spacer/>
                            <v-select
                                class="ms-4"
                                density="compact"
                                item-title="displayName"
                                item-value="type"
                                persistent-placeholder
                                max-width="400px"
                                :disabled="loading || saving"
                                :placeholder="tt('Long Time Format')"
                                :items="allLongTimeFormats"
                                v-model="newProfile.longTimeFormat"
                            />
                        </div>

                        <div class="setting-item">
                            <span>{{ tt('Short Time Format') }}</span>
                            <v-spacer/>
                            <v-select
                                class="ms-4"
                                density="compact"
                                item-title="displayName"
                                item-value="type"
                                persistent-placeholder
                                max-width="400px"
                                :disabled="loading || saving"
                                :placeholder="tt('Short Time Format')"
                                :items="allShortTimeFormats"
                                v-model="newProfile.shortTimeFormat"
                            />
                        </div>

                        <div class="setting-item">
                            <span>{{ tt('Fiscal Year Format') }}</span>
                            <v-spacer/>
                            <v-select
                                class="ms-4"
                                density="compact"
                                item-title="displayName"
                                item-value="type"
                                persistent-placeholder
                                max-width="400px"
                                :disabled="loading || saving"
                                :placeholder="tt('Fiscal Year Format')"
                                :items="allFiscalYearFormats"
                                v-model="newProfile.fiscalYearFormat"
                            />
                        </div>
                    </v-card-text>

                    <v-divider />

                    <v-card-text class="my-1 pa-0 text-body-medium">
                        <div class="setting-item">
                            <span>{{ tt('Currency Display Mode') }}</span>
                            <v-spacer/>
                            <v-select
                                class="ms-4"
                                density="compact"
                                item-title="displayName"
                                item-value="type"
                                persistent-placeholder
                                max-width="400px"
                                :disabled="loading || saving"
                                :placeholder="tt('Currency Display Mode')"
                                :items="allCurrencyDisplayTypes"
                                v-model="newProfile.currencyDisplayType"
                            />
                        </div>

                        <div class="setting-item">
                            <span>{{ tt('Numeral System') }}</span>
                            <v-spacer/>
                            <v-select
                                class="ms-4"
                                density="compact"
                                item-title="displayName"
                                item-value="type"
                                persistent-placeholder
                                max-width="400px"
                                :disabled="loading || saving"
                                :placeholder="tt('Numeral System')"
                                :items="allNumeralSystemTypes"
                                v-model="newProfile.numeralSystem"
                            />
                        </div>

                        <div class="setting-item">
                            <span>{{ tt('Digit Grouping') }}</span>
                            <v-spacer/>
                            <v-select
                                class="ms-4"
                                density="compact"
                                item-title="displayName"
                                item-value="type"
                                persistent-placeholder
                                max-width="400px"
                                :disabled="loading || saving"
                                :placeholder="tt('Digit Grouping')"
                                :items="allDigitGroupingTypes"
                                v-model="newProfile.digitGrouping"
                            />
                        </div>

                        <div class="setting-item">
                            <span>{{ tt('Digit Grouping Symbol') }}</span>
                            <v-spacer/>
                            <v-select
                                class="ms-4"
                                density="compact"
                                item-title="displayName"
                                item-value="type"
                                persistent-placeholder
                                max-width="400px"
                                :disabled="loading || saving || !supportDigitGroupingSymbol"
                                :placeholder="tt('Digit Grouping Symbol')"
                                :items="allDigitGroupingSymbols"
                                v-model="newProfile.digitGroupingSymbol"
                            />
                        </div>

                        <div class="setting-item">
                            <span>{{ tt('Decimal Separator') }}</span>
                            <v-spacer/>
                            <v-select
                                class="ms-4"
                                density="compact"
                                item-title="displayName"
                                item-value="type"
                                persistent-placeholder
                                max-width="400px"
                                :disabled="loading || saving"
                                :placeholder="tt('Decimal Separator')"
                                :items="allDecimalSeparators"
                                v-model="newProfile.decimalSeparator"
                            />
                        </div>
                    </v-card-text>

                    <v-divider />

                    <v-card-text class="my-1 pa-0 text-body-medium">
                        <div class="setting-item">
                            <span>{{ tt('Geographic Location Format') }}</span>
                            <v-spacer/>
                            <v-select
                                class="ms-4"
                                density="compact"
                                item-title="displayName"
                                item-value="type"
                                persistent-placeholder
                                max-width="400px"
                                :disabled="loading || saving"
                                :placeholder="tt('Geographic Location Format')"
                                :items="allCoordinateDisplayTypes"
                                v-model="newProfile.coordinateDisplayType"
                            />
                        </div>
                    </v-card-text>

                    <v-divider />

                    <v-card-text class="my-1 pa-0 text-body-medium">
                        <div class="setting-item">
                            <span>{{ tt('Expense Amount Color') }}</span>
                            <v-spacer/>
                            <v-select
                                class="ms-4"
                                density="compact"
                                item-title="displayName"
                                item-value="type"
                                persistent-placeholder
                                max-width="400px"
                                :disabled="loading || saving"
                                :placeholder="tt('Expense Amount Color')"
                                :items="allExpenseAmountColorTypes"
                                v-model="newProfile.expenseAmountColor"
                            />
                        </div>

                        <div class="setting-item">
                            <span>{{ tt('Income Amount Color') }}</span>
                            <v-spacer/>
                            <v-select
                                class="ms-4"
                                density="compact"
                                item-title="displayName"
                                item-value="type"
                                persistent-placeholder
                                max-width="400px"
                                :disabled="loading || saving"
                                :placeholder="tt('Income Amount Color')"
                                :items="allIncomeAmountColorTypes"
                                v-model="newProfile.incomeAmountColor"
                            />
                        </div>
                    </v-card-text>

                    <v-divider />

                    <v-card-text class="my-1 pa-0 text-body-medium">
                        <div class="setting-item">
                            <span>{{ tt('Credit Card Notification Check Interval') }}</span>
                            <v-spacer/>
                            <v-select
                                class="ms-4"
                                density="compact"
                                item-title="displayName"
                                item-value="type"
                                persistent-placeholder
                                max-width="400px"
                                :disabled="loading || saving"
                                :placeholder="tt('Credit Card Notification Check Interval')"
                                :items="allCreditCardNotificationIntervals"
                                v-model="newProfile.creditCardNotificationInterval"
                            />
                        </div>
                    </v-card-text>

                    <v-divider />

                    <v-card-text class="d-flex flex-wrap gap-4">
                        <v-btn :disabled="inputIsNotChanged || inputIsInvalid || saving" @click="save">
                            {{ tt('Save Changes') }}
                            <v-progress-circular indeterminate size="22" class="ms-2" v-if="saving"></v-progress-circular>
                        </v-btn>

                        <v-btn color="default" variant="tonal" @click="reset">
                            {{ tt('Reset') }}
                        </v-btn>
                    </v-card-text>
                </v-form>
            </v-card>
        </v-col>
    </v-row>

    <v-row>
        <v-col cols="12">
            <v-card class="setting-items" :class="{ 'disabled': dailyReportLoading }">
                <v-card-title>{{ tt('Daily Transaction Report') }}</v-card-title>

                <v-card-text class="text-body-medium mt-1">{{ tt('Push the daily income and expense report to a feishu group chat via a custom bot webhook') }}</v-card-text>

                <v-form>
                    <v-card-text class="my-1 pa-0 text-body-medium">
                        <div class="setting-item">
                            <span>{{ tt('Enable Daily Report') }}</span>
                            <v-spacer/>
                            <v-switch
                                class="ms-4"
                                :disabled="dailyReportLoading || dailyReportSaving || dailyReportTesting"
                                v-model="dailyReport.enabled"
                                hide-details
                            />
                        </div>

                        <div class="setting-item">
                            <span>{{ tt('Feishu Webhook') }}</span>
                            <v-spacer/>
                            <v-text-field
                                class="ms-4"
                                density="compact"
                                max-width="500px"
                                persistent-placeholder
                                autocomplete="off"
                                :disabled="dailyReportLoading || dailyReportSaving || dailyReportTesting"
                                :label="tt('Feishu Webhook')"
                                :placeholder="tt('Not set')"
                                :hint="dailyReportWebhookUrlHint"
                                persistent-hint
                                v-model="dailyReport.webhookUrl"
                            />
                        </div>

                        <div class="setting-item">
                            <span>{{ tt('Feishu Signing Secret') }}</span>
                            <v-spacer/>
                            <v-text-field
                                class="ms-4"
                                density="compact"
                                max-width="500px"
                                persistent-placeholder
                                autocomplete="new-password"
                                type="password"
                                :disabled="dailyReportLoading || dailyReportSaving || dailyReportTesting"
                                :label="tt('Feishu Signing Secret')"
                                :hint="dailyReportSigningSecretSet ? tt('Leave blank to keep the saved secret') : tt('Encrypted with the server secret key before being stored')"
                                persistent-hint
                                v-model="dailyReport.signingSecret"
                            />
                        </div>

                        <div class="setting-item">
                            <span>{{ tt('Custom Keyword') }}</span>
                            <v-spacer/>
                            <v-text-field
                                class="ms-4"
                                density="compact"
                                max-width="500px"
                                persistent-placeholder
                                autocomplete="off"
                                :disabled="dailyReportLoading || dailyReportSaving || dailyReportTesting"
                                :label="tt('Custom Keyword')"
                                :hint="tt('Required when the bot enables the keyword check, leave blank otherwise')"
                                persistent-hint
                                v-model="dailyReport.keyword"
                            />
                        </div>

                        <div class="setting-item">
                            <span>{{ tt('Daily Report Send Time') }}</span>
                            <v-spacer/>
                            <v-text-field
                                class="ms-4"
                                density="compact"
                                max-width="500px"
                                persistent-placeholder
                                type="time"
                                :disabled="dailyReportLoading || dailyReportSaving || dailyReportTesting"
                                :label="tt('Daily Report Send Time')"
                                :hint="tt('Local time of the report timezone, 00:00 means following the server default')"
                                persistent-hint
                                v-model="dailyReport.sendTime"
                            />
                        </div>
                    </v-card-text>

                    <v-card-text class="d-flex flex-wrap gap-4">
                        <v-btn :disabled="dailyReportLoading || !dailyReportChanged || dailyReportSaving || dailyReportTesting" @click="saveDailyReport">
                            {{ tt('Save Daily Report Settings') }}
                            <v-progress-circular indeterminate size="22" class="ms-2" v-if="dailyReportSaving"></v-progress-circular>
                        </v-btn>

                        <v-btn color="default" variant="tonal" :disabled="dailyReportLoading || dailyReportSaving || dailyReportTesting" @click="testDailyReport">
                            {{ tt('Send Test Message') }}
                            <v-progress-circular indeterminate size="22" class="ms-2" v-if="dailyReportTesting"></v-progress-circular>
                        </v-btn>
                    </v-card-text>
                </v-form>
            </v-card>
        </v-col>
    </v-row>

    <confirm-dialog ref="confirmDialog"/>
    <snack-bar ref="snackbar" />
    <input ref="avatarInput" type="file" style="display: none" :accept="SUPPORTED_IMAGE_EXTENSIONS" @change="updateAvatar($event)" />
</template>

<script setup lang="ts">
import ConfirmDialog from '@/components/desktop/ConfirmDialog.vue';
import SnackBar from '@/components/desktop/SnackBar.vue';

import { ref, computed, useTemplateRef } from 'vue';

import { useI18n } from '@/locales/helpers.ts';
import { useUserProfilePageBase } from '@/views/base/users/UserProfilePageBase.ts';

import { useRootStore } from '@/stores/index.ts';
import { useUserStore } from '@/stores/user.ts';
import { useAccountsStore } from '@/stores/account.ts';

import { TransactionEditScopeType } from '@/core/transaction.ts';
import { SUPPORTED_IMAGE_EXTENSIONS } from '@/consts/file.ts';
import type { UserProfileResponse } from '@/models/user.ts';
import { Account } from '@/models/account.ts';

import { isUserVerifyEmailEnabled } from '@/lib/server_settings.ts';
import services from '@/lib/services.ts';
import type { UserDailyReportSettingGetResponse } from '@/models/user_daily_report_setting.ts';

import {
    mdiAccount,
    mdiAccountEditOutline
} from '@mdi/js';

type ConfirmDialogType = InstanceType<typeof ConfirmDialog>;
type SnackBarType = InstanceType<typeof SnackBar>;

const { tt } = useI18n();

const {
    newProfile,
    oldProfile,
    emailVerified,
    loading,
    resending,
    saving,
    allAccounts,
    allVisibleAccounts,
    allVisibleCategorizedAccounts,
    allWeekDays,
    allCalendarDisplayTypes,
    allDateDisplayTypes,
    allLongDateFormats,
    allShortDateFormats,
    allLongTimeFormats,
    allShortTimeFormats,
    allFiscalYearFormats,
    allCurrencyDisplayTypes,
    allNumeralSystemTypes,
    allDecimalSeparators,
    allDigitGroupingSymbols,
    allDigitGroupingTypes,
    allCoordinateDisplayTypes,
    allExpenseAmountColorTypes,
    allIncomeAmountColorTypes,
    allTransactionEditScopeTypes,
    enableDisableOptions,
    allCreditCardNotificationIntervals,
    languageTitle,
    supportDigitGroupingSymbol,
    inputIsNotChangedProblemMessage,
    inputInvalidProblemMessage,
    langAndRegionInputInvalidProblemMessage,
    extendInputInvalidProblemMessage,
    inputIsNotChanged,
    inputIsInvalid,
    setCurrentUserProfile,
    reset,
    doAfterProfileUpdate
} = useUserProfilePageBase();

const rootStore = useRootStore();
const userStore = useUserStore();
const accountsStore = useAccountsStore();

const confirmDialog = useTemplateRef<ConfirmDialogType>('confirmDialog');
const snackbar = useTemplateRef<SnackBarType>('snackbar');
const avatarInput = useTemplateRef<HTMLInputElement>('avatarInput');

const avatarUrl = ref<string>('');
const avatarProvider = ref<string | undefined>('');

const currentUserAvatar = computed<string | null>(() => userStore.getUserAvatarUrl(avatarUrl.value));

const dailyReportLoading = ref<boolean>(false);
const dailyReportSaving = ref<boolean>(false);
const dailyReportTesting = ref<boolean>(false);
const dailyReportSaved = ref<UserDailyReportSettingGetResponse | null>(null);
const dailyReport = ref({
    enabled: false,
    webhookUrl: '',
    signingSecret: '',
    keyword: '',
    sendTime: '00:05'
});

const dailyReportSendTimeMinutes = computed<number>(() => dailyReportParseTimeToMinutes(dailyReport.value.sendTime));

const dailyReportChanged = computed<boolean>(() => {
    const saved = dailyReportSaved.value;

    if (!saved) {
        return false;
    }

    return dailyReport.value.enabled !== saved.enabled
        || dailyReport.value.webhookUrl.trim() !== ''
        || dailyReport.value.signingSecret !== ''
        || dailyReport.value.keyword.trim() !== saved.keyword
        || dailyReportSendTimeMinutes.value !== saved.sendTimeMinutesOfDay;
});

const dailyReportSigningSecretSet = computed<boolean>(() => !!dailyReportSaved.value?.signingSecretSet);

const dailyReportWebhookUrlHint = computed<string>(() => {
    const saved = dailyReportSaved.value;

    if (saved && saved.webhookUrlSet && saved.webhookUrlMasked) {
        return saved.webhookUrlMasked;
    }

    return tt('Not set');
});

function dailyReportParseTimeToMinutes(value: string): number {
    const parts = value.split(':');

    if (parts.length !== 2) {
        return 0;
    }

    const h = parseInt(parts[0] ?? '', 10);
    const m = parseInt(parts[1] ?? '', 10);

    if (isNaN(h) || isNaN(m) || h < 0 || h > 23 || m < 0 || m > 59) {
        return 0;
    }

    return h * 60 + m;
}

function dailyReportMinutesToTimeText(minutes: number): string {
    const h = Math.floor(minutes / 60);
    const m = minutes % 60;

    return (h < 10 ? '0' + h : '' + h) + ':' + (m < 10 ? '0' + m : '' + m);
}

function loadDailyReportSettings(): void {
    dailyReportLoading.value = true;

    services.getDailyReportSettings().then(response => {
        const data = response.data;

        if (!data || !data.success || !data.result) {
            dailyReportLoading.value = false;
            return;
        }

        dailyReportSaved.value = data.result;
        dailyReport.value.enabled = data.result.enabled;
        dailyReport.value.keyword = data.result.keyword;
        dailyReport.value.sendTime = dailyReportMinutesToTimeText(data.result.sendTimeMinutesOfDay);
        dailyReportLoading.value = false;
    }).catch(error => {
        dailyReportLoading.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function saveDailyReportSettings(): Promise<void> {
    dailyReportSaving.value = true;

    return services.updateDailyReportSettings({
        enabled: dailyReport.value.enabled,
        webhookUrl: dailyReport.value.webhookUrl.trim(),
        signingSecret: dailyReport.value.signingSecret,
        keyword: dailyReport.value.keyword.trim(),
        sendTimeMinutesOfDay: dailyReportSendTimeMinutes.value,
        timezoneName: ''
    }).then(response => {
        const data = response.data;

        if (data && data.success && data.result) {
            dailyReportSaved.value = data.result;
            dailyReport.value.keyword = data.result.keyword;
            dailyReport.value.sendTime = dailyReportMinutesToTimeText(data.result.sendTimeMinutesOfDay);
        }

        dailyReport.value.webhookUrl = '';
        dailyReport.value.signingSecret = '';
        dailyReportSaving.value = false;
        snackbar.value?.showMessage('Daily report settings have been saved');
    }).catch(error => {
        dailyReportSaving.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }

        throw error;
    });
}

function saveDailyReport(): void {
    saveDailyReportSettings().catch(() => {});
}

function testDailyReport(): void {
    const doTest = () => {
        dailyReportTesting.value = true;

        services.testDailyReportSettings().then(() => {
            dailyReportTesting.value = false;
            snackbar.value?.showMessage('Test message has been sent, please check the feishu group');
        }).catch(error => {
            dailyReportTesting.value = false;

            if (!error.processed) {
                snackbar.value?.showError(error);
            }
        });
    };

    if (dailyReportChanged.value) {
        saveDailyReportSettings().then(doTest).catch(() => {});
    } else {
        doTest();
    }
}

function init(): void {
    loading.value = true;
    loadDailyReportSettings();

    const promises = [
        accountsStore.loadAllAccounts({ force: false }),
        userStore.getCurrentUserProfile()
    ];

    Promise.all(promises).then(responses => {
        const profile = responses[1] as UserProfileResponse;
        setCurrentUserProfile(profile);
        avatarUrl.value = profile.avatar;
        avatarProvider.value = profile.avatarProvider;
        loading.value = false;
    }).catch(error => {
        oldProfile.value.nickname = '';
        oldProfile.value.email = '';
        newProfile.value.nickname = '';
        newProfile.value.email = '';
        loading.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function save(): void {
    const problemMessage = inputIsNotChangedProblemMessage.value || inputInvalidProblemMessage.value || extendInputInvalidProblemMessage.value || langAndRegionInputInvalidProblemMessage.value;

    if (problemMessage) {
        snackbar.value?.showMessage(problemMessage);
        return;
    }

    saving.value = true;

    rootStore.updateUserProfile(newProfile.value.toProfileUpdateRequest()).then(response => {
        saving.value = false;

        doAfterProfileUpdate(response.user);
        snackbar.value?.showMessage('Your profile has been successfully updated');
    }).catch(error => {
        saving.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function updateAvatar(event: Event): void {
    if (!event || !event.target) {
        return;
    }

    const el = event.target as HTMLInputElement;

    if (!el.files || !el.files.length || !el.files[0]) {
        return;
    }

    const avatarFile = el.files[0] as File;

    el.value = '';

    saving.value = true;

    userStore.updateUserAvatar({ avatarFile }).then(response => {
        saving.value = false;

        if (response) {
            avatarUrl.value = response.avatar;
            avatarProvider.value = response.avatarProvider;
            setCurrentUserProfile(response);
        }

        snackbar.value?.showMessage('Your avatar has been successfully updated');
    }).catch(error => {
        saving.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function removeAvatar(): void {
    confirmDialog.value?.open('Are you sure you want to remove avatar?').then(() => {
        saving.value = true;

        userStore.removeUserAvatar().then(response => {
            saving.value = false;

            if (response) {
                avatarUrl.value = response.avatar;
                avatarProvider.value = response.avatarProvider;
                setCurrentUserProfile(response);
            }

            snackbar.value?.showMessage('Your profile has been successfully updated');
        }).catch(error => {
            saving.value = false;

            if (!error.processed) {
                snackbar.value?.showError(error);
            }
        });
    });
}

function resendVerifyEmail(): void {
    resending.value = true;

    rootStore.resendVerifyEmailByLoginedUser().then(() => {
        resending.value = false;
        snackbar.value?.showMessage('Validation email has been sent');
    }).catch(error => {
        resending.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function showOpenAvatarDialog(): void {
    avatarInput.value?.click();
}

init();
</script>

<style>
.user-profile-avatar-icon .avatar-edit-icon {
    display: none;
    position: absolute;
    width: 100% !important;
    height: 100% !important;
    background-color: rgba(0, 0, 0, 0.4);
}

.user-profile-avatar-icon .avatar-edit-icon > i.v-icon {
    background-color: transparent;
    color: rgba(255, 255, 255, 0.8);
}

.user-profile-avatar-icon:hover .avatar-edit-icon {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    vertical-align: middle;
}

.v-theme--dark {
    .user-profile-avatar-icon:hover .avatar-edit-icon {
        background-color: rgba(64, 64, 64, 0.6);
    }
}

.user-profile-avatar-icon-modifiable:hover .user-profile-avatar-placeholder {
    display: none;
}
</style>
