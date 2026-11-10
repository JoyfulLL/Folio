import { ref, computed, watch } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useSettingsStore } from '@/stores/setting.ts';
import { useUserStore } from '@/stores/user.ts';
import { useAccountsStore } from '@/stores/account.ts';

import type { TypeAndDisplayName } from '@/core/base.ts';
import { AccountCategory, AccountType } from '@/core/account.ts';
import type { LocalizedAccountCategory } from '@/core/account.ts';
import { ImageUploadQualityType } from '@/core/image.ts';
import { KnownFileType } from '@/core/file.ts';
import { ACCOUNT_CURRENCY_NOT_SET_VALUE } from '@/consts/currency.ts';

import { Account } from '@/models/account.ts';

import { isDefined, isNumber } from '@/lib/common.ts';
import logger from '@/lib/logger.ts';
import { parseBigDecimal } from '@/lib/numeral.ts';
import { compressJpgImageByQuality } from '@/lib/ui/common.ts';
import {
    getTimezoneOffsetMinutes,
    getSameDateTimeWithCurrentTimezone,
    parseDateTimeFromUnixTimeWithBrowserTimezone,
    getBrowserTimezoneName,
    getBrowserTimezoneOffsetMinutes,
    getCurrentUnixTime
} from '@/lib/datetime.ts';

const DEFAULT_CREDIT_CARD_NOTIFICATION_MINUTES_OF_DAY = 9 * 60;
const CREDIT_CARD_NOTIFICATION_TIME_STEP_MINUTES = 5;
const MINUTES_PER_DAY = 24 * 60;

interface UploadedUnboundCreditCardPicture {
    account: Account;
    pictureId: string;
}

export function useAccountEditPageBase() {
    const {
        tt,
        getAvailableMonthDays,
        getAllLongMonthNames,
        getAllAccountCategories,
        getAllAccountTypes,
        formatAmountToLocalizedNumeralsWithCurrency
    } = useI18n();

    const settingsStore = useSettingsStore();
    const userStore = useUserStore();
    const accountsStore = useAccountsStore();

    const defaultAccountCategory = AccountCategory.values(settingsStore.appSettings.accountCategoryOrders)[0] ?? AccountCategory.Default;

    const editAccountId = ref<string | null>(null);
    const clientSessionId = ref<string>('');
    const loading = ref<boolean>(false);
    const submitting = ref<boolean>(false);
    const account = ref<Account>(Account.createNewAccount(defaultAccountCategory, userStore.currentUserDefaultCurrency, getCurrentUnixTimeForNewAccount()));
    const subAccounts = ref<Account[]>([]);
    const uploadedUnboundCreditCardPictures = ref<UploadedUnboundCreditCardPicture[]>([]);

    const useLastReconciledTime = computed(() => userStore.currentUserUseLastReconciledTime);

    const title = computed<string>(() => {
        if (!editAccountId.value) {
            return 'Add Account';
        } else {
            return 'Edit Account';
        }
    });

    const saveButtonTitle = computed<string>(() => {
        if (!editAccountId.value) {
            return 'Add';
        } else {
            return 'Save';
        }
    });

    const inputEmptyProblemMessage = computed<string | null>(() => {
        let problemMessage = getInputEmptyProblemMessage(account.value, false);

        if (problemMessage) {
            return problemMessage;
        }

        if (account.value.type === AccountType.MultiSubAccounts.type) {
            for (const subAccount of subAccounts.value) {
                problemMessage = getInputEmptyProblemMessage(subAccount, true);

                if (problemMessage) {
                    return problemMessage;
                }
            }
        }

        return null;
    });

    const inputIsEmpty = computed<boolean>(() => !!inputEmptyProblemMessage.value);

    const customAccountCategoryOrder = computed<string>(() => settingsStore.appSettings.accountCategoryOrders);
    const allAccountCategories = computed<LocalizedAccountCategory[]>(() => getAllAccountCategories(customAccountCategoryOrder.value));
    const allAccountTypes = computed<TypeAndDisplayName[]>(() => getAllAccountTypes());

    const allAvailableMonthDays = computed<TypeAndDisplayName[]>(() => {
        const allAvailableDays: TypeAndDisplayName[] = getAvailableMonthDays(28);

        allAvailableDays.splice(0, 0, {
            type: 0,
            displayName: tt('Not set'),
        });

        return allAvailableDays;
    });

    const allAvailableExpireMonths = computed<TypeAndDisplayName[]>(() => {
        const allAvailableMonths: TypeAndDisplayName[] = [{
            type: 0,
            displayName: tt('Not set'),
        }];

        const monthNames = getAllLongMonthNames();

        for (let i = 0; i < monthNames.length; i++) {
            allAvailableMonths.push({
                type: i + 1,
                displayName: monthNames[i] as string
            });
        }

        return allAvailableMonths;
    });

    const allAvailableExpireYears = computed<TypeAndDisplayName[]>(() => {
        const currentYear = parseDateTimeFromUnixTimeWithBrowserTimezone(getCurrentUnixTime()).getGregorianCalendarYear();
        const allAvailableYears: TypeAndDisplayName[] = [{
            type: 0,
            displayName: tt('Not set'),
        }];

        for (let year = currentYear - 10; year <= currentYear + 20; year++) {
            allAvailableYears.push({
                type: year,
                displayName: year.toString(10)
            });
        }

        return allAvailableYears;
    });

    const allAvailableRepaymentDateTypes = computed<TypeAndDisplayName[]>(() => [{
        type: 0,
        displayName: tt('Not set'),
    }, {
        type: 1,
        displayName: tt('Days After Statement Date'),
    }, {
        type: 2,
        displayName: tt('Fixed Day Each Month'),
    }]);

    const allAvailableRepaymentDays = computed<TypeAndDisplayName[]>(() => {
        const allAvailableDays: TypeAndDisplayName[] = [{
            type: 0,
            displayName: tt('Not set'),
        }];

        for (let day = 1; day <= 99; day++) {
            allAvailableDays.push({
                type: day,
                displayName: day.toString(10)
            });
        }

        return allAvailableDays;
    });

    const allAvailableRepaymentReminderDaysBefore = computed<TypeAndDisplayName[]>(() => {
        const allAvailableDays: TypeAndDisplayName[] = [{
            type: 0,
            displayName: tt('None'),
        }];

        for (let day = 1; day <= 99; day++) {
            allAvailableDays.push({
                type: day,
                displayName: day.toString(10)
            });
        }

        return allAvailableDays;
    });

    const allAvailableNotificationTimes = computed<TypeAndDisplayName[]>(() => {
        const allAvailableTimes: TypeAndDisplayName[] = [];

        for (let minutes = 0; minutes < MINUTES_PER_DAY; minutes += CREDIT_CARD_NOTIFICATION_TIME_STEP_MINUTES) {
            allAvailableTimes.push({
                type: minutes,
                displayName: formatCreditCardNotificationTime(minutes)
            });
        }

        return allAvailableTimes;
    });

    const notificationMinutesOfDay = computed<number>({
        get: () => {
            const minutesOfDay = account.value.creditCardNotificationMinutesOfDay;

            if (!isNumber(minutesOfDay) || minutesOfDay < 0 || minutesOfDay >= MINUTES_PER_DAY) {
                return DEFAULT_CREDIT_CARD_NOTIFICATION_MINUTES_OF_DAY;
            }

            return minutesOfDay;
        },
        set: (minutesOfDay: number) => {
            account.value.creditCardNotificationMinutesOfDay = minutesOfDay;
        }
    });

    const notificationRepaymentDaysBefore = computed<number>({
        get: () => {
            const daysBefore = account.value.creditCardRepaymentReminderDaysBefore;

            return isNumber(daysBefore) && daysBefore >= 0 ? daysBefore : 0;
        },
        set: (daysBefore: number) => {
            account.value.creditCardRepaymentReminderDaysBefore = daysBefore;
        }
    });

    const hasCreditCardNotificationEnabled = computed<boolean>(() => {
        if (account.value.category !== AccountCategory.CreditCard.type) {
            return false;
        }

        return !!account.value.creditCardStatementReminderEnabled ||
            !!account.value.creditCardStatementEmailEnabled ||
            !!account.value.creditCardRepaymentReminderEnabled;
    });

    const notificationTimezoneName = computed<string>(() => getBrowserTimezoneName());

    function formatCreditCardNotificationTime(minutesOfDay: number): string {
        const hours = Math.floor(minutesOfDay / 60);
        const minutes = minutesOfDay % 60;

        return `${hours.toString(10).padStart(2, '0')}:${minutes.toString(10).padStart(2, '0')}`;
    }

    function getAccountCreditCardNotificationTime(minutesOfDay?: number): string {
        if (!isNumber(minutesOfDay) || minutesOfDay < 0 || minutesOfDay >= MINUTES_PER_DAY) {
            return formatCreditCardNotificationTime(DEFAULT_CREDIT_CARD_NOTIFICATION_MINUTES_OF_DAY);
        }

        return formatCreditCardNotificationTime(minutesOfDay);
    }

    function hasAccountCreditCardNotificationEnabled(targetAccount: Account): boolean {
        if (targetAccount.category !== AccountCategory.CreditCard.type) {
            return false;
        }

        return !!targetAccount.creditCardStatementReminderEnabled ||
            !!targetAccount.creditCardStatementEmailEnabled ||
            !!targetAccount.creditCardRepaymentReminderEnabled;
    }

    function getAccountNotificationMinutesOfDay(targetAccount: Account): number {
        const minutesOfDay = targetAccount.creditCardNotificationMinutesOfDay;

        if (!isNumber(minutesOfDay) || minutesOfDay < 0 || minutesOfDay >= MINUTES_PER_DAY) {
            return DEFAULT_CREDIT_CARD_NOTIFICATION_MINUTES_OF_DAY;
        }

        return minutesOfDay;
    }

    function setAccountNotificationMinutesOfDay(targetAccount: Account, minutesOfDay: number): void {
        targetAccount.creditCardNotificationMinutesOfDay = minutesOfDay;
    }

    function getAccountRepaymentReminderDaysBefore(targetAccount: Account): number {
        const daysBefore = targetAccount.creditCardRepaymentReminderDaysBefore;

        return isNumber(daysBefore) && daysBefore >= 0 ? daysBefore : 0;
    }

    function setAccountRepaymentReminderDaysBefore(targetAccount: Account, daysBefore: number): void {
        targetAccount.creditCardRepaymentReminderDaysBefore = daysBefore;
    }

    // every physical card (including sub-accounts) needs its own notification time and timezone,
    // otherwise the backend rejects the request when the card has notification enabled
    function fillCreditCardNotificationSettings(): void {
        const allAccounts: Account[] = [account.value, ...subAccounts.value];
        const timezoneName = getBrowserTimezoneName();
        const timezoneUtcOffset = getBrowserTimezoneOffsetMinutes(getCurrentUnixTime());

        for (const targetAccount of allAccounts) {
            if (!hasAccountCreditCardNotificationEnabled(targetAccount)) {
                continue;
            }

            if (!isNumber(targetAccount.creditCardNotificationMinutesOfDay)) {
                targetAccount.creditCardNotificationMinutesOfDay = DEFAULT_CREDIT_CARD_NOTIFICATION_MINUTES_OF_DAY;
            }

            if (timezoneName) {
                targetAccount.creditCardNotificationTimezoneName = timezoneName;
                targetAccount.creditCardNotificationTimezoneUtcOffset = timezoneUtcOffset;
            }
        }
    }

    function getCurrentUnixTimeForNewAccount(): number {
        return getSameDateTimeWithCurrentTimezone(parseDateTimeFromUnixTimeWithBrowserTimezone(getCurrentUnixTime())).getUnixTime();
    }

    function getDefaultTimezoneOffsetMinutes(unixTime?: number): number {
        if (!unixTime) {
            return getTimezoneOffsetMinutes(getCurrentUnixTime());
        }

        return getTimezoneOffsetMinutes(unixTime);
    }

    function getAccountCreditCardStatementDate(statementDate?: number): string | null {
        if (!isDefined(statementDate)) {
            return tt('Not set');
        }

        for (const item of allAvailableMonthDays.value) {
            if (item.type === statementDate) {
                return item.displayName;
            }
        }

        return null;
    }

    function getAccountCreditCardExpireMonth(expireMonth?: number): string {
        if (!isDefined(expireMonth) || !expireMonth) {
            return tt('Not set');
        }

        for (const item of allAvailableExpireMonths.value) {
            if (item.type === expireMonth) {
                return item.displayName;
            }
        }

        return expireMonth.toString(10);
    }

    function getAccountCreditCardExpireYear(expireYear?: number): string {
        if (!isDefined(expireYear) || !expireYear) {
            return tt('Not set');
        }

        return expireYear.toString(10);
    }

    function getAccountCreditCardRepaymentDateType(repaymentDateType?: number): string {
        if (!isDefined(repaymentDateType) || !repaymentDateType) {
            return tt('Not set');
        }

        for (const item of allAvailableRepaymentDateTypes.value) {
            if (item.type === repaymentDateType) {
                return item.displayName;
            }
        }

        return tt('Not set');
    }

    function getAccountCreditCardRepaymentDaysAfterStatement(daysAfterStatement?: number): string {
        if (!isDefined(daysAfterStatement) || !daysAfterStatement) {
            return tt('Not set');
        }

        return daysAfterStatement.toString(10);
    }

    function getAccountCreditCardRepaymentDayOfMonth(dayOfMonth?: number): string | null {
        return getAccountCreditCardStatementDate(dayOfMonth);
    }

    function updateAccountCreditCardPicture(file: File, targetAccount?: Account): Promise<void> {
        const card = targetAccount || account.value;

        return compressJpgImageByQuality(file, ImageUploadQualityType.HD720P).then(blob => {
            return accountsStore.uploadCreditCardPicture({
                pictureFile: KnownFileType.JPG.createFileFromBlob(blob, 'credit_card_picture')
            });
        }).then(pictureInfo => {
            discardUploadedUnboundCreditCardPicture(card);

            card.creditCardPictureId = pictureInfo.pictureId;
            card.creditCardPictureUrl = pictureInfo.originalUrl;
            uploadedUnboundCreditCardPictures.value.push({ account: card, pictureId: pictureInfo.pictureId });
        });
    }

    function removeAccountCreditCardPicture(targetAccount?: Account): void {
        const card = targetAccount || account.value;

        card.creditCardPictureId = undefined;
        card.creditCardPictureUrl = undefined;
        discardUploadedUnboundCreditCardPicture(card);
    }

    // an uploaded picture is not bound to any account until the account is saved, so it must be removed once it is replaced or discarded
    function discardUploadedUnboundCreditCardPicture(targetAccount?: Account): void {
        const remaining: UploadedUnboundCreditCardPicture[] = [];

        for (const item of uploadedUnboundCreditCardPictures.value) {
            if (targetAccount && item.account !== targetAccount) {
                remaining.push(item);
                continue;
            }

            accountsStore.removeUnusedCreditCardPicture({ pictureId: item.pictureId }).catch(error => {
                logger.warn('failed to remove the uploaded credit card picture', error);
            });
        }

        uploadedUnboundCreditCardPictures.value = remaining;
    }

    function confirmUploadedCreditCardPicture(): void {
        uploadedUnboundCreditCardPictures.value = [];
    }

    function getAccountCreditCardCreditLimitDisplayValue(creditLimit: number | undefined, currency: string): string {
        if (!isNumber(creditLimit) || creditLimit <= 0 || !currency || currency === ACCOUNT_CURRENCY_NOT_SET_VALUE) {
            return tt('Not set');
        }

        return formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(creditLimit), currency);
    }

    function updateAccountBalanceTime(account: Account, balanceTime: number): void {
        if (!isDefined(account.balanceTime)) {
            account.balanceTime = balanceTime;
            return;
        }

        const oldUtcOffset = getTimezoneOffsetMinutes(account.balanceTime);
        const newUtcOffset = getTimezoneOffsetMinutes(balanceTime);

        if (oldUtcOffset === newUtcOffset) {
            account.balanceTime = balanceTime;
            return;
        }

        account.balanceTime = balanceTime - (newUtcOffset - oldUtcOffset) * 60;
    }

    function updateAccountLastReconciledTime(account: Account, lastReconciledTime: number): void {
        if (!isDefined(account.lastReconciledTime)) {
            account.lastReconciledTime = lastReconciledTime;
            return;
        }

        const oldUtcOffset = getTimezoneOffsetMinutes(account.lastReconciledTime);
        const newUtcOffset = getTimezoneOffsetMinutes(lastReconciledTime);

        if (oldUtcOffset === newUtcOffset) {
            account.lastReconciledTime = lastReconciledTime;
            return;
        }

        account.lastReconciledTime = lastReconciledTime - (newUtcOffset - oldUtcOffset) * 60;
    }

    function getInputEmptyProblemMessage(account: Account, isSubAccount: boolean): string | null {
        if (!isSubAccount && !account.category) {
            return 'Account category cannot be blank';
        } else if (!isSubAccount && !account.type) {
            return 'Account type cannot be blank';
        } else if (!account.name) {
            return 'Account name cannot be blank';
        } else if (account.type === AccountType.SingleAccount.type && !account.currency) {
            return 'Account currency cannot be blank';
        } else {
            return null;
        }
    }

    function isNewAccount(account: Account): boolean {
        return account.id === '' || account.id === '0';
    }

    function addSubAccount(): boolean {
        if (account.value.type !== AccountType.MultiSubAccounts.type) {
            return false;
        }

        const subAccount = account.value.createNewSubAccount(userStore.currentUserDefaultCurrency, getCurrentUnixTimeForNewAccount());
        subAccounts.value.push(subAccount);
        return true;
    }

    function setAccount(newAccount: Account): void {
        account.value.fillFrom(newAccount);
        subAccounts.value = [];

        if (newAccount.subAccounts && newAccount.subAccounts.length > 0) {
            for (const oldSubAccount of newAccount.subAccounts) {
                const subAccount: Account = account.value.createNewSubAccount(userStore.currentUserDefaultCurrency, getCurrentUnixTimeForNewAccount());
                subAccount.fillFrom(oldSubAccount);

                subAccounts.value.push(subAccount);
            }
        }
    }

    watch(() => account.value.category, (newValue, oldValue) => {
        account.value.setSuitableIcon(oldValue, newValue);
    });

    watch(() => account.value.currency, (newValue) => {
        if (account.value.category === AccountCategory.CreditCard.type && (!newValue || newValue === ACCOUNT_CURRENCY_NOT_SET_VALUE)) {
            account.value.numericCreditCardLimit = 0;
        }
    });

    return {
        // constants
        defaultAccountCategory,
        // states
        editAccountId,
        clientSessionId,
        loading,
        submitting,
        account,
        subAccounts,
        // computed states
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
        notificationMinutesOfDay,
        notificationRepaymentDaysBefore,
        hasCreditCardNotificationEnabled,
        hasAccountCreditCardNotificationEnabled,
        getAccountNotificationMinutesOfDay,
        setAccountNotificationMinutesOfDay,
        getAccountRepaymentReminderDaysBefore,
        setAccountRepaymentReminderDaysBefore,
        notificationTimezoneName,
        // functions
        getCurrentUnixTimeForNewAccount,
        getDefaultTimezoneOffsetMinutes,
        getAccountCreditCardStatementDate,
        getAccountCreditCardExpireMonth,
        getAccountCreditCardExpireYear,
        getAccountCreditCardRepaymentDateType,
        getAccountCreditCardRepaymentDaysAfterStatement,
        getAccountCreditCardRepaymentDayOfMonth,
        getAccountCreditCardNotificationTime,
        fillCreditCardNotificationSettings,
        updateAccountCreditCardPicture,
        removeAccountCreditCardPicture,
        discardUploadedUnboundCreditCardPicture,
        confirmUploadedCreditCardPicture,
        getAccountCreditCardCreditLimitDisplayValue,
        updateAccountBalanceTime,
        updateAccountLastReconciledTime,
        isNewAccount,
        addSubAccount,
        setAccount
    };
}
