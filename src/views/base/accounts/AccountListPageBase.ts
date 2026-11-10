import { ref, computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useSettingsStore } from '@/stores/setting.ts';
import { useUserStore } from '@/stores/user.ts';
import { useAccountsStore } from '@/stores/account.ts';

import type { BigDecimal, HiddenAmount, BigDecimalWithSuffix } from '@/core/numeral.ts';
import type { WeekDayValue } from '@/core/datetime.ts';
import { AccountCategory, AccountType } from '@/core/account.ts';
import { ACCOUNT_CURRENCY_NOT_SET_VALUE } from '@/consts/currency.ts';

import type { Account, CategorizedAccount } from '@/models/account.ts';

import { isDefined, isObject, isString } from '@/lib/common.ts';
import { isBigDecimal, parseBigDecimal } from '@/lib/numeral.ts';

export function useAccountListPageBase() {
    const { formatAmountToLocalizedNumeralsWithCurrency } = useI18n();

    const settingsStore = useSettingsStore();
    const userStore = useUserStore();
    const accountsStore = useAccountsStore();

    const loading = ref<boolean>(true);
    const showHidden = ref<boolean>(false);
    const displayOrderModified = ref<boolean>(false);

    const showAccountBalance = computed<boolean>({
        get: () => settingsStore.appSettings.showAccountBalance,
        set: (value) => settingsStore.setShowAccountBalance(value)
    });

    const customAccountCategoryOrder = computed<string>(() => settingsStore.appSettings.accountCategoryOrders);
    const defaultAccountCategory = computed<AccountCategory>(() => AccountCategory.values(customAccountCategoryOrder.value)[0] ?? AccountCategory.Default);

    const firstDayOfWeek = computed<WeekDayValue>(() => userStore.currentUserFirstDayOfWeek);
    const fiscalYearStart = computed<number>(() => userStore.currentUserFiscalYearStart);
    const defaultCurrency = computed<string>(() => userStore.currentUserDefaultCurrency);
    const useLastReconciledTime = computed(() => userStore.currentUserUseLastReconciledTime);

    const allAccounts = computed<Account[]>(() => accountsStore.allAccounts);
    const allCategorizedAccountsMap = computed<Record<number, CategorizedAccount>>(() => accountsStore.allCategorizedAccountsMap);
    const allAccountCount = computed<number>(() => accountsStore.allAvailableAccountsCount);
    const maxCategoryAccountCount = computed<number>(() => accountsStore.maxCategoryAccountCount);

    const netAssets = computed<string>(() => {
        const netAssets: BigDecimal | HiddenAmount | BigDecimalWithSuffix = accountsStore.getNetAssets(showAccountBalance.value, settingsStore.appSettings.totalAmountExcludeAccountIds);
        return formatAmountToLocalizedNumeralsWithCurrency(netAssets, defaultCurrency.value);
    });

    const totalAssets = computed<string>(() => {
        const totalAssets: BigDecimal | HiddenAmount | BigDecimalWithSuffix = accountsStore.getTotalAssets(showAccountBalance.value, settingsStore.appSettings.totalAmountExcludeAccountIds);
        return formatAmountToLocalizedNumeralsWithCurrency(totalAssets, defaultCurrency.value);
    });

    const totalLiabilities = computed<string>(() => {
        const totalLiabilities: BigDecimal | HiddenAmount | BigDecimalWithSuffix = accountsStore.getTotalLiabilities(showAccountBalance.value, settingsStore.appSettings.totalAmountExcludeAccountIds);
        return formatAmountToLocalizedNumeralsWithCurrency(totalLiabilities, defaultCurrency.value);
    });

    function canShowAvailableCredit(account: Account): boolean {
        return account.category === AccountCategory.CreditCard.type && account.numericCreditCardLimit > 0 && !!account.currency && account.currency !== ACCOUNT_CURRENCY_NOT_SET_VALUE;
    }

    function accountCategoryTotalBalance(accountCategory: AccountCategory | undefined, showAvailableCreditForCreditCard: boolean): string {
        if (!accountCategory) {
            return '';
        }

        const totalBalance: BigDecimal | HiddenAmount | BigDecimalWithSuffix | undefined = accountsStore.getAccountCategoryTotalBalance(showAccountBalance.value, accountCategory, showAvailableCreditForCreditCard);

        if (isDefined(totalBalance)) {
            return formatAmountToLocalizedNumeralsWithCurrency(totalBalance, defaultCurrency.value);
        } else {
            return '';
        }
    }

    function accountBalance(account: Account, currentSubAccountId: string | undefined, showBalance: boolean, onlyShowSelectedAccountIds?: string[]): string | null {
        if (account.type === AccountType.SingleAccount.type) {
            const balance: BigDecimal | HiddenAmount | null = accountsStore.getAccountBalance(showBalance, account);

            if (isBigDecimal(balance) || isString(balance)) {
                return formatAmountToLocalizedNumeralsWithCurrency(balance, account.currency);
            } else {
                return '';
            }
        } else if (account.type === AccountType.MultiSubAccounts.type) {
            const balanceResult = accountsStore.getAccountSubAccountBalance(showBalance, showHidden.value, account, currentSubAccountId, onlyShowSelectedAccountIds);

            if (!isObject(balanceResult)) {
                return '';
            }

            return formatAmountToLocalizedNumeralsWithCurrency(balanceResult.balance, balanceResult.currency);
        } else {
            return null;
        }
    }

    function accountAvailableCredit(account: Account, showBalance: boolean): string | null {
        if (!canShowAvailableCredit(account)) {
            return null;
        }

        const creditCardLimit = parseBigDecimal(account.creditCardLimit);

        if (account.type === AccountType.SingleAccount.type) {
            const balance: BigDecimal | HiddenAmount | null = accountsStore.getAccountBalance(showBalance, account);

            if (isBigDecimal(balance)) {
                return formatAmountToLocalizedNumeralsWithCurrency(creditCardLimit.subtract(balance), account.currency);
            } else if (isString(balance)) {
                return formatAmountToLocalizedNumeralsWithCurrency(balance, account.currency);
            } else {
                return null;
            }
        } else if (account.type === AccountType.MultiSubAccounts.type) {
            // the available credit of a shared-limit pool is the remaining credit of the whole
            // pool, counted once in the pool's currency (foreign-currency card debts excluded
            // and marked incomplete)
            const remainingCreditResult = accountsStore.getCreditCardPoolRemainingCredit(showBalance, account);

            if (!isObject(remainingCreditResult)) {
                return null;
            }

            return formatAmountToLocalizedNumeralsWithCurrency(remainingCreditResult.balance, remainingCreditResult.currency);
        } else {
            return null;
        }
    }

    // Returns the remaining credit of the shared pool expressed in the currency of one of its
    // cards. All cards of a pool share one limit, so a purchase on any card reduces the available
    // credit shown on every card. Returns null when no limit is stated in the card's currency
    function poolCardAvailableCredit(account: Account, subAccount: Account, showBalance: boolean): string | null {
        const remainingCreditResult = accountsStore.getCreditCardPoolCardRemainingCredit(showBalance, account, subAccount);

        if (!isObject(remainingCreditResult)) {
            return null;
        }

        return formatAmountToLocalizedNumeralsWithCurrency(remainingCreditResult.balance, remainingCreditResult.currency);
    }

    // Returns the available credit displayed on a card row of a shared-limit pool: the shared
    // remaining credit in the card's own currency, or the remaining credit of the whole pool
    // when no limit is stated in that currency
    function subAccountAvailableCredit(account: Account, subAccount: Account, showBalance: boolean): string | null {
        const cardRemainingCredit = poolCardAvailableCredit(account, subAccount, showBalance);

        if (cardRemainingCredit !== null) {
            return cardRemainingCredit;
        }

        return accountAvailableCredit(account, showBalance);
    }

    // Returns the available credit displayed on a pool account row: the shared remaining credit
    // in the currency of the selected card, or the remaining credit of the whole pool
    function accountOrSubAccountAvailableCredit(account: Account, currentSubAccountId: string | undefined, showBalance: boolean): string | null {
        if (currentSubAccountId && account.subAccounts) {
            const subAccount = account.getSubAccount(currentSubAccountId);

            if (subAccount) {
                const cardRemainingCredit = poolCardAvailableCredit(account, subAccount, showBalance);

                if (cardRemainingCredit !== null) {
                    return cardRemainingCredit;
                }
            }
        }

        return accountAvailableCredit(account, showBalance);
    }

    function accountBalanceOrAvailableCredit(account: Account, currentSubAccountId: string | undefined, showAvailableCreditForCreditCard: boolean, showBalance: boolean, onlyShowSelectedAccountIds?: string[]): string | null {
        if (showAvailableCreditForCreditCard && account.category === AccountCategory.CreditCard.type) {
            return accountAvailableCredit(account, showBalance);
        } else {
            return accountBalance(account, currentSubAccountId, showBalance, onlyShowSelectedAccountIds);
        }
    }

    return {
        // states
        loading,
        showHidden,
        displayOrderModified,
        // computed states
        showAccountBalance,
        customAccountCategoryOrder,
        defaultAccountCategory,
        firstDayOfWeek,
        fiscalYearStart,
        defaultCurrency,
        useLastReconciledTime,
        allAccounts,
        allCategorizedAccountsMap,
        allAccountCount,
        maxCategoryAccountCount,
        netAssets,
        totalAssets,
        totalLiabilities,
        // functions
        canShowAvailableCredit,
        accountCategoryTotalBalance,
        accountBalance,
        accountAvailableCredit,
        subAccountAvailableCredit,
        accountOrSubAccountAvailableCredit,
        accountBalanceOrAvailableCredit
    };
}
