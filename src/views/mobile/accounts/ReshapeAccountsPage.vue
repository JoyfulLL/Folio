<template>
    <f7-page @page:afterin="onPageAfterIn">
        <f7-navbar>
            <f7-nav-left :class="{ 'disabled': reshaping }" :back-link="tt('Back')"></f7-nav-left>
            <f7-nav-title :title="pageTitle"></f7-nav-title>
            <f7-nav-right :class="{ 'disabled': loading }">
                <f7-link icon-f7="checkmark_alt"
                         :class="{ 'disabled': !canConfirm || reshaping }"
                         :aria-label="tt('Confirm')"
                         @click="confirm"></f7-link>
            </f7-nav-right>
        </f7-navbar>

        <f7-card class="margin-top-half"
                 :title="pageTitle"
                 :content="tt('Account group reshape tip')"></f7-card>

        <f7-list form strong inset dividers class="margin-vertical skeleton-text" v-if="loading">
            <f7-list-item class="list-item-with-header-and-title list-item-no-item-after" header="Group Name" title="Unspecified"></f7-list-item>
        </f7-list>

        <template v-if="!loading">
            <f7-list form strong inset dividers class="margin-vertical">
                <f7-list-input
                    type="text"
                    autocomplete="off"
                    clear-button
                    :disabled="reshaping"
                    :label="tt('Group Name')"
                    :placeholder="tt('Group Name')"
                    v-model:value="groupName"
                ></f7-list-input>
            </f7-list>

            <f7-block-title>{{ candidateListTitle }}</f7-block-title>

            <f7-list strong inset dividers class="margin-vertical">
                <f7-list-item checkbox
                              :title="account.name"
                              :checked="true"
                              :disabled="true"
                              :key="account.id"
                              v-for="account in fixedAccounts">
                    <template #media>
                        <ItemIcon :icon-type="getAccountIconType(account.iconType)"
                                  :icon-id="account.icon"
                                  :color="account.color" />
                    </template>
                </f7-list-item>

                <f7-list-item checkbox
                              :title="candidate.name"
                              :value="candidate.id"
                              :checked="selectedIds.includes(candidate.id)"
                              :disabled="reshaping"
                              :key="candidate.id"
                              v-for="candidate in candidateAccounts"
                              @change="toggleCandidate(candidate.id, $event.target.checked)">
                    <template #media>
                        <ItemIcon :icon-type="getAccountIconType(candidate.iconType)"
                                  :icon-id="candidate.icon"
                                  :color="candidate.color" />
                    </template>
                </f7-list-item>

                <f7-list-item :title="tt('No available account')"
                              v-if="!candidateAccounts.length && !fixedAccounts.length"></f7-list-item>
            </f7-list>
        </template>
    </f7-page>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import type { Router } from 'framework7/types';

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents, showLoading, hideLoading } from '@/lib/ui/mobile.ts';

import { useAccountsStore } from '@/stores/account.ts';

import { AccountType, AccountCategory, AccountReshapeOperation } from '@/core/account.ts';
import type { Account } from '@/models/account.ts';
import { getAccountIconType } from '@/lib/icon.ts';

const props = defineProps<{
    f7route: Router.Route;
    f7router: Router.Router;
}>();

const { tt } = useI18n();
const { showToast, routeBackOnError } = useI18nUIComponents();

const accountsStore = useAccountsStore();

const loading = ref<boolean>(true);
const loadingError = ref<unknown | null>(null);
const reshaping = ref<boolean>(false);
const mode = ref<'group' | 'split'>('group');
const primaryAccount = ref<Account | null>(null);
const groupName = ref<string>('');
const selectedIds = ref<string[]>([]);

const pageTitle = computed<string>(() => mode.value === 'group' ? tt('Convert to Group') : tt('Split Group'));
const candidateListTitle = computed<string>(() => mode.value === 'group' ? tt('Select Accounts to Include') : tt('Select Sub-accounts to Include'));

const fixedAccounts = computed<Account[]>(() => mode.value === 'group' && primaryAccount.value ? [primaryAccount.value] : []);

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
    }

    const totalSubAccountCount = primaryAccount.value?.subAccounts?.length ?? 0;
    return selectedIds.value.length >= 1 && selectedIds.value.length < totalSubAccountCount;
});

function init(): void {
    const query = props.f7route.query;
    const accountId = query['accountId'];
    const reshapeMode = query['mode'];

    if (!accountId || (reshapeMode !== 'group' && reshapeMode !== 'split')) {
        showToast('Parameter Invalid');
        loadingError.value = 'Parameter Invalid';
        loading.value = false;
        return;
    }

    mode.value = reshapeMode === 'split' ? 'split' : 'group';

    accountsStore.loadAllAccounts({
        force: false
    }).then(() => {
        loading.value = false;

        const account = accountsStore.allAccountsMap[accountId];

        if (!account ||
            account.category === AccountCategory.CreditCard.type ||
            (mode.value === 'group' && account.type !== AccountType.SingleAccount.type) ||
            (mode.value === 'split' && (account.type !== AccountType.MultiSubAccounts.type || (account.subAccounts?.length ?? 0) < 2))) {
            showToast('Parameter Invalid');
            loadingError.value = 'Parameter Invalid';
            return;
        }

        primaryAccount.value = account;
        groupName.value = mode.value === 'group' ? account.name : '';
    }).catch(error => {
        if (error.processed) {
            loading.value = false;
        } else {
            loadingError.value = error;
            showToast(error.message || error);
        }
    });
}

function toggleCandidate(candidateId: string, checked: boolean): void {
    if (checked) {
        if (!selectedIds.value.includes(candidateId)) {
            selectedIds.value.push(candidateId);
        }
    } else {
        selectedIds.value = selectedIds.value.filter(id => id !== candidateId);
    }
}

function confirm(): void {
    if (!canConfirm.value || reshaping.value || !primaryAccount.value) {
        return;
    }

    reshaping.value = true;
    showLoading(() => reshaping.value);

    let operation = AccountReshapeOperation.Group.type;
    let accountIds: string[] | undefined = undefined;
    let parentAccountId: string | undefined = undefined;
    let subAccountIds: string[] | undefined = undefined;

    if (mode.value === 'group') {
        accountIds = [primaryAccount.value.id, ...selectedIds.value];
    } else {
        operation = AccountReshapeOperation.Split.type;
        parentAccountId = primaryAccount.value.id;
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
        hideLoading();

        showToast('Account group has been created');
        props.f7router.back();
    }).catch(error => {
        reshaping.value = false;
        hideLoading();

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function onPageAfterIn(): void {
    routeBackOnError(props.f7router, loadingError);
}

init();
</script>
