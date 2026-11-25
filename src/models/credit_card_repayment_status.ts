export enum CreditCardRepaymentStatusSource {
    Auto = 1,
    Manual = 2
}

export interface CreditCardRepaymentStatusRequest {
    readonly id: string;
    readonly scheduledDate?: number;
}

export interface CreditCardRepaymentStatusResponse {
    readonly scheduledDate: number;
    readonly statementDate: number;
    readonly paid: boolean;
    readonly source: CreditCardRepaymentStatusSource;
}
