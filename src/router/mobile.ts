import type { Router } from 'framework7/types';

import { isUserLogined, isUserUnlocked } from '@/lib/userstate.ts';

/**
 * First-paint critical pages stay as static imports (wrapped in a resolved loader) so the
 * initial route needs no extra network round-trip. Every other page is loaded on demand,
 * which keeps the mobile entry bundle small and shortens the pre-render window.
 */
import HomeComponent from '@/views/mobile/HomePage.vue';
import LoginComponent from '@/views/mobile/LoginPage.vue';
import SignUpComponent from '@/views/mobile/SignupPage.vue';
import UnlockComponent from '@/views/mobile/UnlockPage.vue';

const HomePage = () => Promise.resolve(HomeComponent);
const LoginPage = () => Promise.resolve(LoginComponent);
const SignUpPage = () => Promise.resolve(SignUpComponent);
const UnlockPage = () => Promise.resolve(UnlockComponent);

const TransactionListPage = () => import('@/views/mobile/transactions/ListPage.vue').then(m => m.default);
const TransactionEditPage = () => import('@/views/mobile/transactions/EditPage.vue').then(m => m.default);
const TransactionAmountFilterPage = () => import('@/views/mobile/transactions/AmountFilterPage.vue').then(m => m.default);

const AccountListPage = () => import('@/views/mobile/accounts/ListPage.vue').then(m => m.default);
const AccountEditPage = () => import('@/views/mobile/accounts/EditPage.vue').then(m => m.default);
const AccountReconciliationStatementPage = () => import('@/views/mobile/accounts/ReconciliationStatementPage.vue').then(m => m.default);
const AccountMoveAllTransactionsPage = () => import('@/views/mobile/accounts/MoveAllTransactionsPage.vue').then(m => m.default);
const AccountReshapeAccountsPage = () => import('@/views/mobile/accounts/ReshapeAccountsPage.vue').then(m => m.default);

const StatisticsTransactionPage = () => import('@/views/mobile/statistics/TransactionPage.vue').then(m => m.default);
const StatisticsSettingsPage = () => import('@/views/mobile/statistics/SettingsPage.vue').then(m => m.default);

const TextSizeSettingsPage = () => import('@/views/mobile/settings/TextSizeSettingsPage.vue').then(m => m.default);
const PreferencesSettingsPage = () => import('@/views/mobile/settings/PreferencesSettingsPage.vue').then(m => m.default);
const OverviewLayoutEditorPage = () => import('@/views/mobile/overview/LayoutEditorPage.vue').then(m => m.default);
const ChartColorSchemeSettingsPage = () => import('@/views/mobile/settings/ChartColorSchemeSettingsPage.vue').then(m => m.default);
const AccountCategoryDisplayOrderSettingsPage = () => import('@/views/mobile/settings/AccountCategoryDisplayOrderSettingsPage.vue').then(m => m.default);
const ApplicationCloudSyncSettingsPage = () => import('@/views/mobile/settings/ApplicationCloudSyncSettingsPage.vue').then(m => m.default);
const BrowserCacheSettingPage = () => import('@/views/mobile/settings/BrowserCacheSettingPage.vue').then(m => m.default);
const AccountFilterSettingsPage = () => import('@/views/mobile/settings/AccountFilterSettingsPage.vue').then(m => m.default);
const CategoryFilterSettingsPage = () => import('@/views/mobile/settings/CategoryFilterSettingsPage.vue').then(m => m.default);
const TransactionTagFilterSettingsPage = () => import('@/views/mobile/settings/TransactionTagFilterSettingsPage.vue').then(m => m.default);

const SettingsPage = () => import('@/views/mobile/SettingsPage.vue').then(m => m.default);
const ApplicationLockPage = () => import('@/views/mobile/ApplicationLockPage.vue').then(m => m.default);
const ExchangeRatesListPage = () => import('@/views/mobile/exchangerates/ListPage.vue').then(m => m.default);
const ExchangeRatesUpdatePage = () => import('@/views/mobile/exchangerates/UpdatePage.vue').then(m => m.default);
const AboutPage = () => import('@/views/mobile/AboutPage.vue').then(m => m.default);

const UserProfilePage = () => import('@/views/mobile/users/UserProfilePage.vue').then(m => m.default);
const DataManagementPage = () => import('@/views/mobile/users/DataManagementPage.vue').then(m => m.default);
const TwoFactorAuthPage = () => import('@/views/mobile/users/TwoFactorAuthPage.vue').then(m => m.default);
const SessionListPage = () => import('@/views/mobile/users/SessionListPage.vue').then(m => m.default);

const CategoryAllPage = () => import('@/views/mobile/categories/AllPage.vue').then(m => m.default);
const CategoryListPage = () => import('@/views/mobile/categories/ListPage.vue').then(m => m.default);
const CategoryEditPage = () => import('@/views/mobile/categories/EditPage.vue').then(m => m.default);
const CategoryPresetPage = () => import('@/views/mobile/categories/PresetPage.vue').then(m => m.default);

const TagListPage = () => import('@/views/mobile/tags/ListPage.vue').then(m => m.default);
const TagGroupListPage = () => import('@/views/mobile/tags/GroupListPage.vue').then(m => m.default);

const TemplateListPage = () => import('@/views/mobile/templates/ListPage.vue').then(m => m.default);

type PageLoader = () => Promise<unknown>;

function asyncResolve(loader: PageLoader): (ctx: Router.RouteCallbackCtx) => void {
    return function({ resolve, reject }: { resolve: ({ component }: { component: unknown }) => void, reject: () => void }): void {
        loader().then(component => {
            resolve({ component: component });
        }).catch(error => {
            console.error('failed to load page component', error);
            reject();
        });
    } as unknown as (ctx: Router.RouteCallbackCtx) => void;
}

function checkLogin({ router, resolve, reject }: { router: Router.Router, resolve: () => void, reject: () => void }): void {
    if (!isUserLogined()) {
        reject();
        router.navigate('/login', {
            clearPreviousHistory: true,
            browserHistory: false
        });
        return;
    }

    if (!isUserUnlocked()) {
        reject();
        router.navigate('/unlock', {
            clearPreviousHistory: true,
            browserHistory: false
        });
        return;
    }

    resolve();
}

function checkLocked({ router, resolve, reject }: { router: Router.Router, resolve: () => void, reject: () => void }): void {
    if (!isUserLogined()) {
        reject();
        router.navigate('/login', {
            clearPreviousHistory: true,
            browserHistory: false
        });
        return;
    }

    if (isUserUnlocked()) {
        reject();
        router.navigate('/', {
            clearPreviousHistory: true,
            browserHistory: false
        });
        return;
    }

    resolve();
}

function checkNotLogin({ router, resolve, reject }: { router: Router.Router, resolve: () => void, reject: () => void }): void {
    if (isUserLogined() && !isUserUnlocked()) {
        reject();
        router.navigate('/unlock', {
            clearPreviousHistory: true,
            browserHistory: false
        });
        return;
    }

    if (isUserLogined()) {
        reject();
        router.navigate('/', {
            clearPreviousHistory: true,
            browserHistory: false
        });
        return;
    }

    resolve();
}

const routes: Router.RouteParameters[] = [
    {
        path: '/',
        async: asyncResolve(HomePage),
        beforeEnter: [checkLogin],
        options: {
            animate: false,
        }
    },
    {
        path: '/login',
        async: asyncResolve(LoginPage),
        beforeEnter: [checkNotLogin],
        options: {
            animate: false,
        }
    },
    {
        path: '/signup',
        async: asyncResolve(SignUpPage),
        beforeEnter: [checkNotLogin],
        options: {
            animate: false,
        }
    },
    {
        path: '/unlock',
        async: asyncResolve(UnlockPage),
        beforeEnter: [checkLocked],
        options: {
            animate: false,
        }
    },
    {
        path: '/transaction/list',
        async: asyncResolve(TransactionListPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/transaction/filter/amount',
        async: asyncResolve(TransactionAmountFilterPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/transaction/add',
        async: asyncResolve(TransactionEditPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/transaction/edit',
        async: asyncResolve(TransactionEditPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/transaction/detail',
        async: asyncResolve(TransactionEditPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/account/list',
        async: asyncResolve(AccountListPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/account/add',
        async: asyncResolve(AccountEditPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/account/edit',
        async: asyncResolve(AccountEditPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/account/reconciliation_statements',
        async: asyncResolve(AccountReconciliationStatementPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/account/move_all_transactions',
        async: asyncResolve(AccountMoveAllTransactionsPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/account/reshape',
        async: asyncResolve(AccountReshapeAccountsPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/statistic/transaction',
        async: asyncResolve(StatisticsTransactionPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/statistic/settings',
        async: asyncResolve(StatisticsSettingsPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/settings/textsize',
        async: asyncResolve(TextSizeSettingsPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/settings/filter/account',
        async: asyncResolve(AccountFilterSettingsPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/settings/filter/category',
        async: asyncResolve(CategoryFilterSettingsPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/settings/filter/tag',
        async: asyncResolve(TransactionTagFilterSettingsPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/settings/preferences',
        async: asyncResolve(PreferencesSettingsPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/settings/overview_layout',
        async: asyncResolve(OverviewLayoutEditorPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/settings/chart_color_scheme',
        async: asyncResolve(ChartColorSchemeSettingsPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/settings/account_category_display_order',
        async: asyncResolve(AccountCategoryDisplayOrderSettingsPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/settings/sync',
        async: asyncResolve(ApplicationCloudSyncSettingsPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/settings/browser_caches',
        async: asyncResolve(BrowserCacheSettingPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/settings',
        async: asyncResolve(SettingsPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/app_lock',
        async: asyncResolve(ApplicationLockPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/exchange_rates',
        async: asyncResolve(ExchangeRatesListPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/exchange_rates/update',
        async: asyncResolve(ExchangeRatesUpdatePage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/about',
        async: asyncResolve(AboutPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/user/profile',
        async: asyncResolve(UserProfilePage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/user/data/management',
        async: asyncResolve(DataManagementPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/user/2fa',
        async: asyncResolve(TwoFactorAuthPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/user/sessions',
        async: asyncResolve(SessionListPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/category/all',
        async: asyncResolve(CategoryAllPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/category/list',
        async: asyncResolve(CategoryListPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/category/add',
        async: asyncResolve(CategoryEditPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/category/edit',
        async: asyncResolve(CategoryEditPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/category/preset',
        async: asyncResolve(CategoryPresetPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/tag/list',
        async: asyncResolve(TagListPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/tag/group/list',
        async: asyncResolve(TagGroupListPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/template/list',
        async: asyncResolve(TemplateListPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/schedule/list',
        async: asyncResolve(TemplateListPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/template/add',
        async: asyncResolve(TransactionEditPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/template/edit',
        async: asyncResolve(TransactionEditPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '(.*)',
        redirect: '/'
    }
];

export default routes;
