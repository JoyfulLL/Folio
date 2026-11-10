import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import moment from 'moment-timezone';

import { DateRange } from '@/core/datetime.ts';
import type { WeekDayValue } from '@/core/datetime.ts';
import {
    getDateRangeByBillingCycleDateType,
    getBillingCycleFirstUnixTimeBySpecifiedUnixTime
} from '@/lib/datetime.ts';

function localUnix(year: number, month1based: number, day: number, hour: number, minute: number, second: number): number {
    return moment().set({ year, month: month1based - 1, date: day, hour, minute, second, millisecond: 0 }).unix();
}

describe('billing cycle include-statement-date switch', () => {
    beforeEach(() => {
        vi.useFakeTimers();
    });

    afterEach(() => {
        vi.useRealTimers();
    });

    it('OFF, today Nov 9, statement day 10: current cycle is Oct 10 - Nov 9', () => {
        vi.setSystemTime(new Date(localUnix(2026, 11, 9, 12, 0, 0) * 1000));
        const range = getDateRangeByBillingCycleDateType(DateRange.CurrentBillingCycle.type, 0 as WeekDayValue, 0, 10, false);
        expect(range?.minTime).toBe(localUnix(2026, 10, 10, 0, 0, 0));
        expect(range?.maxTime).toBe(localUnix(2026, 11, 9, 23, 59, 59));
    });

    it('ON, today Nov 9, statement day 10: current cycle is Oct 11 - Nov 10', () => {
        vi.setSystemTime(new Date(localUnix(2026, 11, 9, 12, 0, 0) * 1000));
        const range = getDateRangeByBillingCycleDateType(DateRange.CurrentBillingCycle.type, 0 as WeekDayValue, 0, 10, true);
        expect(range?.minTime).toBe(localUnix(2026, 10, 11, 0, 0, 0));
        expect(range?.maxTime).toBe(localUnix(2026, 11, 10, 23, 59, 59));
    });

    it('OFF, today Nov 10, statement day 10: current cycle is Nov 10 - Dec 9', () => {
        vi.setSystemTime(new Date(localUnix(2026, 11, 10, 12, 0, 0) * 1000));
        const range = getDateRangeByBillingCycleDateType(DateRange.CurrentBillingCycle.type, 0 as WeekDayValue, 0, 10, false);
        expect(range?.minTime).toBe(localUnix(2026, 11, 10, 0, 0, 0));
        expect(range?.maxTime).toBe(localUnix(2026, 12, 9, 23, 59, 59));
    });

    it('OFF: Nov 10 transaction belongs to cycle starting Nov 10', () => {
        const first = getBillingCycleFirstUnixTimeBySpecifiedUnixTime(localUnix(2026, 11, 10, 15, 2, 29), 10, undefined, false);
        expect(first.getUnixTime()).toBe(localUnix(2026, 11, 10, 0, 0, 0));
    });

    it('ON: Nov 10 transaction belongs to cycle starting Oct 11', () => {
        const first = getBillingCycleFirstUnixTimeBySpecifiedUnixTime(localUnix(2026, 11, 10, 15, 2, 29), 10, undefined, true);
        expect(first.getUnixTime()).toBe(localUnix(2026, 10, 11, 0, 0, 0));
    });
});