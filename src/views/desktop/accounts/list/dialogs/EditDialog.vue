<template>
    <v-dialog width="1000" :persistent="loading || submitting || isAccountModified" v-model="showState">
        <two-column-dialog-layout :disabled="loading || submitting" :loading="loading"
                                  :title="tt(title)" :cancel-button-title="tt('Cancel')"
                                  @cancel="cancel">
            <template #content-left-column>
                <div class="px-4">
                    <v-tabs class="v-tabs-pill" direction="vertical" :class="{ 'readonly': !!editAccountId }"
                            :disabled="loading || submitting" v-model="account.type">
                        <v-tab :key="accountType.type" :value="accountType.type" :disabled="!!editAccountId && accountType.type !== account.type"
                               v-for="accountType in allAccountTypes">
                            <span>{{ accountType.displayName }}</span>
                        </v-tab>
                    </v-tabs>
                </div>
                <v-divider class="my-2"/>
                <div class="px-4" v-if="account.type === AccountType.SingleAccount.type">
                    <v-tabs direction="vertical" :disabled="loading || submitting" :model-value="-1">
                        <v-tab :value="-1">
                            <span>{{ tt('Basic Information') }}</span>
                        </v-tab>
                    </v-tabs>
                </div>
                <div class="px-4" v-else-if="account.type === AccountType.MultiSubAccounts.type">
                    <v-tabs direction="vertical" :disabled="loading || submitting" v-model="currentAccountIndex">
                        <v-tab :value="-1">
                            <span>{{ tt('Main Account') }}</span>
                        </v-tab>
                        <template v-if="account.type === AccountType.MultiSubAccounts.type">
                            <v-tab :key="idx" :value="idx" v-for="(subAccount, idx) in subAccounts">
                                <span>{{ tt('Sub Account') + ' #' + (idx + 1) }}</span>
                                <v-btn class="ms-2" color="error" size="24" variant="text"
                                       :aria-label="tt('Remove')" :icon="mdiDeleteOutline"
                                       @click="removeSubAccount(subAccount)"></v-btn>
                            </v-tab>
                        </template>
                    </v-tabs>
                    <div class="w-100">
                        <v-btn class="mt-2 w-100" color="primary" variant="text" density="comfortable"
                               :disabled="loading || submitting" :prepend-icon="mdiPlus"
                               @click="addSubAccount">{{ tt('Add Sub-account') }}</v-btn>
                    </div>
                </div>
            </template>

            <template #content-right-column>
                <v-window class="d-flex flex-grow-1 disable-tab-transition w-100-window-container"
                          v-model="activeTab">
                    <v-window-item value="account">
                        <v-form class="my-4">
                            <v-row>
                                <!-- Basic information -->
                                <v-col cols="12" md="12">
                                    <v-text-field
                                        type="text"
                                        autocomplete="off"
                                        persistent-placeholder
                                        :disabled="loading || submitting"
                                        :label="currentAccountIndex < 0 ? tt('Account Name') : tt('Sub-account Name')"
                                        :placeholder="currentAccountIndex < 0 ? tt('Your account name') : tt('Your sub-account name')"
                                        v-model="selectedAccount.name"
                                    />
                                </v-col>
                                <v-col cols="12" md="12" v-if="account.type === AccountType.SingleAccount.type || currentAccountIndex < 0">
                                    <v-select
                                        item-title="displayName"
                                        item-value="type"
                                        persistent-placeholder
                                        :disabled="loading || submitting"
                                        :label="tt('Account Category')"
                                        :placeholder="tt('Account Category')"
                                        :items="allAccountCategories"
                                        :no-data-text="tt('No results')"
                                        v-model="selectedAccount.category"
                                    >
                                        <template #item="{ props, internalItem }">
                                            <v-list-item :value="internalItem.value" v-bind="props">
                                                <template #title>
                                                    <v-list-item-title>
                                                        <div class="d-flex align-center">
                                                            <ItemIcon icon-type="account"
                                                                      :icon-id="internalItem.raw.defaultAccountIconId"
                                                                      v-if="internalItem.raw" />
                                                            <span class="ms-2">{{ internalItem.title }}</span>
                                                        </div>
                                                    </v-list-item-title>
                                                </template>
                                            </v-list-item>
                                        </template>
                                    </v-select>
                                </v-col>
                                <v-col cols="12" md="6">
                                    <icon-select type="account"
                                                 :all-system-icon-infos="ALL_ACCOUNT_ICONS"
                                                 :label="currentAccountIndex < 0 ? tt('Account Icon') : tt('Sub-account Icon')"
                                                 :color="selectedAccount.color"
                                                 :disabled="loading || submitting"
                                                 v-model:icon-type="selectedAccount.iconType"
                                                 v-model="selectedAccount.icon" />
                                </v-col>
                                <v-col cols="12" md="6">
                                    <color-select :all-system-color-infos="ALL_ACCOUNT_COLORS"
                                                  :label="currentAccountIndex < 0 ? tt('Account Color') : tt('Sub-account Color')"
                                                  :disabled="loading || submitting"
                                                  v-model="selectedAccount.color" />
                                </v-col>
                                <v-col cols="12" md="12" v-if="account.type === AccountType.SingleAccount.type || currentAccountIndex >= 0 || (account.type === AccountType.MultiSubAccounts.type && account.category === AccountCategory.CreditCard.type && currentAccountIndex < 0)">
                                    <currency-select :disabled="loading || submitting || (!!editAccountId && !isNewAccount(selectedAccount) && !(account.type === AccountType.MultiSubAccounts.type && account.category === AccountCategory.CreditCard.type && currentAccountIndex < 0))"
                                                     :label="(account.type === AccountType.MultiSubAccounts.type && account.category === AccountCategory.CreditCard.type && currentAccountIndex < 0) ? tt('Default Currency') : tt('Currency')"
                                                     :placeholder="(account.type === AccountType.MultiSubAccounts.type && account.category === AccountCategory.CreditCard.type && currentAccountIndex < 0) ? tt('Default Currency') : tt('Currency')"
                                                     :with-not-set="account.type === AccountType.MultiSubAccounts.type && account.category === AccountCategory.CreditCard.type && currentAccountIndex < 0"
                                                     v-model="selectedAccount.currency" />
                                </v-col>
                                <v-col cols="12" :md="((canShowBalanceTime && selectedAccount.numericBalance) || canShowLastReconciledTime) ? 6 : 12"
                                       v-if="account.type === AccountType.SingleAccount.type || currentAccountIndex >= 0">
                                    <amount-input :disabled="loading || submitting || (!!editAccountId && !isNewAccount(selectedAccount))"
                                                  :persistent-placeholder="true"
                                                  :currency="selectedAccount.currency"
                                                  :show-currency="true"
                                                  :flip-negative="account.isLiability"
                                                  :label="accountAmountTitle"
                                                  :placeholder="accountAmountTitle"
                                                  v-model="selectedAccount.numericBalance"/>
                                </v-col>
                                <v-col cols="12" md="6" v-show="selectedAccount.numericBalance" v-if="canShowBalanceTime">
                                    <date-time-select
                                        :disabled="loading || submitting"
                                        :label="tt('Balance Time')"
                                        :timezone-utc-offset="getDefaultTimezoneOffsetMinutes(selectedAccount.balanceTime)"
                                        :model-value="selectedAccount.balanceTime"
                                        @update:model-value="updateAccountBalanceTime(selectedAccount, $event)"
                                        @error="onShowDateTimeError" />
                                </v-col>
                                <v-col cols="12" md="6" v-if="canShowLastReconciledTime">
                                    <date-time-select
                                        :disabled="loading || submitting"
                                        :clearable="true"
                                        :label="tt('Last Reconciled Time')"
                                        :timezone-utc-offset="getDefaultTimezoneOffsetMinutes(selectedAccount.lastReconciledTime)"
                                        :model-value="selectedAccount.lastReconciledTime ?? getCurrentUnixTime()"
                                        :empty-value="!selectedAccount.lastReconciledTime"
                                        @update:model-value="updateAccountLastReconciledTime(selectedAccount, $event)"
                                        @clear:model-value="selectedAccount.lastReconciledTime = undefined"
                                        @error="onShowDateTimeError" />
                                </v-col>
                                <!-- Credit card profile -->
                                <v-col cols="12" md="6" v-if="account.category === AccountCategory.CreditCard.type">
                                    <v-text-field
                                        type="text"
                                        autocomplete="off"
                                        persistent-placeholder
                                        :maxlength="64"
                                        :disabled="loading || submitting"
                                        :label="tt('Bank Name')"
                                        :placeholder="tt('Bank Name')"
                                        v-model="selectedAccount.creditCardBankName"
                                    />
                                </v-col>
                                <v-col cols="12" md="6" v-if="account.category === AccountCategory.CreditCard.type">
                                    <v-text-field
                                        type="text"
                                        autocomplete="off"
                                        persistent-placeholder
                                        :maxlength="64"
                                        :disabled="loading || submitting"
                                        :label="tt('Card Name')"
                                        :placeholder="tt('Card Name')"
                                        v-model="selectedAccount.creditCardCardName"
                                    />
                                </v-col>
                                <v-col cols="12" v-if="account.category === AccountCategory.CreditCard.type">
                                    <v-btn variant="text" size="small" class="advanced-card-details-toggle"
                                           :disabled="loading || submitting"
                                           @click="showAdvancedCardDetails = !showAdvancedCardDetails">
                                        {{ showAdvancedCardDetails ? tt('Hide Advanced Card Details') : tt('Advanced Card Details') }}
                                    </v-btn>
                                </v-col>
                                <template v-if="showAdvancedCardDetails">
                                <v-col cols="12" md="6" v-if="account.category === AccountCategory.CreditCard.type">
                                    <v-text-field
                                        type="text"
                                        autocomplete="off"
                                        persistent-placeholder
                                        :maxlength="4"
                                        :disabled="loading || submitting"
                                        :label="tt('Last Four Digits')"
                                        :placeholder="tt('Last Four Digits')"
                                        v-model="selectedAccount.creditCardLastFourDigits"
                                    />
                                </v-col>
                                <v-col cols="12" md="6" v-if="account.category === AccountCategory.CreditCard.type">
                                    <v-text-field
                                        type="text"
                                        autocomplete="off"
                                        persistent-placeholder
                                        :maxlength="64"
                                        :disabled="loading || submitting"
                                        :label="tt('Cardholder Name')"
                                        :placeholder="tt('Cardholder Name')"
                                        v-model="selectedAccount.creditCardCardholderName"
                                    />
                                </v-col>
                                <v-col cols="12" md="6" v-if="account.category === AccountCategory.CreditCard.type">
                                    <v-autocomplete
                                        item-title="displayName"
                                        item-value="type"
                                        auto-select-first
                                        persistent-placeholder
                                        :disabled="loading || submitting"
                                        :label="tt('Expiration Month')"
                                        :placeholder="tt('Expiration Month')"
                                        :items="allAvailableExpireMonths"
                                        :no-data-text="tt('No results')"
                                        v-model="selectedAccount.creditCardExpireMonth"
                                    ></v-autocomplete>
                                </v-col>
                                <v-col cols="12" md="6" v-if="account.category === AccountCategory.CreditCard.type">
                                    <v-autocomplete
                                        item-title="displayName"
                                        item-value="type"
                                        auto-select-first
                                        persistent-placeholder
                                        :disabled="loading || submitting"
                                        :label="tt('Expiration Year')"
                                        :placeholder="tt('Expiration Year')"
                                        :items="allAvailableExpireYears"
                                        :no-data-text="tt('No results')"
                                        v-model="selectedAccount.creditCardExpireYear"
                                    ></v-autocomplete>
                                </v-col>
                                </template>
                                <v-col cols="12" v-if="account.category === AccountCategory.CreditCard.type">
                                    <div class="text-subtitle-2 mb-2">{{ tt('Card Picture') }}</div>
                                    <v-img v-if="selectedAccount.creditCardPictureUrl"
                                           :src="selectedAccount.creditCardPictureUrl"
                                           max-width="320"
                                           max-height="200"
                                           contain
                                           class="mb-2"></v-img>
                                    <div>
                                        <v-btn size="small" :disabled="loading || submitting" @click="showOpenCreditCardPictureDialog(selectedAccount)">
                                            {{ selectedAccount.creditCardPictureUrl ? tt('Replace Picture') : tt('Upload Picture') }}
                                        </v-btn>
                                        <v-btn size="small" class="ms-2" :disabled="loading || submitting"
                                               v-if="selectedAccount.creditCardPictureUrl"
                                               @click="removeAccountCreditCardPicture(selectedAccount)">
                                            {{ tt('Remove Picture') }}
                                        </v-btn>
                                    </div>
                                </v-col>
                                <!-- Credit limit and billing cycle -->
                                <v-col cols="12" md="6" v-if="currentAccountIndex < 0 && account.category === AccountCategory.CreditCard.type">
                                    <amount-input :disabled="loading || submitting"
                                                  :persistent-placeholder="true"
                                                  :currency="account.currency"
                                                  :show-currency="true"
                                                  :label="tt('Credit Limit')"
                                                  :placeholder="tt('Credit Limit')"
                                                  v-model="account.numericCreditCardLimit"
                                                  v-if="account.currency && account.currency !== ACCOUNT_CURRENCY_NOT_SET_VALUE" />
                                    <v-text-field disabled
                                                  persistent-placeholder
                                                  type="text"
                                                  :label="tt('Credit Limit')"
                                                  :placeholder="tt('Credit Limit')"
                                                  :model-value="getAccountCreditCardCreditLimitDisplayValue(account.numericCreditCardLimit, account.currency)"
                                                  v-else-if="!account.currency || account.currency === ACCOUNT_CURRENCY_NOT_SET_VALUE" />
                                </v-col>
                                <v-col cols="12" md="6" v-if="currentAccountIndex < 0 && account.category === AccountCategory.CreditCard.type">
                                    <v-switch :disabled="loading || submitting"
                                              :label="tt('Physical Card')"
                                              :hint="tt('Turn off if this account is only a shared credit limit container')"
                                              persistent-hint
                                              :model-value="account.creditCardIsEntityCard !== false"
                                              @update:model-value="account.creditCardIsEntityCard = $event !== false" />
                                </v-col>
                                <v-col cols="12" md="6" v-if="account.category === AccountCategory.CreditCard.type">
                                    <v-text-field
                                        type="text"
                                        autocomplete="off"
                                        persistent-placeholder
                                        :disabled="loading || submitting"
                                        :label="tt('Bank limit in this card currency')"
                                        :placeholder="tt('Bank limit in this card currency')"
                                        :hint="tt('The same shared pool limit stated by the bank in this card currency')"
                                        persistent-hint
                                        v-model="selectedAccount.creditCardCurrencyLimit"
                                    />
                                </v-col>
                                <v-col cols="12" md="6" v-if="account.category === AccountCategory.CreditCard.type">
                                    <v-autocomplete
                                        item-title="displayName"
                                        item-value="type"
                                        auto-select-first
                                        persistent-placeholder
                                        :disabled="loading || submitting"
                                        :label="tt('Statement Date')"
                                        :placeholder="tt('Statement Date')"
                                        :items="allAvailableMonthDays"
                                        :no-data-text="tt('No results')"
                                        v-model="selectedAccount.creditCardStatementDate"
                                    ></v-autocomplete>
                                </v-col>
                                <v-col cols="12" md="6" v-if="account.category === AccountCategory.CreditCard.type">
                                    <v-switch :disabled="loading || submitting"
                                              :label="tt('Include Statement Date Transactions')"
                                              :hint="tt('When enabled, transactions on the statement date are included in the current bill; otherwise they are included in the next bill')"
                                              persistent-hint
                                              v-model="selectedAccount.creditCardStatementDateIncludeTransaction"/>
                                </v-col>
                                <v-col cols="12" md="6" v-if="account.category === AccountCategory.CreditCard.type">
                                    <v-autocomplete
                                        item-title="displayName"
                                        item-value="type"
                                        auto-select-first
                                        persistent-placeholder
                                        :disabled="loading || submitting"
                                        :label="tt('Final Repayment Date')"
                                        :placeholder="tt('Final Repayment Date')"
                                        :items="allAvailableRepaymentDateTypes"
                                        :no-data-text="tt('No results')"
                                        v-model="selectedAccount.creditCardRepaymentDateType"
                                    ></v-autocomplete>
                                </v-col>
                                <v-col cols="12" md="6" v-if="account.category === AccountCategory.CreditCard.type && selectedAccount.creditCardRepaymentDateType === 1">
                                    <v-autocomplete
                                        item-title="displayName"
                                        item-value="type"
                                        auto-select-first
                                        persistent-placeholder
                                        :disabled="loading || submitting"
                                        :label="tt('Days After Statement Date')"
                                        :placeholder="tt('Days After Statement Date')"
                                        :items="allAvailableRepaymentDays"
                                        :no-data-text="tt('No results')"
                                        v-model="selectedAccount.creditCardRepaymentDaysAfterStatement"
                                    ></v-autocomplete>
                                </v-col>
                                <v-col cols="12" md="6" v-if="account.category === AccountCategory.CreditCard.type && selectedAccount.creditCardRepaymentDateType === 2">
                                    <v-autocomplete
                                        item-title="displayName"
                                        item-value="type"
                                        auto-select-first
                                        persistent-placeholder
                                        :disabled="loading || submitting"
                                        :label="tt('Repayment Day of Month')"
                                        :placeholder="tt('Repayment Day of Month')"
                                        :items="allAvailableMonthDays"
                                        :no-data-text="tt('No results')"
                                        v-model="selectedAccount.creditCardRepaymentDayOfMonth"
                                    ></v-autocomplete>
                                </v-col>
                                <!-- Credit card email notifications -->
                                <v-col cols="12" md="6" v-if="account.category === AccountCategory.CreditCard.type">
                                    <div class="text-subtitle-2 mb-2">{{ tt('Email Notifications') }}</div>
                                    <v-switch :disabled="loading || submitting"
                                              :label="tt('Statement Date Reminder')"
                                              :hint="tt('Send an email on each statement date')"
                                              persistent-hint
                                              v-model="selectedAccount.creditCardStatementReminderEnabled"/>
                                    <v-switch class="mt-2" :disabled="loading || submitting"
                                              :label="tt('Statement Email')"
                                              :hint="tt('Send an email containing all transactions of the just-ended billing cycle')"
                                              persistent-hint
                                              v-model="selectedAccount.creditCardStatementEmailEnabled"/>
                                    <v-switch class="mt-2" :disabled="loading || submitting"
                                              :label="tt('Repayment Date Reminder')"
                                              :hint="tt('Send an email before and on each repayment date')"
                                              persistent-hint
                                              v-model="selectedAccount.creditCardRepaymentReminderEnabled"/>
                                </v-col>
                                <v-col cols="12" md="6" v-if="account.category === AccountCategory.CreditCard.type && selectedAccount.creditCardRepaymentReminderEnabled">
                                    <v-autocomplete
                                        item-title="displayName"
                                        item-value="type"
                                        auto-select-first
                                        persistent-placeholder
                                        :disabled="loading || submitting"
                                        :label="tt('Days Before Repayment Date')"
                                        :placeholder="tt('Days Before Repayment Date')"
                                        :items="allAvailableRepaymentReminderDaysBefore"
                                        :no-data-text="tt('No results')"
                                        :model-value="getAccountRepaymentReminderDaysBefore(selectedAccount)"
                                        @update:model-value="setAccountRepaymentReminderDaysBefore(selectedAccount, $event)"
                                    ></v-autocomplete>
                                </v-col>
                                <v-col cols="12" md="6" v-if="account.category === AccountCategory.CreditCard.type && hasAccountCreditCardNotificationEnabled(selectedAccount)">
                                    <v-autocomplete
                                        item-title="displayName"
                                        item-value="type"
                                        auto-select-first
                                        persistent-placeholder
                                        :disabled="loading || submitting"
                                        :label="tt('Notification Time')"
                                        :placeholder="tt('Notification Time')"
                                        :items="allAvailableNotificationTimes"
                                        :no-data-text="tt('No results')"
                                        :hint="tt('Notifications are sent at this time in your timezone') + ' (' + notificationTimezoneName + ')'"
                                        persistent-hint
                                        :model-value="getAccountNotificationMinutesOfDay(selectedAccount)"
                                        @update:model-value="setAccountNotificationMinutesOfDay(selectedAccount, $event)"
                                    ></v-autocomplete>
                                </v-col>
                                <!-- Other -->
                                <v-col cols="12" md="12">
                                    <v-textarea
                                        type="text"
                                        autocomplete="off"
                                        persistent-placeholder
                                        rows="3"
                                        :disabled="loading || submitting"
                                        :label="tt('Description')"
                                        :placeholder="currentAccountIndex < 0 ? tt('Your account description (optional)') : tt('Your sub-account description (optional)')"
                                        v-model="selectedAccount.comment"
                                    />
                                </v-col>
                                <v-col class="py-0 my-n2" cols="12" md="12" v-if="editAccountId && !isNewAccount(selectedAccount)">
                                    <v-switch :disabled="loading || submitting"
                                              :label="tt('Visible')" v-model="selectedAccount.visible"/>
                                </v-col>
                            </v-row>
                        </v-form>
                    </v-window-item>
                </v-window>
            </template>

            <template #footer>
                <v-spacer/>
                <v-tooltip :disabled="!inputIsEmpty" :text="inputEmptyProblemMessage ? tt(inputEmptyProblemMessage) : ''">
                    <template v-slot:activator="{ props }">
                        <div v-bind="props" class="d-inline-block">
                            <v-btn :disabled="inputIsEmpty || loading || submitting" @click="save">
                                {{ tt(saveButtonTitle) }}
                                <v-progress-circular indeterminate size="22" class="ms-2" v-if="submitting"></v-progress-circular>
                            </v-btn>
                        </div>
                    </template>
                </v-tooltip>
            </template>
        </two-column-dialog-layout>
    </v-dialog>

    <confirm-dialog ref="confirmDialog"/>
    <snack-bar ref="snackbar" />
    <input ref="creditCardPictureInput" type="file" style="display: none" :accept="SUPPORTED_IMAGE_EXTENSIONS" @change="onUploadCreditCardPicture($event)" />
</template>

<script setup lang="ts">
import ConfirmDialog from '@/components/desktop/ConfirmDialog.vue';
import SnackBar from '@/components/desktop/SnackBar.vue';

import { ref, computed, useTemplateRef, watch } from 'vue';

import { useI18n } from '@/locales/helpers.ts';
import { useAccountEditPageBase } from '@/views/base/accounts/AccountEditPageBase.ts';

import { useUserStore } from '@/stores/user.ts';
import { useAccountsStore } from '@/stores/account.ts';

import { itemAndIndex } from '@/core/base.ts';
import { AccountType, AccountCategory } from '@/core/account.ts';
import { ALL_ACCOUNT_ICONS } from '@/consts/icon.ts';
import { ALL_ACCOUNT_COLORS } from '@/consts/color.ts';
import { ACCOUNT_CURRENCY_NOT_SET_VALUE } from '@/consts/currency.ts';
import { SUPPORTED_IMAGE_EXTENSIONS } from '@/consts/file.ts';

import { Account } from '@/models/account.ts';

import { isNumber, isEquals } from '@/lib/common.ts';
import { getCurrentUnixTime } from '@/lib/datetime.ts';
import { generateRandomUUID } from '@/lib/misc.ts';

import {
    mdiPlus,
    mdiDeleteOutline
} from '@mdi/js';

interface AccountEditResponse {
    message: string;
}

type ConfirmDialogType = InstanceType<typeof ConfirmDialog>;
type SnackBarType = InstanceType<typeof SnackBar>;

const { tt } = useI18n();
const {
    defaultAccountCategory,
    editAccountId,
    clientSessionId,
    loading,
    submitting,
    account,
    subAccounts,
    useLastReconciledTime,
    title,
    saveButtonTitle,
    inputEmptyProblemMessage,
    inputIsEmpty,
    allAccountCategories,
    allAccountTypes,
    allAvailableMonthDays,
    allAvailableExpireMonths,
    allAvailableExpireYears,
    allAvailableRepaymentDateTypes,
    allAvailableRepaymentDays,
    allAvailableRepaymentReminderDaysBefore,
    allAvailableNotificationTimes,
    hasAccountCreditCardNotificationEnabled,
    getAccountNotificationMinutesOfDay,
    setAccountNotificationMinutesOfDay,
    getAccountRepaymentReminderDaysBefore,
    setAccountRepaymentReminderDaysBefore,
    notificationTimezoneName,
    getCurrentUnixTimeForNewAccount,
    getDefaultTimezoneOffsetMinutes,
    getAccountCreditCardCreditLimitDisplayValue,
    updateAccountBalanceTime,
    updateAccountLastReconciledTime,
    fillCreditCardNotificationSettings,
    updateAccountCreditCardPicture,
    removeAccountCreditCardPicture,
    discardUploadedUnboundCreditCardPicture,
    confirmUploadedCreditCardPicture,
    isNewAccount,
    addSubAccount,
    setAccount
} = useAccountEditPageBase();

const userStore = useUserStore();
const accountsStore = useAccountsStore();

const confirmDialog = useTemplateRef<ConfirmDialogType>('confirmDialog');
const snackbar = useTemplateRef<SnackBarType>('snackbar');
const creditCardPictureInput = useTemplateRef<HTMLInputElement>('creditCardPictureInput');

let resolveFunc: ((value: AccountEditResponse) => void) | null = null;
let rejectFunc: ((reason?: unknown) => void) | null = null;

const showState = ref<boolean>(false);
const showAdvancedCardDetails = ref<boolean>(false);
const activeTab = ref<string>('account');
const initAccountCategory = ref<AccountCategory>(defaultAccountCategory);
const initAccount = ref<Account | null>(null);
const currentAccountIndex = ref<number>(-1);

const canShowBalanceTime = computed<boolean>(() => (!editAccountId.value || isNewAccount(selectedAccount.value)) && (account.value.type === AccountType.SingleAccount.type || currentAccountIndex.value >= 0));
const canShowLastReconciledTime = computed<boolean>(() => useLastReconciledTime.value && !!editAccountId.value && !isNewAccount(selectedAccount.value) && (account.value.type === AccountType.SingleAccount.type || currentAccountIndex.value >= 0));

const selectedAccount = computed<Account>(() => {
    if (currentAccountIndex.value < 0) {
        return account.value;
    }

    return subAccounts.value[currentAccountIndex.value] as Account;
});

const accountAmountTitle = computed<string>(() => {
    if (currentAccountIndex.value < 0) {
        return account.value.isLiability ? tt('Account Outstanding Balance') : tt('Account Balance');
    } else {
        return account.value.isLiability ? tt('Sub-account Outstanding Balance') : tt('Sub-account Balance');
    }
});

const isAccountModified = computed<boolean>(() => {
    if (!editAccountId.value) { // Add
        return !!initAccount.value && !isEquals(account.value.toCreateRequest(clientSessionId.value, subAccounts.value), initAccount.value.toCreateRequest(clientSessionId.value, initAccount.value.subAccounts));
    } else { // Edit
        return !!initAccount.value && !isEquals(account.value.toModifyRequest(clientSessionId.value, subAccounts.value), initAccount.value.toModifyRequest(clientSessionId.value, initAccount.value.subAccounts));
    }
});

function open(options?: { id?: string, currentAccount?: Account, category?: number }): Promise<AccountEditResponse> {
    showState.value = true;
    loading.value = true;
    submitting.value = false;
    discardUploadedUnboundCreditCardPicture();

    if (isNumber(options?.category) && AccountCategory.valueOf(options.category)) {
        initAccountCategory.value = AccountCategory.valueOf(options.category)!;
    }

    initAccount.value = Account.createNewAccount(initAccountCategory.value, userStore.currentUserDefaultCurrency, getCurrentUnixTimeForNewAccount());
    account.value.fillFrom(initAccount.value);
    subAccounts.value = [];
    currentAccountIndex.value = -1;
    clientSessionId.value = generateRandomUUID();

    if (options && options.id) {
        if (options.currentAccount) {
            setAccount(options.currentAccount);
        }

        editAccountId.value = options.id;
        accountsStore.getAccount({
            accountId: editAccountId.value
        }).then(response => {
            setAccount(response);
            initAccount.value = Account.of(response);
            loading.value = false;
        }).catch(error => {
            loading.value = false;
            showState.value = false;

            if (!error.processed) {
                if (rejectFunc) {
                    rejectFunc(error);
                }
            }
        });
    } else {
        if (options && isNumber(options.category)) {
            initAccount.value.category = options.category;
            initAccount.value.setSuitableIcon(1, options.category);

            account.value.category = initAccount.value.category;
            account.value.icon = initAccount.value.icon;
        }

        editAccountId.value = null;
        loading.value = false;
    }

    return new Promise<AccountEditResponse>((resolve, reject) => {
        resolveFunc = resolve;
        rejectFunc = reject;
    });
}

function save(): void {
    const problemMessage = inputEmptyProblemMessage.value;

    if (problemMessage) {
        snackbar.value?.showMessage(problemMessage);
        return;
    }

    submitting.value = true;
    fillCreditCardNotificationSettings();

    accountsStore.saveAccount({
        account: account.value,
        subAccounts: subAccounts.value,
        isEdit: !!editAccountId.value,
        clientSessionId: clientSessionId.value
    }).then(() => {
        submitting.value = false;
        confirmUploadedCreditCardPicture();

        let message = 'You have saved this account';

        if (!editAccountId.value) {
            message = 'You have added a new account';
        }

        resolveFunc?.({ message });
        showState.value = false;
    }).catch(error => {
        submitting.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function removeSubAccount(currentSubAccount: Account): void {
    confirmDialog.value?.open('Are you sure you want to remove this sub-account?').then(() => {
        for (const [subAccount, index] of itemAndIndex(subAccounts.value)) {
            if (subAccount === currentSubAccount) {
                subAccounts.value.splice(index, 1);

                if (currentAccountIndex.value >= subAccounts.value.length) {
                    currentAccountIndex.value = subAccounts.value.length - 1;
                }
            }
        }
    });
}

function cancel(): void {
    discardUploadedUnboundCreditCardPicture();
    rejectFunc?.();
    showState.value = false;
}

const pictureTargetAccount = ref<Account | null>(null);

function showOpenCreditCardPictureDialog(targetAccount?: Account): void {
    if (loading.value || submitting.value) {
        return;
    }

    pictureTargetAccount.value = targetAccount || account.value;
    creditCardPictureInput.value?.click();
}

function onUploadCreditCardPicture(event: Event): void {
    if (!event || !event.target) {
        return;
    }

    const el = event.target as HTMLInputElement;

    if (!el.files || !el.files.length || !el.files[0]) {
        return;
    }

    const pictureFile = el.files[0] as File;

    el.value = '';
    updateAccountCreditCardPicture(pictureFile, pictureTargetAccount.value || account.value).catch(error => {
        snackbar.value?.showError(error);
    });
}

function onShowDateTimeError(error: string): void {
    snackbar.value?.showError(error);
}

watch(() => account.value.type, (newValue) => {
    if (subAccounts.value.length < 1) {
        addSubAccount();
    }

    if (newValue === AccountType.SingleAccount.type) {
        currentAccountIndex.value = -1;
    }
});

defineExpose({
    open
});
</script>
