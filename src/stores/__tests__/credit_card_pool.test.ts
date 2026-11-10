import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';

import { useUserStore } from '@/stores/user.ts';
import { useAccountsStore } from '@/stores/account.ts';

import { EMPTY_USER_BASIC_INFO } from '@/models/user.ts';

import { AccountCategory, AccountType } from '@/core/account.ts';
import { Account } from '@/models/account.ts';
import type { AccountInfoResponse } from '@/models/account.ts';
import type { BigDecimal, BigDecimalWithSuffix } from '@/core/numeral.ts';

import { isBigDecimal } from '@/lib/numeral.ts';

// The stores read window.location and localStorage when they are loaded and the test environment is node
vi.hoisted(() => {
    const data = new Map<string, string>();
    const storage = {
        getItem: (key: string) => data.has(key) ? data.get(key) as string : null,
        setItem: (key: string, value: string) => { data.set(key, String(value)); },
        removeItem: (key: string) => { data.delete(key); },
        clear: () => { data.clear(); },
        key: (index: number) => Array.from(data.keys())[index] ?? null,
        get length() { return data.size; }
    };

    Object.assign(globalThis, {
        window: { location: { pathname: '/', origin: 'http://localhost', href: 'http://localhost/' }, localStorage: storage, sessionStorage: storage },
        localStorage: storage,
        sessionStorage: storage
    });
});

function cardResponse(id: string, parentId: string, currency: string, balance: string, overrides?: Partial<AccountInfoResponse>): AccountInfoResponse {
    return {
        id: id,
        name: 'Card ' + id,
        parentId: parentId,
        category: AccountCategory.CreditCard.type,
        type: AccountType.SingleAccount.type,
        icon: '1',
        iconType: 0,
        color: '000000',
        currency: currency,
        balance: balance,
        comment: '',
        displayOrder: 1,
        hidden: false,
        ...overrides
    };
}

function poolResponse(id: string, currency: string, limit: string, subAccountResponses: AccountInfoResponse[], overrides?: Partial<AccountInfoResponse>): AccountInfoResponse {
    return {
        id: id,
        name: 'Pool ' + id,
        parentId: '0',
        category: AccountCategory.CreditCard.type,
        type: AccountType.MultiSubAccounts.type,
        icon: '1',
        iconType: 0,
        color: '000000',
        currency: currency,
        balance: '0',
        comment: '',
        displayOrder: 1,
        hidden: false,
        creditCardLimit: limit,
        subAccounts: subAccountResponses,
        ...overrides
    };
}

function expectBigDecimal(value: unknown, expected: number): void {
    // a plain balance or a suffix-less display balance both represent the exact amount
    if (isBigDecimal(value)) {
        expect((value as BigDecimal).equals(expected)).toBe(true);
    } else {
        expect((value as BigDecimalWithSuffix).suffix).toBe('');
        expect((value as BigDecimalWithSuffix).value.equals(expected)).toBe(true);
    }
}

function expectTruncatedAmount(value: unknown, expected: number): void {
    // amounts converted between currencies carry fractions of a cent, which the display
    // formatter truncates, so compare the amount actually shown to the user
    const amount = isBigDecimal(value) ? value as BigDecimal : (value as BigDecimalWithSuffix).value;
    expect(amount.truncate().equals(expected)).toBe(true);
}

describe('credit card shared pool statistics (stage 3)', () => {
    beforeEach(() => {
        setActivePinia(createPinia());
        useUserStore().storeUserBasicInfo({ ...EMPTY_USER_BASIC_INFO, username: 'test', defaultCurrency: 'CNY' });
    });

    it('counts the pool limit once: limit 20000, card A spends 3000 and card B spends 5000, remaining 12000', () => {
        const pool = Account.of(poolResponse('100', 'CNY', '2000000', [
            cardResponse('101', '100', 'CNY', '-300000'),
            cardResponse('102', '100', 'CNY', '-500000')
        ]));
        const accountsStore = useAccountsStore();
        accountsStore.loadAccountList([pool]);

        // total available credit of the credit card category is 12000, not 40000 or 37000
        const categoryAvailableCredit = accountsStore.getAccountCategoryTotalBalance(true, AccountCategory.CreditCard, true);
        expectBigDecimal(categoryAvailableCredit, 1200000);

        // the remaining credit of the pool is 12000 in the pool currency
        const remainingCredit = accountsStore.getCreditCardPoolRemainingCredit(true, pool);
        expect(remainingCredit).not.toBeNull();
        expect(remainingCredit!.currency).toBe('CNY');
        expectBigDecimal(remainingCredit!.balance, 1200000);

        // used amount of the pool is 8000
        const poolBalance = accountsStore.getAccountSubAccountBalance(true, false, pool);
        expect(poolBalance).not.toBeNull();
        expect(poolBalance!.currency).toBe('CNY');
        expectBigDecimal(poolBalance!.balance, 800000);
    });

    it('counts hidden card debts into the pool remaining', () => {
        const pool = Account.of(poolResponse('100', 'CNY', '2000000', [
            cardResponse('101', '100', 'CNY', '-300000'),
            cardResponse('102', '100', 'CNY', '-500000', { hidden: true })
        ]));
        const accountsStore = useAccountsStore();
        accountsStore.loadAccountList([pool]);

        // the hidden card still occupies the shared limit, so the remaining credit is 12000
        const categoryAvailableCredit = accountsStore.getAccountCategoryTotalBalance(true, AccountCategory.CreditCard, true);
        expectBigDecimal(categoryAvailableCredit, 1200000);

        const remainingCredit = accountsStore.getCreditCardPoolRemainingCredit(true, pool);
        expect(remainingCredit).not.toBeNull();
        expectBigDecimal(remainingCredit!.balance, 1200000);

        const poolBalance = accountsStore.getAccountSubAccountBalance(true, false, pool);
        expectBigDecimal(poolBalance!.balance, 800000);
    });

    it('does not double count when several pools and single entity cards exist', () => {
        const firstPool = Account.of(poolResponse('100', 'CNY', '2000000', [
            cardResponse('101', '100', 'CNY', '-300000'),
            cardResponse('102', '100', 'CNY', '-500000')
        ]));
        const secondPool = Account.of(poolResponse('200', 'CNY', '1000000', [
            cardResponse('201', '200', 'CNY', '-100000')
        ]));
        const singleCard = Account.of(cardResponse('300', '0', 'CNY', '-22924', { creditCardLimit: '2000000' }));
        const accountsStore = useAccountsStore();
        accountsStore.loadAccountList([firstPool, secondPool, singleCard]);

        // 12000 + 9000 + 19770.76, each limit counted exactly once
        const categoryAvailableCredit = accountsStore.getAccountCategoryTotalBalance(true, AccountCategory.CreditCard, true);
        expectBigDecimal(categoryAvailableCredit, 4077076);
    });

    it('does not mark the pool amounts incomplete when foreign currency cards carry no balance', () => {
        // mirrors the real 全球支付 pool: CNY limit 30000, one CNY card in debt and two USD
        // cards with no balance, so the pool amounts are complete without the foreign cards
        const pool = Account.of(poolResponse('100', 'CNY', '3000000', [
            cardResponse('101', '100', 'CNY', '-1530'),
            cardResponse('102', '100', 'USD', '0'),
            cardResponse('103', '100', 'USD', '0')
        ]));
        const accountsStore = useAccountsStore();
        accountsStore.loadAccountList([pool]);

        const categoryAvailableCredit = accountsStore.getAccountCategoryTotalBalance(true, AccountCategory.CreditCard, true);
        expectBigDecimal(categoryAvailableCredit, 2998470);

        const remainingCredit = accountsStore.getCreditCardPoolRemainingCredit(true, pool);
        expect(remainingCredit).not.toBeNull();
        expect(remainingCredit!.currency).toBe('CNY');
        expectBigDecimal(remainingCredit!.balance, 2998470);

        const poolBalance = accountsStore.getAccountSubAccountBalance(true, false, pool);
        expectBigDecimal(poolBalance!.balance, 1530);
    });

    it('excludes a foreign currency debt which cannot be converted and marks the result incomplete', () => {
        // the USD card has no limit stated by the bank and no exchange rate is available, so
        // its debt cannot be counted into the CNY pool
        const pool = Account.of(poolResponse('100', 'CNY', '3000000', [
            cardResponse('101', '100', 'CNY', '-1530'),
            cardResponse('102', '100', 'USD', '-12345')
        ]));
        const accountsStore = useAccountsStore();
        accountsStore.loadAccountList([pool]);

        // remaining credit counts the CNY card only (2998470) and is marked incomplete
        // because the USD card debt is not counted
        const categoryAvailableCredit = accountsStore.getAccountCategoryTotalBalance(true, AccountCategory.CreditCard, true);
        expect(isBigDecimal(categoryAvailableCredit)).toBe(false);
        expect((categoryAvailableCredit as BigDecimalWithSuffix).value.equals(2998470)).toBe(true);
        expect((categoryAvailableCredit as BigDecimalWithSuffix).suffix).toBe('+');

        const remainingCredit = accountsStore.getCreditCardPoolRemainingCredit(true, pool);
        expect(remainingCredit).not.toBeNull();
        expect(remainingCredit!.currency).toBe('CNY');
        expect(isBigDecimal(remainingCredit!.balance)).toBe(false);
        expect((remainingCredit!.balance as BigDecimalWithSuffix).value.equals(2998470)).toBe(true);
        expect((remainingCredit!.balance as BigDecimalWithSuffix).suffix).toBe('+');

        // the pool balance display also excludes the foreign currency card and is marked
        const poolBalance = accountsStore.getAccountSubAccountBalance(true, false, pool);
        expect((poolBalance!.balance as BigDecimalWithSuffix).value.equals(1530)).toBe(true);
        expect((poolBalance!.balance as BigDecimalWithSuffix).suffix).toBe('+');

        // no limit is stated in USD, so the card row cannot show an amount in its own currency
        expect(accountsStore.getCreditCardPoolCardRemainingCredit(true, pool, pool.getSubAccount('102')!)).toBeNull();
    });

    it('keeps single entity card available credit unchanged', () => {
        // mirrors the real 欢享信用卡3958 account: CNY limit 20000, balance -229.24
        const singleCard = Account.of(cardResponse('300', '0', 'CNY', '-22924', { creditCardLimit: '2000000' }));
        const accountsStore = useAccountsStore();
        accountsStore.loadAccountList([singleCard]);

        const categoryAvailableCredit = accountsStore.getAccountCategoryTotalBalance(true, AccountCategory.CreditCard, true);
        expectBigDecimal(categoryAvailableCredit, 1977076);
    });

    it('keeps net assets unchanged for the pool scenario', () => {
        const pool = Account.of(poolResponse('100', 'CNY', '2000000', [
            cardResponse('101', '100', 'CNY', '-300000'),
            cardResponse('102', '100', 'CNY', '-500000')
        ]));
        const accountsStore = useAccountsStore();
        accountsStore.loadAccountList([pool]);

        const netAssets = accountsStore.getNetAssets(true, {});
        expectBigDecimal(netAssets, -800000);
    });

    it('returns each card balance when a specific sub account is requested', () => {
        const pool = Account.of(poolResponse('100', 'CNY', '2000000', [
            cardResponse('101', '100', 'CNY', '-300000'),
            cardResponse('102', '100', 'CNY', '-500000')
        ]));
        const accountsStore = useAccountsStore();
        accountsStore.loadAccountList([pool]);

        const cardABalance = accountsStore.getAccountSubAccountBalance(true, false, pool, '101');
        expect(cardABalance).not.toBeNull();
        expect(cardABalance!.currency).toBe('CNY');
        expectBigDecimal(cardABalance!.balance, 300000);

        const cardBBalance = accountsStore.getAccountSubAccountBalance(true, false, pool, '102');
        expectBigDecimal(cardBBalance!.balance, 500000);
    });

    it('returns no remaining credit for a pool without a limit', () => {
        const pool = Account.of(poolResponse('100', 'CNY', '0', [
            cardResponse('101', '100', 'CNY', '-300000')
        ]));
        const accountsStore = useAccountsStore();
        accountsStore.loadAccountList([pool]);

        expect(accountsStore.getCreditCardPoolRemainingCredit(true, pool)).toBeNull();

        // without any calculable limit the category total of available credit is not displayed
        const categoryAvailableCredit = accountsStore.getAccountCategoryTotalBalance(true, AccountCategory.CreditCard, true);
        expect(categoryAvailableCredit).toBeUndefined();
    });

    it('updates the pool remaining after a card spends more', () => {
        const cardAResponse = cardResponse('101', '100', 'CNY', '-300000');
        const pool = Account.of(poolResponse('100', 'CNY', '2000000', [
            cardAResponse,
            cardResponse('102', '100', 'CNY', '-500000')
        ]));
        const accountsStore = useAccountsStore();
        accountsStore.loadAccountList([pool]);

        const remainingCredit = accountsStore.getCreditCardPoolRemainingCredit(true, pool);
        expectBigDecimal(remainingCredit!.balance, 1200000);

        // card A spends 2000 more, so the pool remaining drops to 10000 immediately
        const updatedCardA = Account.of({ ...cardAResponse, balance: '-500000' });
        const updatedPool = Account.of(poolResponse('100', 'CNY', '2000000', [
            updatedCardA,
            cardResponse('102', '100', 'CNY', '-500000')
        ]));
        accountsStore.loadAccountList([updatedPool]);

        const updatedRemainingCredit = accountsStore.getCreditCardPoolRemainingCredit(true, updatedPool);
        expectBigDecimal(updatedRemainingCredit!.balance, 1000000);

        const updatedCategoryAvailableCredit = accountsStore.getAccountCategoryTotalBalance(true, AccountCategory.CreditCard, true);
        expectBigDecimal(updatedCategoryAvailableCredit, 1000000);
    });

    it('uses the own statement date of a card for billing cycle filters', () => {
        const pool = Account.of(poolResponse('100', 'CNY', '2000000', [
            cardResponse('101', '100', 'CNY', '-300000', { creditCardStatementDate: 10, creditCardStatementDateIncludeTransaction: true }),
            cardResponse('102', '100', 'CNY', '-500000')
        ], { creditCardStatementDate: 5 }));
        const accountsStore = useAccountsStore();
        accountsStore.loadAccountList([pool]);

        // card A has its own statement date, so it follows its own billing cycle
        expect(accountsStore.getAccountStatementDate('101')).toBe(10);
        expect(accountsStore.getAccountStatementDateIncludeTransaction('101')).toBe(true);

        // card B has no own statement date, so it falls back to the pool's statement date
        expect(accountsStore.getAccountStatementDate('102')).toBe(5);
        expect(accountsStore.getAccountStatementDateIncludeTransaction('102')).toBe(false);

        // the pool itself and a multi-account selection use the pool's statement date
        expect(accountsStore.getAccountStatementDate('100')).toBe(5);
        expect(accountsStore.getAccountStatementDate('100,101')).toBe(5);
    });

    it('does not count a hidden pool into the category total', () => {
        const pool = Account.of(poolResponse('100', 'CNY', '2000000', [
            cardResponse('101', '100', 'CNY', '-300000'),
            cardResponse('102', '100', 'CNY', '-500000')
        ], { hidden: true }));
        const singleCard = Account.of(cardResponse('300', '0', 'CNY', '-22924', { creditCardLimit: '2000000' }));
        const accountsStore = useAccountsStore();
        accountsStore.loadAccountList([pool, singleCard]);

        // the hidden pool is excluded, only the single entity card is counted
        const categoryAvailableCredit = accountsStore.getAccountCategoryTotalBalance(true, AccountCategory.CreditCard, true);
        expectBigDecimal(categoryAvailableCredit, 1977076);
    });

    it('shares the used limit between the foreign currency cards of one pool', () => {
        // mirrors the real 全球支付信用卡 pool: the bank states CNY 30000 as USD 4615 on both
        // the Mastercard and the Visa card, so they are one limit expressed in two currencies
        const pool = Account.of(poolResponse('100', 'CNY', '3000000', [
            cardResponse('101', '100', 'USD', '0', { creditCardCurrencyLimit: '4615' }),
            cardResponse('102', '100', 'USD', '-10000', { creditCardCurrencyLimit: '4615' })
        ], { creditCardIsEntityCard: false }));
        const accountsStore = useAccountsStore();
        accountsStore.loadAccountList([pool]);

        const mastercard = pool.getSubAccount('101')!;
        const visa = pool.getSubAccount('102')!;

        // the 100 USD purchase on Visa occupies the shared pool, so both cards show 4515 USD
        const visaRemaining = accountsStore.getCreditCardPoolCardRemainingCredit(true, pool, visa);
        expect(visaRemaining).not.toBeNull();
        expect(visaRemaining!.currency).toBe('USD');
        expectBigDecimal(visaRemaining!.balance, 451500);

        const mastercardRemaining = accountsStore.getCreditCardPoolCardRemainingCredit(true, pool, mastercard);
        expect(mastercardRemaining).not.toBeNull();
        expect(mastercardRemaining!.currency).toBe('USD');
        expectBigDecimal(mastercardRemaining!.balance, 451500);

        // the pool itself shows the remaining credit in CNY: 30000 - 100 * 30000 / 4615
        const poolRemaining = accountsStore.getCreditCardPoolRemainingCredit(true, pool);
        expect(poolRemaining).not.toBeNull();
        expect(poolRemaining!.currency).toBe('CNY');
        expectTruncatedAmount(poolRemaining!.balance, 2934994);

        // the used limit of the pool is the USD debt converted by the limits stated by the bank
        const poolBalance = accountsStore.getAccountSubAccountBalance(true, false, pool);
        expect(poolBalance!.currency).toBe('CNY');
        expectTruncatedAmount(poolBalance!.balance, 65005);

        // the category total counts the pool once, in the pool currency
        const categoryAvailableCredit = accountsStore.getAccountCategoryTotalBalance(true, AccountCategory.CreditCard, true);
        expectTruncatedAmount(categoryAvailableCredit, 2934994);
    });

    it('shares the used limit between cards whose stated limits differ', () => {
        // the bank states the same pool as CNY 8600 and JPY 129000, so 1 CNY = 15 JPY inside
        // this pool. The CNY card has no stated limit of its own and uses the pool limit
        const pool = Account.of(poolResponse('100', 'CNY', '860000', [
            cardResponse('101', '100', 'CNY', '-10000'),
            cardResponse('102', '100', 'JPY', '-150000', { creditCardCurrencyLimit: '129000' })
        ]));
        const accountsStore = useAccountsStore();
        accountsStore.loadAccountList([pool]);

        // used limit is 100 CNY + 1500 JPY / 15 = 200 CNY, so the pool has 8400 CNY left
        const poolRemaining = accountsStore.getCreditCardPoolRemainingCredit(true, pool);
        expectTruncatedAmount(poolRemaining!.balance, 840000);

        const cnyCardRemaining = accountsStore.getCreditCardPoolCardRemainingCredit(true, pool, pool.getSubAccount('101')!);
        expect(cnyCardRemaining!.currency).toBe('CNY');
        expectTruncatedAmount(cnyCardRemaining!.balance, 840000);

        // the JPY card shows the same shared remaining credit in JPY: 129000 - 100 * 15 - 1500
        const jpyCardRemaining = accountsStore.getCreditCardPoolCardRemainingCredit(true, pool, pool.getSubAccount('102')!);
        expect(jpyCardRemaining!.currency).toBe('JPY');
        expectBigDecimal(jpyCardRemaining!.balance, 12600000);

        const poolBalance = accountsStore.getAccountSubAccountBalance(true, false, pool);
        expectTruncatedAmount(poolBalance!.balance, 20000);
    });

    it('keeps every card of a pool in sync when one card spends more', () => {
        const visaResponse = cardResponse('102', '100', 'USD', '-10000', { creditCardCurrencyLimit: '4615' });
        const pool = Account.of(poolResponse('100', 'CNY', '3000000', [
            cardResponse('101', '100', 'USD', '0', { creditCardCurrencyLimit: '4615' }),
            visaResponse
        ]));
        const accountsStore = useAccountsStore();
        accountsStore.loadAccountList([pool]);

        expectBigDecimal(accountsStore.getCreditCardPoolCardRemainingCredit(true, pool, pool.getSubAccount('101')!)!.balance, 451500);

        // Visa spends 100 USD more, so the available credit of both cards drops to 4415 USD
        const updatedPool = Account.of(poolResponse('100', 'CNY', '3000000', [
            cardResponse('101', '100', 'USD', '0', { creditCardCurrencyLimit: '4615' }),
            Account.of({ ...visaResponse, balance: '-20000' })
        ]));
        accountsStore.loadAccountList([updatedPool]);

        expectBigDecimal(accountsStore.getCreditCardPoolCardRemainingCredit(true, updatedPool, updatedPool.getSubAccount('101')!)!.balance, 441500);
        expectBigDecimal(accountsStore.getCreditCardPoolCardRemainingCredit(true, updatedPool, updatedPool.getSubAccount('102')!)!.balance, 441500);
    });

    it('counts a hidden foreign currency card into the shared limit', () => {
        const pool = Account.of(poolResponse('100', 'CNY', '3000000', [
            cardResponse('101', '100', 'USD', '0', { creditCardCurrencyLimit: '4615' }),
            cardResponse('102', '100', 'USD', '-10000', { creditCardCurrencyLimit: '4615', hidden: true })
        ]));
        const accountsStore = useAccountsStore();
        accountsStore.loadAccountList([pool]);

        // the hidden card still occupies the shared limit, so the visible card shows 4515 USD
        const visibleCardRemaining = accountsStore.getCreditCardPoolCardRemainingCredit(true, pool, pool.getSubAccount('101')!);
        expectBigDecimal(visibleCardRemaining!.balance, 451500);
        expectTruncatedAmount(accountsStore.getCreditCardPoolRemainingCredit(true, pool)!.balance, 2934994);
    });

    it('returns the hidden amount instead of the shared remaining credit', () => {
        const pool = Account.of(poolResponse('100', 'CNY', '3000000', [
            cardResponse('101', '100', 'USD', '0', { creditCardCurrencyLimit: '4615' }),
            cardResponse('102', '100', 'USD', '-10000', { creditCardCurrencyLimit: '4615' })
        ]));
        const accountsStore = useAccountsStore();
        accountsStore.loadAccountList([pool]);

        const cardRemaining = accountsStore.getCreditCardPoolCardRemainingCredit(false, pool, pool.getSubAccount('101')!);
        expect(cardRemaining!.balance).toBe('***');
        expect(accountsStore.getCreditCardPoolRemainingCredit(false, pool)!.balance).toBe('***');
    });
});
