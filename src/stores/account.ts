import { ref, computed } from 'vue';
import { defineStore } from 'pinia';

import { useSettingsStore } from './setting.ts';
import { useUserStore } from './user.ts';
import { useOverviewStore } from '@/stores/overview.ts';
import { useExchangeRatesStore } from './exchangeRates.ts';

import { type BeforeResolveFunction, itemAndIndex, reversed, entries, values } from '@/core/base.ts';
import type { BigDecimal, HiddenAmount, BigDecimalWithSuffix } from '@/core/numeral.ts';
import { AccountType, AccountCategory } from '@/core/account.ts';
import { DISPLAY_HIDDEN_AMOUNT, INCOMPLETE_AMOUNT_SUFFIX } from '@/consts/numeral.ts';
import { ACCOUNT_CURRENCY_NOT_SET_VALUE } from '@/consts/currency.ts';

import {
    type AccountNewDisplayOrderRequest,
    type AccountDisplayBalance,
    type CategorizedAccount,
    type AccountShowingIds,
    Account
} from '@/models/account.ts';
import type { CreditCardPictureInfoBasicResponse } from '@/models/credit_card_picture_info.ts';

import { isArray, isEquals, arrayItemToObjectField } from '@/lib/common.ts';
import { BIG_DECIMAL_ZERO, parseBigDecimal } from '@/lib/numeral.ts';
import { getCategorizedAccountsMap, getAllFilteredAccountsBalance } from '@/lib/account.ts';
import services from '@/lib/services.ts';
import logger from '@/lib/logger.ts';

export const useAccountsStore = defineStore('accounts', () => {
    const settingsStore = useSettingsStore();
    const userStore = useUserStore();
    const overviewStore = useOverviewStore();
    const exchangeRatesStore = useExchangeRatesStore();

    let loadingPromise: Promise<Account[]> | null = null;

    const allAccounts = ref<Account[]>([]);
    const allAccountsMap = ref<Record<string, Account>>({});
    const allCategorizedAccountsMap = ref<Record<number, CategorizedAccount>>({});
    const accountListStateInvalid = ref<boolean>(true);

    const allPlainAccounts = computed<Account[]>(() => {
        const allAccountsList: Account[] = [];

        for (const account of allAccounts.value) {
            if (account.type === AccountType.SingleAccount.type) {
                allAccountsList.push(account);
            } else if (account.type === AccountType.MultiSubAccounts.type) {
                if (account.subAccounts) {
                    for (const subAccount of account.subAccounts) {
                        allAccountsList.push(subAccount);
                    }
                }
            }
        }

        return Account.sortAccounts(allAccountsList, settingsStore.accountCategoryDisplayOrders, allAccountsMap.value);
    });

    const allMixedPlainAccounts = computed<Account[]>(() => {
        const allAccountsList: Account[] = [];

        for (const account of allAccounts.value) {
            if (account.type === AccountType.SingleAccount.type) {
                allAccountsList.push(account);
            } else if (account.type === AccountType.MultiSubAccounts.type) {
                allAccountsList.push(account);

                if (account.subAccounts) {
                    for (const subAccount of account.subAccounts) {
                        allAccountsList.push(subAccount);
                    }
                }
            }
        }

        return Account.sortAccounts(allAccountsList, settingsStore.accountCategoryDisplayOrders, allAccountsMap.value);
    });

    const allVisiblePlainAccounts = computed<Account[]>(() => {
        const allVisibleAccounts: Account[] = [];

        for (const account of allAccounts.value) {
            if (account.hidden) {
                continue;
            }

            if (account.type === AccountType.SingleAccount.type) {
                allVisibleAccounts.push(account);
            } else if (account.type === AccountType.MultiSubAccounts.type) {
                if (account.subAccounts) {
                    for (const subAccount of account.subAccounts) {

                        if (subAccount.hidden) {
                            continue;
                        }

                        allVisibleAccounts.push(subAccount);
                    }
                }
            }
        }

        return Account.sortAccounts(allVisibleAccounts, settingsStore.accountCategoryDisplayOrders, allAccountsMap.value);
    });

    const allAvailableAccountsCount = computed<number>(() => {
        let allAccountCount = 0;

        for (const categorizedAccounts of values(allCategorizedAccountsMap.value)) {
            allAccountCount += categorizedAccounts.accounts.length;
        }

        return allAccountCount;
    });

    const allVisibleAccountsCount = computed<number>(() => {
        let shownAccountCount = 0;

        for (const categorizedAccounts of values(allCategorizedAccountsMap.value)) {
            const accountList = categorizedAccounts.accounts;

            for (const account of accountList) {
                if (!account.hidden) {
                    shownAccountCount++;
                }
            }
        }

        return shownAccountCount;
    });

    const maxCategoryAccountCount = computed<number>(() => {
        let maxCount = 0;

        for (const categorizedAccounts of values(allCategorizedAccountsMap.value)) {
            const accountCount = categorizedAccounts.accounts.length;

            if (accountCount > maxCount) {
                maxCount = accountCount;
            }
        }

        return maxCount;
    });

    function loadAccountList(accounts: Account[]): void {
        allAccounts.value = accounts;
        allAccountsMap.value = {};

        for (const account of accounts) {
            allAccountsMap.value[account.id] = account;

            if (account.subAccounts) {
                for (const subAccount of account.subAccounts) {
                    allAccountsMap.value[subAccount.id] = subAccount;
                }
            }
        }

        allCategorizedAccountsMap.value = getCategorizedAccountsMap(accounts);
    }

    function addAccountToAccountList(currentAccount: Account): void {
        const newAccountCategory = AccountCategory.valueOf(currentAccount.category);
        let insertIndexToAllList = allAccounts.value.length;

        if (newAccountCategory) {
            for (const [account, index] of itemAndIndex(allAccounts.value)) {
                const accountCategory = AccountCategory.valueOf(account.category);
                const accountCategoryDisplayOrder = settingsStore.accountCategoryDisplayOrders[accountCategory?.type ?? 0] || Number.MAX_SAFE_INTEGER;
                const newAccountCategoryDisplayOrder = settingsStore.accountCategoryDisplayOrders[newAccountCategory.type] || Number.MAX_SAFE_INTEGER;

                if (accountCategory && accountCategoryDisplayOrder > newAccountCategoryDisplayOrder) {
                    insertIndexToAllList = index;
                    break;
                }
            }
        }

        allAccounts.value.splice(insertIndexToAllList, 0, currentAccount);

        allAccountsMap.value[currentAccount.id] = currentAccount;

        if (currentAccount.subAccounts) {
            for (const subAccount of currentAccount.subAccounts) {
                allAccountsMap.value[subAccount.id] = subAccount;
            }
        }

        if (allCategorizedAccountsMap.value[currentAccount.category]) {
            const accountList = allCategorizedAccountsMap.value[currentAccount.category]!.accounts;
            accountList.push(currentAccount);
        } else {
            allCategorizedAccountsMap.value = getCategorizedAccountsMap(allAccounts.value);
        }
    }

    function updateAccountToAccountList(oldAccount: Account, newAccount: Account): void {
        for (const [account, index] of itemAndIndex(allAccounts.value)) {
            if (account.id === newAccount.id) {
                allAccounts.value.splice(index, 1, newAccount);
                break;
            }
        }

        if (oldAccount.subAccounts) {
            for (const subAccount of oldAccount.subAccounts) {
                if (allAccountsMap.value[subAccount.id]) {
                    delete allAccountsMap.value[subAccount.id];
                }
            }
        }

        allAccountsMap.value[newAccount.id] = newAccount;

        if (newAccount.subAccounts) {
            for (const subAccount of newAccount.subAccounts) {
                allAccountsMap.value[subAccount.id] = subAccount;
            }
        }

        if (allCategorizedAccountsMap.value[newAccount.category]) {
            const accountList = allCategorizedAccountsMap.value[newAccount.category]!.accounts;

            for (const [account, index] of itemAndIndex(accountList)) {
                if (account.id === newAccount.id) {
                    accountList.splice(index, 1, newAccount);
                    break;
                }
            }
        }
    }

    function updateAccountDisplayOrderInAccountList({ account, from, to, updateListOrder, updateGlobalListOrder }: { account: Account, from: number, to: number, updateListOrder: boolean, updateGlobalListOrder: boolean }): void {
        let fromAccount = null;
        let toAccount = null;

        if (allCategorizedAccountsMap.value[account.category]) {
            const accountList = allCategorizedAccountsMap.value[account.category]!.accounts;

            if (updateListOrder) {
                fromAccount = accountList[from];
                toAccount = accountList[to];
                accountList.splice(to, 0, accountList.splice(from, 1)[0] as Account);
            } else {
                fromAccount = accountList[to];

                if (from < to) {
                    toAccount = accountList[to - 1];
                } else if (from > to) {
                    toAccount = accountList[to + 1];
                }
            }
        }

        if (updateGlobalListOrder && fromAccount && toAccount) {
            let globalFromIndex = -1;
            let globalToIndex = -1;

            for (const [account, index] of itemAndIndex(allAccounts.value)) {
                if (account.id === fromAccount.id) {
                    globalFromIndex = index;
                } else if (account.id === toAccount.id) {
                    globalToIndex = index;
                }
            }

            if (globalFromIndex >= 0 && globalToIndex >= 0) {
                allAccounts.value.splice(globalToIndex, 0, allAccounts.value.splice(globalFromIndex, 1)[0] as Account);
            }
        }
    }

    function updateAccountVisibilityInAccountList({ account, hidden }: { account: Account, hidden: boolean }): void {
        if (allAccountsMap.value[account.id]) {
            allAccountsMap.value[account.id]!.visible = !hidden;
        }
    }

    function removeAccountFromAccountList(currentAccount: Account): void {
        for (const [account, index] of itemAndIndex(allAccounts.value)) {
            if (account.id === currentAccount.id) {
                allAccounts.value.splice(index, 1);
                break;
            }
        }

        if (allAccountsMap.value[currentAccount.id] && allAccountsMap.value[currentAccount.id]!.subAccounts) {
            const subAccounts = allAccountsMap.value[currentAccount.id]!.subAccounts as Account[];

            for (const subAccount of subAccounts) {
                if (allAccountsMap.value[subAccount.id]) {
                    delete allAccountsMap.value[subAccount.id];
                }
            }
        }

        if (allAccountsMap.value[currentAccount.id]) {
            delete allAccountsMap.value[currentAccount.id];
        }

        if (allCategorizedAccountsMap.value[currentAccount.category]) {
            const accountList = allCategorizedAccountsMap.value[currentAccount.category]!.accounts;

            for (const [account, index] of itemAndIndex(accountList)) {
                if (account.id === currentAccount.id) {
                    accountList.splice(index, 1);
                    break;
                }
            }
        }
    }

    function removeSubAccountFromAccountList(currentSubAccount: Account): void {
        for (const account of allAccounts.value) {
            if (account.type !== AccountType.MultiSubAccounts.type || !account.subAccounts) {
                continue;
            }

            const subAccounts = account.subAccounts as Account[];

            for (const [subAccount, index] of itemAndIndex(subAccounts)) {
                if (subAccount.id === currentSubAccount.id) {
                    subAccounts.splice(index, 1);
                    break;
                }
            }
        }

        if (allAccountsMap.value[currentSubAccount.id]) {
            delete allAccountsMap.value[currentSubAccount.id];
        }

        if (allCategorizedAccountsMap.value[currentSubAccount.category]) {
            const accountList = allCategorizedAccountsMap.value[currentSubAccount.category]!.accounts;

            for (const account of accountList) {
                if (account.type !== AccountType.MultiSubAccounts.type || !account.subAccounts) {
                    continue;
                }

                const subAccounts = account.subAccounts as Account[];

                for (const [subAccount, index] of itemAndIndex(subAccounts)) {
                    if (subAccount.id === currentSubAccount.id) {
                        subAccounts.splice(index, 1);
                        break;
                    }
                }
            }
        }
    }

    function updateAccountListInvalidState(invalidState: boolean): void {
        accountListStateInvalid.value = invalidState;
    }

    function resetAccounts(): void {
        loadingPromise = null;
        allAccounts.value = [];
        allAccountsMap.value = {};
        allCategorizedAccountsMap.value = {};
        accountListStateInvalid.value = true;
    }

    function getFirstShowingIds(showHidden: boolean): AccountShowingIds {
        const ret: AccountShowingIds = {
            accounts: {},
            subAccounts: {}
        };

        for (const [category, categorizedAccounts] of entries(allCategorizedAccountsMap.value)) {
            if (!categorizedAccounts || !categorizedAccounts.accounts) {
                continue;
            }

            const accounts = categorizedAccounts.accounts;

            for (const account of accounts) {
                if (account.type === AccountType.MultiSubAccounts.type && account.subAccounts) {
                    for (const subAccount of account.subAccounts) {
                        if (showHidden || !subAccount.hidden) {
                            ret.subAccounts[account.id] = subAccount.id;
                            break;
                        }
                    }
                }

                if (showHidden || !account.hidden) {
                    ret.accounts[parseInt(category)] = account.id;
                    break;
                }
            }
        }

        return ret;
    }

    function getLastShowingIds(showHidden: boolean): AccountShowingIds {
        const ret: AccountShowingIds = {
            accounts: {},
            subAccounts: {}
        };

        for (const [category, categorizedAccounts] of entries(allCategorizedAccountsMap.value)) {
            if (!categorizedAccounts || !categorizedAccounts.accounts) {
                continue;
            }

            const accounts = categorizedAccounts.accounts;

            for (const account of reversed(accounts)) {
                if (account.type === AccountType.MultiSubAccounts.type && account.subAccounts) {
                    for (const subAccount of reversed(account.subAccounts)) {
                        if (showHidden || !subAccount.hidden) {
                            ret.subAccounts[account.id] = subAccount.id;
                            break;
                        }
                    }
                }

                if (showHidden || !account.hidden) {
                    ret.accounts[parseInt(category)] = account.id;
                    break;
                }
            }
        }

        return ret;
    }

    function getMainCreditCardAccount(accountId?: string): Account | null {
        if (!accountId) {
            return null;
        }

        const accountIds = accountId.split(',');
        let mainAccount = null;

        for (const accountId of accountIds) {
            let account = allAccountsMap.value[accountId];

            if (!account) {
                return null;
            }

            if (account.parentId !== '0') {
                account = allAccountsMap.value[account.parentId];
            }

            if (!account) {
                return null;
            }

            if (mainAccount) {
                if (mainAccount.id !== account.id) {
                    return null;
                } else {
                    continue;
                }
            }

            mainAccount = account;
        }

        if (mainAccount && mainAccount.category === AccountCategory.CreditCard.type) {
            return mainAccount;
        }

        return null;
    }

    // Returns the statement date used for billing cycle date ranges of the given account(s).
    // A card with its own statement date (a sub-account card of a shared-limit pool) follows
    // its own billing cycle, so its own statement date takes precedence over the pool's one
    function getAccountStatementDate(accountId?: string): number | undefined | null {
        if (accountId && !accountId.includes(',')) {
            const account = allAccountsMap.value[accountId];

            if (account && account.parentId !== '0' && account.creditCardStatementDate) {
                return account.creditCardStatementDate;
            }
        }

        const mainAccount = getMainCreditCardAccount(accountId);

        if (!mainAccount) {
            return null;
        }

        return mainAccount.creditCardStatementDate;
    }

    function getAccountStatementDateIncludeTransaction(accountId?: string): boolean {
        if (accountId && !accountId.includes(',')) {
            const account = allAccountsMap.value[accountId];

            if (account && account.parentId !== '0' && account.creditCardStatementDate) {
                return !!account.creditCardStatementDateIncludeTransaction;
            }
        }

        const mainAccount = getMainCreditCardAccount(accountId);
        return !!mainAccount && !!mainAccount.creditCardStatementDateIncludeTransaction;
    }

    function getNetAssets(showAccountBalance: boolean, excludedAccountIds: Record<string, boolean>): BigDecimal | HiddenAmount | BigDecimalWithSuffix {
        if (!showAccountBalance) {
            return DISPLAY_HIDDEN_AMOUNT;
        }

        const accountsBalance = getAllFilteredAccountsBalance(allCategorizedAccountsMap.value, settingsStore.appSettings.accountCategoryOrders,
            account => !(account.type === AccountType.SingleAccount.type && excludedAccountIds[account.id])
        );
        let netAssets: BigDecimal = BIG_DECIMAL_ZERO;
        let hasUnCalculatedAmount = false;

        for (const accountBalance of accountsBalance) {
            if (accountBalance.currency === userStore.currentUserDefaultCurrency) {
                netAssets = netAssets.add(accountBalance.balance);
            } else {
                const balance = exchangeRatesStore.getExchangedAmount(accountBalance.balance, accountBalance.currency, userStore.currentUserDefaultCurrency);

                if (!balance) {
                    hasUnCalculatedAmount = true;
                    continue;
                }

                netAssets = netAssets.add(balance.truncate());
            }
        }

        if (hasUnCalculatedAmount) {
            return {
                value: netAssets,
                suffix: INCOMPLETE_AMOUNT_SUFFIX
            };
        } else {
            return netAssets;
        }
    }

    function getTotalAssets(showAccountBalance: boolean, excludedAccountIds: Record<string, boolean>): BigDecimal | HiddenAmount | BigDecimalWithSuffix {
        if (!showAccountBalance) {
            return DISPLAY_HIDDEN_AMOUNT;
        }

        const accountsBalance = getAllFilteredAccountsBalance(allCategorizedAccountsMap.value, settingsStore.appSettings.accountCategoryOrders,
            account => (account.isAsset || false) && !(account.type === AccountType.SingleAccount.type && excludedAccountIds[account.id])
        );
        let totalAssets: BigDecimal = BIG_DECIMAL_ZERO;
        let hasUnCalculatedAmount = false;

        for (const accountBalance of accountsBalance) {
            if (accountBalance.currency === userStore.currentUserDefaultCurrency) {
                totalAssets = totalAssets.add(accountBalance.balance);
            } else {
                const balance = exchangeRatesStore.getExchangedAmount(accountBalance.balance, accountBalance.currency, userStore.currentUserDefaultCurrency);

                if (!balance) {
                    hasUnCalculatedAmount = true;
                    continue;
                }

                totalAssets = totalAssets.add(balance.truncate());
            }
        }

        if (hasUnCalculatedAmount) {
            return {
                value: totalAssets,
                suffix: INCOMPLETE_AMOUNT_SUFFIX
            };
        } else {
            return totalAssets;
        }
    }

    function getTotalLiabilities(showAccountBalance: boolean, excludedAccountIds: Record<string, boolean>): BigDecimal | HiddenAmount | BigDecimalWithSuffix {
        if (!showAccountBalance) {
            return DISPLAY_HIDDEN_AMOUNT;
        }

        const accountsBalance = getAllFilteredAccountsBalance(allCategorizedAccountsMap.value, settingsStore.appSettings.accountCategoryOrders,
            account => (account.isLiability || false) && !(account.type === AccountType.SingleAccount.type && excludedAccountIds[account.id])
        );
        let totalLiabilities: BigDecimal = BIG_DECIMAL_ZERO;
        let hasUnCalculatedAmount = false;

        for (const accountBalance of accountsBalance) {
            if (accountBalance.currency === userStore.currentUserDefaultCurrency) {
                totalLiabilities = totalLiabilities.subtract(accountBalance.balance);
            } else {
                const balance = exchangeRatesStore.getExchangedAmount(accountBalance.balance, accountBalance.currency, userStore.currentUserDefaultCurrency);

                if (!balance) {
                    hasUnCalculatedAmount = true;
                    continue;
                }

                totalLiabilities = totalLiabilities.subtract(balance.truncate());
            }
        }

        if (hasUnCalculatedAmount) {
            return {
                value: totalLiabilities,
                suffix: INCOMPLETE_AMOUNT_SUFFIX
            };
        } else {
            return totalLiabilities;
        }
    }

    // The bank states one shared limit in every currency of the pool (for example CNY 30000
    // and USD 4615 on the same pool), so the ratio of two stated limits is the bank's own
    // conversion rate between those currencies. The pool limit wins for the pool currency and
    // the first stated card limit wins for a card currency
    function getCreditCardPoolLimitsByCurrency(account: Account): Record<string, BigDecimal> {
        const limits: Record<string, BigDecimal> = {};

        if (account.currency && account.currency !== ACCOUNT_CURRENCY_NOT_SET_VALUE && account.numericCreditCardLimit > 0) {
            limits[account.currency] = parseBigDecimal(account.creditCardLimit);
        }

        if (account.subAccounts) {
            for (const subAccount of account.subAccounts) {
                if (!subAccount.currency || subAccount.currency === ACCOUNT_CURRENCY_NOT_SET_VALUE || limits[subAccount.currency]) {
                    continue;
                }

                if (subAccount.numericCreditCardCurrencyLimit > 0) {
                    limits[subAccount.currency] = parseBigDecimal(subAccount.numericCreditCardCurrencyLimit);
                }
            }
        }

        return limits;
    }

    // Returns the rate converting one unit of fromCurrency into toCurrency inside a shared
    // limit pool, or null when neither the limits stated by the bank nor the exchange rates
    // can provide it
    function getCreditCardPoolCurrencyRate(limits: Record<string, BigDecimal>, fromCurrency: string, toCurrency: string): BigDecimal | null {
        if (fromCurrency === toCurrency) {
            return parseBigDecimal(1);
        }

        const fromLimit = limits[fromCurrency];
        const toLimit = limits[toCurrency];

        if (fromLimit && toLimit && fromLimit.isPositive()) {
            return toLimit.divide(fromLimit);
        }

        return exchangeRatesStore.getExchangedAmount(parseBigDecimal(1), fromCurrency, toCurrency);
    }

    // Returns the debt of all cards of a shared limit pool converted into the specified
    // currency, so a purchase on any card occupies the limit of every card. Hidden cards still
    // occupy the shared limit, so they are always counted. Debts which cannot be converted are
    // excluded and reported, so the caller can mark the result as incomplete
    function getCreditCardPoolUsedAmount(account: Account, limits: Record<string, BigDecimal>, currency: string): { usedAmount: BigDecimal; hasUnconvertibleDebt: boolean } {
        let usedAmount: BigDecimal = BIG_DECIMAL_ZERO;
        let hasUnconvertibleDebt = false;

        if (account.subAccounts) {
            for (const subAccount of account.subAccounts) {
                // sub-account balances are negative for debts, so negate them to get the used amount
                const debt = parseBigDecimal(subAccount.balance).negate();

                if (debt.isZero()) {
                    continue;
                }

                const rate = getCreditCardPoolCurrencyRate(limits, subAccount.currency, currency);

                if (rate === null) {
                    hasUnconvertibleDebt = true;
                    continue;
                }

                usedAmount = usedAmount.add(debt.multiply(rate));
            }
        }

        return { usedAmount, hasUnconvertibleDebt };
    }

    function getCreditCardPoolBalance(account: Account): { limitAmount: BigDecimal; usedAmount: BigDecimal; hasUnconvertibleDebt: boolean } | null {
        if (account.type !== AccountType.MultiSubAccounts.type || account.category !== AccountCategory.CreditCard.type) {
            return null;
        }

        const limits = getCreditCardPoolLimitsByCurrency(account);
        const used = getCreditCardPoolUsedAmount(account, limits, account.currency);

        return {
            limitAmount: parseBigDecimal(account.creditCardLimit),
            usedAmount: used.usedAmount,
            hasUnconvertibleDebt: used.hasUnconvertibleDebt
        };
    }

    // Returns the remaining credit of a shared-limit pool, counted once in the pool's currency.
    // Debts of member cards are converted by the limits stated by the bank, and those which
    // cannot be converted are excluded and marked with an incomplete suffix
    function getCreditCardPoolRemainingCredit(showAccountBalance: boolean, account: Account): AccountDisplayBalance | null {
        const pool = getCreditCardPoolBalance(account);

        if (!pool || !account.currency || account.currency === ACCOUNT_CURRENCY_NOT_SET_VALUE || account.numericCreditCardLimit <= 0) {
            return null;
        }

        const remainingCredit = pool.limitAmount.subtract(pool.usedAmount);
        const balance: BigDecimal | BigDecimalWithSuffix = pool.hasUnconvertibleDebt ? {
            value: remainingCredit,
            suffix: INCOMPLETE_AMOUNT_SUFFIX
        } : remainingCredit;

        return {
            balance: showAccountBalance ? balance : DISPLAY_HIDDEN_AMOUNT,
            currency: account.currency
        };
    }

    // Returns the remaining credit of the shared pool expressed in the currency of one of its
    // cards, so every card of the pool shows the same shared remaining credit. The limit stated
    // for that currency is used directly instead of converting the pool result back, which keeps
    // the amount exact. Returns null when no limit is stated in the card's currency
    function getCreditCardPoolCardRemainingCredit(showAccountBalance: boolean, account: Account, subAccount: Account): AccountDisplayBalance | null {
        if (account.type !== AccountType.MultiSubAccounts.type || account.category !== AccountCategory.CreditCard.type) {
            return null;
        }

        if (!subAccount.currency || subAccount.currency === ACCOUNT_CURRENCY_NOT_SET_VALUE) {
            return null;
        }

        const limits = getCreditCardPoolLimitsByCurrency(account);
        const limitAmount = limits[subAccount.currency];

        if (!limitAmount || !limitAmount.isPositive()) {
            return null;
        }

        const used = getCreditCardPoolUsedAmount(account, limits, subAccount.currency);
        const remainingCredit = limitAmount.subtract(used.usedAmount);
        const balance: BigDecimal | BigDecimalWithSuffix = used.hasUnconvertibleDebt ? {
            value: remainingCredit,
            suffix: INCOMPLETE_AMOUNT_SUFFIX
        } : remainingCredit;

        return {
            balance: showAccountBalance ? balance : DISPLAY_HIDDEN_AMOUNT,
            currency: subAccount.currency
        };
    }

    function getAccountCategoryTotalBalance(showAccountBalance: boolean, accountCategory: AccountCategory, showAvailableCreditForCreditCard: boolean): BigDecimal | HiddenAmount | BigDecimalWithSuffix | undefined {
        if (!showAccountBalance) {
            return DISPLAY_HIDDEN_AMOUNT;
        }

        const accountsBalance = getAllFilteredAccountsBalance(allCategorizedAccountsMap.value, settingsStore.appSettings.accountCategoryOrders,
            account => account.category === accountCategory.type);
        let totalBalance: BigDecimal = BIG_DECIMAL_ZERO;
        let hasUnCalculatedAmount = false;

        for (const accountBalance of accountsBalance) {
            let balance: BigDecimal | null = null;

            if (accountBalance.currency === userStore.currentUserDefaultCurrency) {
                balance = accountBalance.balance;
            } else {
                balance = exchangeRatesStore.getExchangedAmount(accountBalance.balance, accountBalance.currency, userStore.currentUserDefaultCurrency);

                if (!balance) {
                    hasUnCalculatedAmount = true;
                    continue;
                }
            }

            if (accountBalance.isAsset) {
                totalBalance = totalBalance.add(balance);
            } else if (accountBalance.isLiability) {
                totalBalance = totalBalance.subtract(balance);
            } else {
                totalBalance = totalBalance.add(balance);
            }
        }

        if (accountCategory.type !== AccountCategory.CreditCard.type || !showAvailableCreditForCreditCard) {
            if (hasUnCalculatedAmount) {
                return {
                    value: totalBalance,
                    suffix: INCOMPLETE_AMOUNT_SUFFIX
                };
            } else {
                return totalBalance;
            }
        }

        // Available credit of credit cards is NOT computed from per-card balance entries: the
        // limit of a shared pool is counted once for the whole pool, and the debts of all its
        // cards are subtracted from it after being converted by the limits stated by the bank.
        // Debts which cannot be converted are excluded and marked incomplete
        let totalCreditCardAvailableCredit: BigDecimal = BIG_DECIMAL_ZERO;
        let hasUnCalculatedAvailableCredit = false;
        let hasCalculatedAvailableCredit = false;
        let hasUnconvertiblePoolDebt = false;

        const addAvailableCredit = function (amount: BigDecimal, currency: string): void {
            if (currency === userStore.currentUserDefaultCurrency) {
                totalCreditCardAvailableCredit = totalCreditCardAvailableCredit.add(amount);
                hasCalculatedAvailableCredit = true;
            } else {
                const exchangedAmount = exchangeRatesStore.getExchangedAmount(amount, currency, userStore.currentUserDefaultCurrency);

                if (exchangedAmount) {
                    totalCreditCardAvailableCredit = totalCreditCardAvailableCredit.add(exchangedAmount.truncate());
                    hasCalculatedAvailableCredit = true;
                } else {
                    hasUnCalculatedAvailableCredit = true;
                }
            }
        };

        const creditCardAccounts = allCategorizedAccountsMap.value[AccountCategory.CreditCard.type]?.accounts;

        if (creditCardAccounts) {
            for (const account of creditCardAccounts) {
                if (account.hidden) {
                    continue;
                }

                if (account.type === AccountType.SingleAccount.type) {
                    // an entity card owns its whole limit
                    if (account.numericCreditCardLimit <= 0 || !account.currency || account.currency === ACCOUNT_CURRENCY_NOT_SET_VALUE) {
                        hasUnCalculatedAvailableCredit = true;
                        continue;
                    }

                    addAvailableCredit(parseBigDecimal(account.creditCardLimit).add(parseBigDecimal(account.balance)), account.currency);
                } else if (account.type === AccountType.MultiSubAccounts.type) {
                    const pool = getCreditCardPoolBalance(account);

                    if (!pool) {
                        continue;
                    }

                    if (account.numericCreditCardLimit <= 0 || !account.currency || account.currency === ACCOUNT_CURRENCY_NOT_SET_VALUE) {
                        // a pool without a limit cannot provide available credit
                        hasUnCalculatedAvailableCredit = true;
                        continue;
                    }

                    if (pool.hasUnconvertibleDebt) {
                        hasUnconvertiblePoolDebt = true;
                    }

                    addAvailableCredit(pool.limitAmount.subtract(pool.usedAmount), account.currency);
                }
            }
        }

        if (!hasCalculatedAvailableCredit) {
            return undefined;
        } else if (hasUnCalculatedAvailableCredit || hasUnconvertiblePoolDebt) {
            return {
                value: totalCreditCardAvailableCredit,
                suffix: INCOMPLETE_AMOUNT_SUFFIX
            };
        } else {
            return totalCreditCardAvailableCredit;
        }
    }

    function getAccountBalance(showAccountBalance: boolean, account: Account): BigDecimal | HiddenAmount | null {
        if (account.type !== AccountType.SingleAccount.type) {
            return null;
        }

        if (showAccountBalance) {
            if (account.isAsset) {
                return parseBigDecimal(account.balance);
            } else if (account.isLiability) {
                return parseBigDecimal(account.balance).negate();
            } else {
                return parseBigDecimal(account.balance);
            }
        } else {
            return DISPLAY_HIDDEN_AMOUNT;
        }
    }

    function getAccountSubAccountBalance(showAccountBalance: boolean, showHidden: boolean, account: Account, subAccountId?: string, onlyShowSelectedAccountIds?: string[]): AccountDisplayBalance | null {
        if (account.type !== AccountType.MultiSubAccounts.type) {
            return null;
        }

        // the balance of a credit card limit pool is displayed in the pool's currency: the debt
        // of every card (hidden cards included, because they still occupy the shared limit) is
        // converted by the limits stated by the bank, so the displayed debt always matches the
        // remaining credit of the pool
        const isCreditCardPool = account.category === AccountCategory.CreditCard.type;
        const poolLimits = isCreditCardPool ? getCreditCardPoolLimitsByCurrency(account) : {};
        let resultCurrency = userStore.currentUserDefaultCurrency;
        let parentHasSetCurrency = false;

        if (isCreditCardPool && account.currency && account.currency !== ACCOUNT_CURRENCY_NOT_SET_VALUE) {
            resultCurrency = account.currency;
            parentHasSetCurrency = true;
        }

        if (!account.subAccounts || !account.subAccounts.length) {
            return {
                balance: showAccountBalance ? BIG_DECIMAL_ZERO : DISPLAY_HIDDEN_AMOUNT,
                currency: resultCurrency
            };
        }

        const selectedAccountIds: Record<string, boolean> = isArray(onlyShowSelectedAccountIds) ? arrayItemToObjectField(onlyShowSelectedAccountIds, true) : {};
        const allSubAccountCurrenciesMap: Record<string, boolean> = {};
        const allSubAccountCurrencies: string[] = [];
        let totalBalance: BigDecimal = BIG_DECIMAL_ZERO;

        for (const subAccount of account.subAccounts) {
            if (!isCreditCardPool && !showHidden && subAccount.hidden) {
                continue;
            }

            if (!allSubAccountCurrenciesMap[subAccount.currency]) {
                allSubAccountCurrenciesMap[subAccount.currency] = true;
                allSubAccountCurrencies.push(subAccount.currency);
            }
        }

        if (allSubAccountCurrencies.length === 0) {
            return {
                balance: showAccountBalance ? BIG_DECIMAL_ZERO : DISPLAY_HIDDEN_AMOUNT,
                currency: resultCurrency
            };
        }

        if (allSubAccountCurrencies.length === 1 && !parentHasSetCurrency) {
            resultCurrency = allSubAccountCurrencies[0] as string;
        }

        let hasUnCalculatedAmount = false;

        for (const subAccount of account.subAccounts) {
            if (subAccountId) {
                if (subAccountId === subAccount.id) {
                    return {
                        balance: getAccountBalance(showAccountBalance, subAccount)!,
                        currency: subAccount.currency
                    };
                }
            }

            if (onlyShowSelectedAccountIds && onlyShowSelectedAccountIds.length > 0 && !selectedAccountIds[subAccount.id]) {
                continue;
            }

            if (!isCreditCardPool && !showHidden && subAccount.hidden) {
                continue;
            }

            if (subAccount.currency === resultCurrency) {
                if (subAccount.isAsset) {
                    totalBalance = totalBalance.add(parseBigDecimal(subAccount.balance));
                } else if (subAccount.isLiability) {
                    totalBalance = totalBalance.subtract(parseBigDecimal(subAccount.balance));
                } else {
                    totalBalance = totalBalance.add(parseBigDecimal(subAccount.balance));
                }
            } else {
                // inside a shared limit pool the limits stated by the bank define the conversion
                // rate, and the exchange rates are only a fallback for cards without a stated limit
                const poolRate = isCreditCardPool ? getCreditCardPoolCurrencyRate(poolLimits, subAccount.currency, resultCurrency) : null;
                const balance = poolRate !== null ? parseBigDecimal(subAccount.balance).multiply(poolRate) : exchangeRatesStore.getExchangedAmount(parseBigDecimal(subAccount.balance), subAccount.currency, resultCurrency);

                if (!balance) {
                    hasUnCalculatedAmount = true;
                    continue;
                }

                if (subAccount.isAsset) {
                    totalBalance = totalBalance.add(balance);
                } else if (subAccount.isLiability) {
                    totalBalance = totalBalance.subtract(balance);
                } else {
                    totalBalance = totalBalance.add(balance);
                }
            }
        }

        if (subAccountId) { // not found specified id in sub accounts
            return null;
        }

        const displayTotalBalance: BigDecimalWithSuffix = {
            value: totalBalance,
            suffix: hasUnCalculatedAmount ? INCOMPLETE_AMOUNT_SUFFIX : ''
        };

        return {
            balance: showAccountBalance ? displayTotalBalance : DISPLAY_HIDDEN_AMOUNT,
            currency: resultCurrency
        };
    }

    function getSortedAccounts(accountIds: string[] | undefined, sortBy: 'displayOrder' | 'balance' | string, count: number): Account[] {
        const selectedAccountIds: Record<string, boolean> = isArray(accountIds) ? arrayItemToObjectField(accountIds, true) : {};
        const accounts: Account[] = allAccounts.value.filter(account => {
            if (account.hidden) {
                return false;
            }

            if (account.type === AccountType.MultiSubAccounts.type && account.subAccounts) {
                for (const subAccount of account.subAccounts) {
                    if (!subAccount.hidden && selectedAccountIds[subAccount.id]) {
                        return true;
                    }
                }
            }

            return !accountIds || accountIds.length < 1 || !!selectedAccountIds[account.id];
        });

        if (sortBy === 'balance') {
            const accountBalances: Record<string, BigDecimal> = {};

            for (const account of accounts) {
                let totalBalance: BigDecimal = BIG_DECIMAL_ZERO;

                if (account.type === AccountType.SingleAccount.type) {
                    totalBalance = parseBigDecimal(account.balance);

                    if (account.currency !== userStore.currentUserDefaultCurrency) {
                        const exchangedBalance = exchangeRatesStore.getExchangedAmount(totalBalance, account.currency, userStore.currentUserDefaultCurrency);

                        if (exchangedBalance) {
                            totalBalance = exchangedBalance.truncate();
                        }
                    }
                } else if (account.type === AccountType.MultiSubAccounts.type && account.subAccounts) {
                    for (const subAccount of account.subAccounts) {
                        if (!selectedAccountIds[subAccount.id]) {
                            continue;
                        }

                        let subAccountBalance = parseBigDecimal(subAccount.balance);

                        if (subAccount.currency !== userStore.currentUserDefaultCurrency) {
                            const exchangedBalance = exchangeRatesStore.getExchangedAmount(subAccountBalance, subAccount.currency, userStore.currentUserDefaultCurrency);

                            if (exchangedBalance) {
                                subAccountBalance = exchangedBalance.truncate();
                            }
                        }

                        totalBalance = totalBalance.add(subAccountBalance);
                    }
                }

                accountBalances[account.id] = totalBalance;
            }

            accounts.sort((a, b) => {
                const balanceA = accountBalances[a.id] ?? BIG_DECIMAL_ZERO;
                const balanceB = accountBalances[b.id] ?? BIG_DECIMAL_ZERO;

                if (balanceA === balanceB) {
                    return a.compareTo(b, settingsStore.accountCategoryDisplayOrders, allAccountsMap.value);
                }

                return balanceB.compareTo(balanceA);
            });
        }

        return accounts.slice(0, count);
    }

    function hasAccount(accountCategory: AccountCategory, visibleOnly: boolean): boolean {
        const categorizedAccounts = allCategorizedAccountsMap.value[accountCategory.type];

        if (!categorizedAccounts || !categorizedAccounts.accounts || !categorizedAccounts.accounts.length) {
            return false;
        }

        let shownCount = 0;

        for (const account of categorizedAccounts.accounts) {
            if (!visibleOnly || !account.hidden) {
                shownCount++;
            }
        }

        return shownCount > 0;
    }

    function hasVisibleSubAccount(showHidden: boolean, account: Account): boolean {
        if (!account || account.type !== AccountType.MultiSubAccounts.type || !account.subAccounts) {
            return false;
        }

        for (const subAccount of account.subAccounts) {
            if (showHidden || !subAccount.hidden) {
                return true;
            }
        }

        return false;
    }

    function loadAllAccounts({ force }: { force: boolean }): Promise<Account[]> {
        if (loadingPromise) {
            return loadingPromise;
        }

        if (!force && !accountListStateInvalid.value) {
            return new Promise((resolve) => {
                resolve(allAccounts.value);
            });
        }

        const currentPromise = loadingPromise = new Promise((resolve, reject) => {
            services.getAllAccounts({
                visibleOnly: false
            }).then(response => {
                if (!loadingPromise || loadingPromise !== currentPromise) {
                    logger.error('loadingPromise is invalid after retrieving account list');
                    reject({ message: 'An error occurred' });
                    return;
                }

                const data = response.data;

                if (!data || !data.success || !data.result) {
                    loadingPromise = null;
                    reject({ message: 'Unable to retrieve account list' });
                    return;
                }

                if (accountListStateInvalid.value) {
                    updateAccountListInvalidState(false);
                }

                const accounts = Account.sortAccounts(Account.ofMulti(data.result), settingsStore.accountCategoryDisplayOrders);

                if (force && data.result && isEquals(allAccounts.value, accounts)) {
                    loadingPromise = null;
                    reject({ message: 'Account list is up to date', isUpToDate: true });
                    return;
                }

                loadAccountList(accounts);
                loadingPromise = null;

                resolve(accounts);
            }).catch(error => {
                if (loadingPromise === currentPromise) {
                    loadingPromise = null;
                }

                if (force) {
                    logger.error('failed to force load account list', error);
                } else {
                    logger.error('failed to load account list', error);
                }

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to retrieve account list' });
                } else {
                    reject(error);
                }
            });
        });

        return loadingPromise;
    }

    function getAccount({ accountId }: { accountId: string }): Promise<Account> {
        return new Promise((resolve, reject) => {
            services.getAccount({
                id: accountId
            }).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to retrieve account' });
                    return;
                }

                const account = Account.of(data.result);

                resolve(account);
            }).catch(error => {
                logger.error('failed to load account info', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to retrieve account' });
                } else {
                    reject(error);
                }
            });
        });
    }

    function saveAccount({ account, subAccounts, isEdit, clientSessionId }: { account: Account, subAccounts: Account[], isEdit: boolean, clientSessionId: string }): Promise<Account> {
        return new Promise((resolve, reject) => {
            const oldAccount = isEdit ? allAccountsMap.value[account.id] : null;
            let promise = null;

            if (!isEdit) {
                promise = services.addAccount(account.toCreateRequest(clientSessionId, subAccounts));
            } else {
                promise = services.modifyAccount(account.toModifyRequest(clientSessionId, subAccounts));
            }

            promise.then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    if (!isEdit) {
                        reject({ message: 'Unable to add account' });
                    } else {
                        reject({ message: 'Unable to save account' });
                    }
                    return;
                }

                const newAccount = Account.of(data.result);

                if (!isEdit) {
                    addAccountToAccountList(newAccount);
                } else {
                    if (oldAccount && oldAccount.category === newAccount.category) {
                        updateAccountToAccountList(oldAccount, newAccount);
                    } else {
                        updateAccountListInvalidState(true);
                    }
                }

                overviewStore.updateTransactionOverviewInvalidState(true);

                resolve(newAccount);
            }).catch(error => {
                logger.error('failed to save account', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    if (!isEdit) {
                        reject({ message: 'Unable to add account' });
                    } else {
                        reject({ message: 'Unable to save account' });
                    }
                } else {
                    reject(error);
                }
            });
        });
    }

    function uploadCreditCardPicture({ pictureFile, clientSessionId }: { pictureFile: File, clientSessionId?: string }): Promise<CreditCardPictureInfoBasicResponse> {
        return new Promise((resolve, reject) => {
            services.uploadCreditCardPicture({ pictureFile, clientSessionId }).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to upload credit card picture' });
                    return;
                }

                resolve(data.result);
            }).catch(error => {
                logger.error('failed to upload credit card picture', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to upload credit card picture' });
                } else {
                    reject(error);
                }
            });
        });
    }

    function removeUnusedCreditCardPicture({ pictureId }: { pictureId: string }): Promise<boolean> {
        return new Promise((resolve, reject) => {
            services.removeUnusedCreditCardPicture({ id: pictureId }).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to remove credit card picture' });
                    return;
                }

                resolve(data.result);
            }).catch(error => {
                logger.error('failed to remove credit card picture', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to remove credit card picture' });
                } else {
                    reject(error);
                }
            });
        });
    }

    function updateAccountLastReconciledTime(accountId: string, lastReconciledTime: number): Promise<boolean> {
        return new Promise((resolve, reject) => {
            const account = allAccountsMap.value[accountId];

            services.updateAccountLastReconciledTime({
                id: accountId,
                lastReconciledTime: lastReconciledTime
            }).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to last reconciled time' });
                    return;
                }

                if (account) {
                    account.lastReconciledTime = lastReconciledTime;
                } else {
                    updateAccountListInvalidState(true);
                }

                overviewStore.updateTransactionOverviewInvalidState(true);

                resolve(data.result);
            }).catch(error => {
                logger.error('failed to update last reconciled time', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to update last reconciled time' });
                } else {
                    reject(error);
                }
            });
        });
    }

    function changeAccountDisplayOrder({ accountId, from, to, updateListOrder, updateGlobalListOrder }: { accountId: string, from: number, to: number, updateListOrder: boolean, updateGlobalListOrder: boolean }): Promise<void> {
        const account = allAccountsMap.value[accountId];

        return new Promise((resolve, reject) => {
            if (!account ||
                !allCategorizedAccountsMap.value[account.category] ||
                !allCategorizedAccountsMap.value[account.category]!.accounts ||
                !allCategorizedAccountsMap.value[account.category]!.accounts[to]) {
                reject({ message: 'Unable to move account' });
                return;
            }

            if (!accountListStateInvalid.value) {
                updateAccountListInvalidState(true);
            }

            updateAccountDisplayOrderInAccountList({ account, from, to, updateListOrder, updateGlobalListOrder });

            resolve();
        });
    }

    function updateAccountDisplayOrders(): Promise<boolean> {
        const newDisplayOrders: AccountNewDisplayOrderRequest[] = [];

        for (const categorizedAccounts of values(allCategorizedAccountsMap.value)) {
            for (const [account, index] of itemAndIndex(categorizedAccounts.accounts)) {
                newDisplayOrders.push({
                    id: account.id,
                    displayOrder: index + 1
                });
            }
        }

        return new Promise((resolve, reject) => {
            services.moveAccount({
                newDisplayOrders: newDisplayOrders
            }).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to move account' });
                    return;
                }

                loadAllAccounts({ force: false }).finally(() => {
                    if (accountListStateInvalid.value) {
                        updateAccountListInvalidState(false);
                    }

                    resolve(data.result);
                });
            }).catch(error => {
                logger.error('failed to save accounts display order', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to move account' });
                } else {
                    reject(error);
                }
            });
        });
    }

    function hideAccount({ account, hidden }: { account: Account, hidden: boolean }): Promise<boolean> {
        return new Promise((resolve, reject) => {
            services.hideAccount({
                id: account.id,
                hidden: hidden
            }).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    if (hidden) {
                        reject({ message: 'Unable to hide this account' });
                    } else {
                        reject({ message: 'Unable to unhide this account' });
                    }

                    return;
                }

                updateAccountVisibilityInAccountList({ account, hidden });

                resolve(data.result);
            }).catch(error => {
                logger.error('failed to change account visibility', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    if (hidden) {
                        reject({ message: 'Unable to hide this account' });
                    } else {
                        reject({ message: 'Unable to unhide this account' });
                    }
                } else {
                    reject(error);
                }
            });
        });
    }

    function deleteAccount({ account, beforeResolve }: { account: Account, beforeResolve?: BeforeResolveFunction }): Promise<boolean> {
        return new Promise((resolve, reject) => {
            services.deleteAccount({
                id: account.id
            }).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to delete this account' });
                    return;
                }

                if (beforeResolve) {
                    beforeResolve(() => {
                        removeAccountFromAccountList(account);
                    });
                } else {
                    removeAccountFromAccountList(account);
                }

                overviewStore.updateTransactionOverviewInvalidState(true);

                resolve(data.result);
            }).catch(error => {
                logger.error('failed to delete account', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to delete this account' });
                } else {
                    reject(error);
                }
            });
        });
    }

    function deleteSubAccount({ subAccount, beforeResolve }: { subAccount: Account, beforeResolve?: BeforeResolveFunction }): Promise<boolean> {
        return new Promise((resolve, reject) => {
            services.deleteSubAccount({
                id: subAccount.id
            }).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to delete this sub-account' });
                    return;
                }

                if (beforeResolve) {
                    beforeResolve(() => {
                        removeSubAccountFromAccountList(subAccount);
                    });
                } else {
                    removeSubAccountFromAccountList(subAccount);
                }

                overviewStore.updateTransactionOverviewInvalidState(true);

                resolve(data.result);
            }).catch(error => {
                logger.error('failed to delete sub-account', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to delete this sub-account' });
                } else {
                    reject(error);
                }
            });
        });
    }

    return {
        // states
        allAccounts,
        allAccountsMap,
        allCategorizedAccountsMap,
        accountListStateInvalid,
        // computed states
        allPlainAccounts,
        allMixedPlainAccounts,
        allVisiblePlainAccounts,
        allAvailableAccountsCount,
        allVisibleAccountsCount,
        maxCategoryAccountCount,
        // functions
        updateAccountListInvalidState,
        resetAccounts,
        getFirstShowingIds,
        getLastShowingIds,
        getAccountStatementDate,
        getAccountStatementDateIncludeTransaction,
        getNetAssets,
        getTotalAssets,
        getTotalLiabilities,
        getAccountCategoryTotalBalance,
        getAccountBalance,
        getAccountSubAccountBalance,
        getCreditCardPoolRemainingCredit,
        getCreditCardPoolCardRemainingCredit,
        getSortedAccounts,
        hasAccount,
        hasVisibleSubAccount,
        loadAccountList,
        loadAllAccounts,
        getAccount,
        saveAccount,
        uploadCreditCardPicture,
        removeUnusedCreditCardPicture,
        updateAccountLastReconciledTime,
        changeAccountDisplayOrder,
        updateAccountDisplayOrders,
        hideAccount,
        deleteAccount,
        deleteSubAccount
    }
});
