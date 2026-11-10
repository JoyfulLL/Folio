import { describe, expect, it } from 'vitest';

import { AccountCategory } from '@/core/account.ts';

import { Account } from '@/models/account.ts';

function createAccount(id: string, category: number, displayOrder: number, parentId: string = '0'): Account {
    return Account.of({
        id: id,
        name: id,
        parentId: parentId,
        category: category,
        type: 1,
        icon: '1',
        iconType: 0,
        color: '000000',
        currency: 'USD',
        balance: '0',
        comment: '',
        displayOrder: displayOrder,
        hidden: false
    });
}

describe('Account.compareTo', () => {
    it('should return zero when comparing an account with itself', () => {
        const account = createAccount('1', AccountCategory.Cash.type, 10);

        expect(account.compareTo(account, { [AccountCategory.Cash.type]: 1 })).toBe(0);
    });

    it('should compare category display orders before account display orders', () => {
        const first = createAccount('1', AccountCategory.Cash.type, 1);
        const second = createAccount('2', AccountCategory.CheckingAccount.type, 10);
        const categoryDisplayOrders = { [AccountCategory.Cash.type]: 2, [AccountCategory.CheckingAccount.type]: 1 };

        expect(first.compareTo(second, categoryDisplayOrders)).toBe(1);
        expect(second.compareTo(first, categoryDisplayOrders)).toBe(-1);
    });

    it('should place an account with an unspecified category order after an account with a specified category order', () => {
        const first = createAccount('1', AccountCategory.Cash.type, 1);
        const second = createAccount('2', AccountCategory.CheckingAccount.type, 10);
        const categoryDisplayOrders = { [AccountCategory.CheckingAccount.type]: 1 };

        expect(first.compareTo(second, categoryDisplayOrders)).toBe(1);
        expect(second.compareTo(first, categoryDisplayOrders)).toBe(-1);
    });

    it.each([
        [1, 2, -1],
        [2, 1, 1],
        [2, 2, 0],
    ])('should compare top-level account display orders %j and %j', (firstOrder, secondOrder, expected) => {
        const first = createAccount('1', AccountCategory.Cash.type, firstOrder);
        const second = createAccount('2', AccountCategory.Cash.type, secondOrder);

        expect(first.compareTo(second, { [AccountCategory.Cash.type]: 1 })).toBe(expected);
    });

    it.each([
        [1, 2, -1],
        [2, 1, 1],
        [2, 2, 0],
    ])('should compare sibling account display orders %j and %j', (firstOrder, secondOrder, expected) => {
        const first = createAccount('11', AccountCategory.Cash.type, firstOrder, '1');
        const second = createAccount('12', AccountCategory.Cash.type, secondOrder, '1');

        expect(first.compareTo(second, { [AccountCategory.Cash.type]: 1 })).toBe(expected);
    });

    it('should place a parent account before its child regardless of their display orders', () => {
        const parent = createAccount('1', AccountCategory.Cash.type, 10);
        const child = createAccount('11', AccountCategory.Cash.type, 1, '1');

        expect(parent.compareTo(child, { [AccountCategory.Cash.type]: 1 })).toBe(-1);
        expect(child.compareTo(parent, { [AccountCategory.Cash.type]: 1 })).toBe(1);
    });

    it('should compare children of different parents using parent display orders', () => {
        const firstParent = createAccount('1', AccountCategory.Cash.type, 1);
        const secondParent = createAccount('2', AccountCategory.Cash.type, 2);
        const firstChild = createAccount('11', AccountCategory.Cash.type, 10, '1');
        const secondChild = createAccount('21', AccountCategory.Cash.type, 1, '2');
        const allAccountsMap = { '1': firstParent, '2': secondParent };

        expect(firstChild.compareTo(secondChild, { [AccountCategory.Cash.type]: 1 }, allAccountsMap)).toBe(-1);
        expect(secondChild.compareTo(firstChild, { [AccountCategory.Cash.type]: 1 }, allAccountsMap)).toBe(1);
    });

    it('should compare a child with an unrelated top-level account using its parent display order', () => {
        const parent = createAccount('1', AccountCategory.Cash.type, 1);
        const child = createAccount('11', AccountCategory.Cash.type, 10, '1');
        const other = createAccount('2', AccountCategory.Cash.type, 2);
        const allAccountsMap = { '1': parent };

        expect(child.compareTo(other, { [AccountCategory.Cash.type]: 1 }, allAccountsMap)).toBe(-1);
        expect(other.compareTo(child, { [AccountCategory.Cash.type]: 1 }, allAccountsMap)).toBe(1);
    });

    it('should return zero when different parents have equal display orders', () => {
        const firstParent = createAccount('1', AccountCategory.Cash.type, 2);
        const secondParent = createAccount('2', AccountCategory.Cash.type, 2);
        const firstChild = createAccount('11', AccountCategory.Cash.type, 10, '1');
        const secondChild = createAccount('21', AccountCategory.Cash.type, 1, '2');
        const allAccountsMap = { '1': firstParent, '2': secondParent };

        expect(firstChild.compareTo(secondChild, { [AccountCategory.Cash.type]: 1 }, allAccountsMap)).toBe(0);
    });

    it('should compare account IDs when the account map is not provided', () => {
        const first = createAccount('11', AccountCategory.Cash.type, 10, '1');
        const second = createAccount('21', AccountCategory.Cash.type, 1, '2');

        expect(first.compareTo(second, { [AccountCategory.Cash.type]: 1 })).toBeLessThan(0);
        expect(second.compareTo(first, { [AccountCategory.Cash.type]: 1 })).toBeGreaterThan(0);
    });

    it('should compare account IDs when either parent is missing from the account map', () => {
        const firstParent = createAccount('1', AccountCategory.Cash.type, 10);
        const secondParent = createAccount('2', AccountCategory.Cash.type, 1);
        const firstChild = createAccount('11', AccountCategory.Cash.type, 10, '1');
        const secondChild = createAccount('21', AccountCategory.Cash.type, 1, '2');

        expect(firstChild.compareTo(secondChild, { [AccountCategory.Cash.type]: 1 }, { '1': firstParent })).toBeLessThan(0);
        expect(secondChild.compareTo(firstChild, { [AccountCategory.Cash.type]: 1 }, { '1': firstParent })).toBeGreaterThan(0);
        expect(firstChild.compareTo(secondChild, { [AccountCategory.Cash.type]: 1 }, { '2': secondParent })).toBeLessThan(0);
        expect(secondChild.compareTo(firstChild, { [AccountCategory.Cash.type]: 1 }, { '2': secondParent })).toBeGreaterThan(0);
    });
});

describe('Account credit card shared pool (stage 1)', () => {
    const poolResponse = {
        id: '100',
        name: 'Global Payment',
        parentId: '0',
        category: AccountCategory.CreditCard.type,
        type: 2,
        icon: '1',
        iconType: 0,
        color: '000000',
        currency: 'CNY',
        balance: '0',
        comment: '',
        displayOrder: 1,
        hidden: false,
        creditCardStatementDate: 5,
        creditCardLimit: '3000000',
        creditCardBankName: 'ABC',
        creditCardCardName: 'Mastercard 5083',
        creditCardLastFourDigits: '5083',
        creditCardPictureId: '9001',
        creditCardStatementEmailEnabled: true,
        creditCardNotificationMinutesOfDay: 540,
        creditCardNotificationTimezoneName: 'Asia/Shanghai',
        creditCardNotificationTimezoneUtcOffset: 480
    };

    const cardResponse = {
        id: '101',
        name: 'Mastercard 5083 CNY',
        parentId: '100',
        category: AccountCategory.CreditCard.type,
        type: 1,
        icon: '1',
        iconType: 0,
        color: '000000',
        currency: 'CNY',
        balance: '-1530',
        comment: '',
        displayOrder: 1,
        hidden: false,
        creditCardStatementDate: 15,
        creditCardBankName: 'ABC',
        creditCardCardName: 'Mastercard 5083',
        creditCardLastFourDigits: '5083',
        creditCardPictureId: '9002',
        creditCardStatementReminderEnabled: true,
        creditCardNotificationTimezoneName: 'Asia/Shanghai',
        creditCardCurrencyLimit: '30000'
    };

    it('should keep existing card profile, picture and notification settings on the pool', () => {
        const pool = Account.of(poolResponse);
        const request = pool.toModifyRequest('session');

        expect(request.creditCardLimit).toBe('3000000');
        expect(request.creditCardBankName).toBe('ABC');
        expect(request.creditCardCardName).toBe('Mastercard 5083');
        expect(request.creditCardLastFourDigits).toBe('5083');
        expect(request.creditCardPictureId).toBe('9001');
        expect(request.creditCardStatementEmailEnabled).toBe(true);
        expect(request.creditCardNotificationMinutesOfDay).toBe(540);
        expect(request.creditCardNotificationTimezoneName).toBe('Asia/Shanghai');
        expect(request.creditCardIsEntityCard).toBeUndefined();
        expect(request.creditCardCurrencyLimit).toBeUndefined();
    });

    it('should let a sub-account carry its own statement date and card profile', () => {
        const card = Account.of(cardResponse);
        const pool = Account.of(poolResponse);
        const request = card.toModifyRequest('session', undefined, pool);

        expect(request.creditCardLimit).toBeUndefined();
        expect(request.creditCardStatementDate).toBe(15);
        expect(request.creditCardBankName).toBe('ABC');
        expect(request.creditCardPictureId).toBe('9002');
        expect(request.creditCardStatementReminderEnabled).toBe(true);
        expect(request.creditCardNotificationTimezoneName).toBe('Asia/Shanghai');
        expect(request.creditCardCurrencyLimit).toBe('30000');
        expect(request.creditCardIsEntityCard).toBeUndefined();
    });

    it('should never copy the pool card profile to a newly added sub-account', () => {
        const pool = Account.of(poolResponse);
        const newCard = pool.createNewSubAccount('USD', 0);
        const request = newCard.toCreateRequest('session', undefined, pool);

        expect(request.creditCardLimit).toBeUndefined();
        expect(request.creditCardBankName).toBeUndefined();
        expect(request.creditCardPictureId).toBe('0');
        expect(request.creditCardStatementEmailEnabled).toBeUndefined();
        expect(request.creditCardNotificationTimezoneName).toBeUndefined();
    });

    it('should mark a pool as a pure credit limit container only when explicitly set', () => {
        const pool = Account.of(poolResponse);
        const request = pool.toModifyRequest('session');

        expect(pool.creditCardIsEntityCard).toBeUndefined();

        pool.creditCardIsEntityCard = false;

        expect(pool.toModifyRequest('session').creditCardIsEntityCard).toBe(false);
        expect(request.creditCardIsEntityCard).toBeUndefined();
    });
});

describe('Account credit card own currency limit', () => {
    const ownLimitCardBase = {
        id: '301',
        name: 'Card 301',
        parentId: '0',
        category: AccountCategory.CreditCard.type,
        type: 1,
        icon: '1',
        iconType: 0,
        color: '000000',
        currency: 'JPY',
        balance: '0',
        comment: '',
        displayOrder: 1,
        hidden: false
    };

    function ownLimitCardResponse(currency: string, limit: string) {
        return {
            ...ownLimitCardBase,
            currency: currency,
            creditCardCurrencyLimit: limit
        };
    }

    it('should convert the own currency limit text into the stored amount unit', () => {
        // all currencies are stored with two decimal places, so 129,000 yen becomes 12900000
        const jpyCard = Account.of(ownLimitCardResponse('JPY', '129,000'));
        expect(jpyCard.numericCreditCardCurrencyLimit).toBe(12900000);

        // 30,000 yuan becomes 3000000 cents
        const cnyCard = Account.of(ownLimitCardResponse('CNY', '30,000'));
        expect(cnyCard.numericCreditCardCurrencyLimit).toBe(3000000);
    });

    it('should tolerate separators, spaces and full-width digits', () => {
        const plainCard = Account.of(ownLimitCardResponse('JPY', '129000'));
        expect(plainCard.numericCreditCardCurrencyLimit).toBe(12900000);

        const fullWidthCard = Account.of(ownLimitCardResponse('JPY', '１２９，０００'));
        expect(fullWidthCard.numericCreditCardCurrencyLimit).toBe(12900000);

        const dunHaoCard = Account.of(ownLimitCardResponse('JPY', '129、000'));
        expect(dunHaoCard.numericCreditCardCurrencyLimit).toBe(12900000);
    });

    it('should return zero for missing or invalid own currency limits', () => {
        const cardWithoutLimit = Account.of(ownLimitCardResponse('JPY', ''));
        expect(cardWithoutLimit.numericCreditCardCurrencyLimit).toBe(0);

        const invalidCard = Account.of(ownLimitCardResponse('JPY', 'not-a-number'));
        expect(invalidCard.numericCreditCardCurrencyLimit).toBe(0);
    });
});
