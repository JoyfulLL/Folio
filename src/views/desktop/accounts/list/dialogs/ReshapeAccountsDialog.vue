<template>
    <v-dialog width="560" :persistent="true" v-model="showState">
        <one-column-dialog-layout :disabled="reshaping"
                                  :title="dialogTitle"
                                  :cancel-button-title="tt('Cancel')"
                                  @cancel="cancel">
            <template #content>
                <div class="text-body-medium mb-2">{{ candidateListTitle }}</div>

                <v-list class="reshape-account-list" density="comfortable">
                    <v-list-item v-if="mode === 'group' && primaryAccount">
                        <v-checkbox :model-value="true" :disabled="true" hide-details>
                            <template #label>
                                <span class="d-flex align-center">
                                    <ItemIcon size="1.4rem"
                                              :icon-type="getAccountIconType(primaryAccount.iconType)"
                                              :icon-id="primaryAccount.icon"
                                              :color="primaryAccount.color" />
                                    <span class="ms-2">{{ primaryAccount.name }}</span>
                                </span>
                            </template>
                        </v-checkbox>
                    </v-list-item>

                    <v-list-item v-for="candidate in candidateAccounts" :key="candidate.id">
                        <v-checkbox :model-value="selectedIds.includes(candidate.id)"
                                    :disabled="reshaping"
                                    hide-details
                                    @update:model-value="toggleCandidate(candidate.id, $event)">
                            <template #label>
                                <span class="d-flex align-center">
                                    <ItemIcon size="1.4rem"
                                              :icon-type="getAccountIconType(candidate.iconType)"
                                              :icon-id="candidate.icon"
                                              :color="candidate.color" />
                                    <span class="ms-2">{{ candidate.name }}</span>
                                </span>
                            </template>
                        </v-checkbox>
                    </v-list-item>
                </v-list>

                <div class="text-body-medium mt-2" v-if="!candidateAccounts.length && !(mode === 'group' && primaryAccount)">
                    {{ tt('No available account') }}
                </div>

                <v-text-field type="text"
                              autocomplete="off"
                              persistent-placeholder
                              :disabled="reshaping"
                              :label="tt('Group Name')"
                              :placeholder="tt('Group Name')"
                              class="mt-4"
                              v-model="groupName" />
            </template>

            <template #footer>
                <v-btn color="secondary" variant="tonal" :disabled="reshaping" @click="cancel">{{ tt('Cancel') }}</v-btn>
                <v-spacer/>
                <v-btn :disabled="!canConfirm || reshaping" @click="confirm">
                    {{ tt('Confirm') }}
                    <v-progress-circular indeterminate size="22" class="ms-2" v-if="reshaping"></v-progress-circular>
                </v-btn>
            </template>
        </one-column-dialog-layout>
    </v-dialog>

    <snack-bar ref="snackbar" />
</template>

<script setup lang="ts">
import SnackBar from '@/components/desktop/SnackBar.vue';

import { ref, computed, useTemplateRef } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useAccountsStore } from '@/stores/account.ts';

import { AccountType, AccountReshapeOperation } from '@/core/account.ts';
import type { Account } from '@/models/account.ts';
import { getAccountIconType } from '@/lib/icon.ts';

type SnackBarType = InstanceType<typeof SnackBar>;

type ReshapeMode = 'group' | 'split';

const { tt } = useI18n();

const accountsStore = useAccountsStore();

let resolveFunc: (() => void) | null = null;
let rejectFunc: ((reason?: unknown) => void) | null = null;

const snackbar = useTemplateRef<SnackBarType>('snackbar');

const showState = ref<boolean>(false);
const reshaping = ref<boolean>(false);
const mode = ref<ReshapeMode>('group');
const primaryAccount = ref<Account | undefined>(undefined);
const groupName = ref<string>('');
const selectedIds = ref<string[]>([]);

const dialogTitle = computed<string>(() => mode.value === 'group' ? tt('Convert to Group') : tt('Split Group'));

const candidateListTitle = computed<string>(() => mode.value === 'group' ? tt('Select Accounts to Include') : tt('Select Sub-accounts to Include'));

const candidateAccounts = computed<Account[]>(() => {
    if (mode.value === 'group' && primaryAccount.value) {
        return accountsStore.allAccounts.filter(account =>
            account.type === AccountType.SingleAccount.type &&
            (!account.parentId || account.parentId === '0') &&
            account.category === primaryAccount.value!.category &&
            account.id !== primaryAccount.value!.id
        );
    } else if (mode.value === 'split' && primaryAccount.value) {
        return primaryAccount.value.subAccounts ?? [];
    }

    return [];
});

const canConfirm = computed<boolean>(() => {
    if (!groupName.value || !groupName.value.trim()) {
        return false;
    }

    if (mode.value === 'group') {
        return !!primaryAccount.value;
    } else if (mode.value === 'split') {
        const totalSubAccountCount = primaryAccount.value?.subAccounts?.length ?? 0;
        return selectedIds.value.length >= 1 && selectedIds.value.length < totalSubAccountCount;
    }

    return false;
});

function open(reshapeMode: ReshapeMode, account: Account): Promise<void> {
    showState.value = true;
    reshaping.value = false;
    mode.value = reshapeMode;
    primaryAccount.value = account;
    selectedIds.value = [];
    groupName.value = reshapeMode === 'group' ? account.name : '';

    return new Promise((resolve, reject) => {
        resolveFunc = resolve;
        rejectFunc = reject;
    });
}

function toggleCandidate(candidateId: string, checked: boolean | null): void {
    if (checked) {
        if (!selectedIds.value.includes(candidateId)) {
            selectedIds.value.push(candidateId);
        }
    } else {
        selectedIds.value = selectedIds.value.filter(id => id !== candidateId);
    }
}

function confirm(): void {
    if (!canConfirm.value) {
        return;
    }

    reshaping.value = true;

    let operation = AccountReshapeOperation.Group.type;
    let accountIds: string[] | undefined = undefined;
    let parentAccountId: string | undefined = undefined;
    let subAccountIds: string[] | undefined = undefined;

    if (mode.value === 'group') {
        accountIds = [primaryAccount.value!.id, ...selectedIds.value];
    } else if (mode.value === 'split') {
        operation = AccountReshapeOperation.Split.type;
        parentAccountId = primaryAccount.value!.id;
        subAccountIds = selectedIds.value;
    }

    accountsStore.reshapeAccount({
        operation: operation,
        accountIds: accountIds,
        parentAccountId: parentAccountId,
        subAccountIds: subAccountIds,
        name: groupName.value.trim()
    }).then(() => {
        reshaping.value = false;

        resolveFunc?.();
        showState.value = false;
    }).catch(error => {
        reshaping.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function cancel(): void {
    rejectFunc?.();
    showState.value = false;
}

defineExpose({
    open
});
</script>

<style scoped>
.reshape-account-list {
    max-height: 320px;
    overflow-y: auto;
}
</style>
