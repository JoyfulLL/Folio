import { type NavigationGuardReturn, createRouter, createWebHashHistory } from 'vue-router';

import { TemplateType } from '@/core/template.ts';
import { isUserLogined, isUserUnlocked } from '@/lib/userstate.ts';

/**
 * First-paint critical pages stay as static imports so the initial route does not need
 * an extra network round-trip. Everything else is loaded on demand to keep the entry
 * bundle small and shorten the window before the app shell can render.
 */
import LoginPage from '@/views/desktop/LoginPage.vue';
import SignUpPage from '@/views/desktop/SignupPage.vue';
import VerifyEmailPage from '@/views/desktop/VerifyEmailPage.vue';
import ForgetPasswordPage from '@/views/desktop/ForgetPasswordPage.vue';
import ResetPasswordPage from '@/views/desktop/ResetPasswordPage.vue';
import OAuth2CallbackPage from '@/views/desktop/OAuth2CallbackPage.vue';
import UnlockPage from '@/views/desktop/UnlockPage.vue';

import HomePage from '@/views/desktop/HomePage.vue';

const TransactionListPage = () => import('@/views/desktop/transactions/ListPage.vue');
const StatisticsTransactionPage = () => import('@/views/desktop/statistics/TransactionPage.vue');
const InsightsExplorerPage = () => import('@/views/desktop/insights/ExplorerPage.vue');
const AccountListPage = () => import('@/views/desktop/accounts/ListPage.vue');
const TransactionCategoryListPage = () => import('@/views/desktop/categories/ListPage.vue');
const TransactionTagListPage = () => import('@/views/desktop/tags/ListPage.vue');
const TransactionTemplateListPage = () => import('@/views/desktop/templates/ListPage.vue');
const UserCustomIconListPage = () => import('@/views/desktop/customicons/ListPage.vue');
const ImportPresetsListPage = () => import('@/views/desktop/presets/ImportPresetsListPage.vue');
const OverviewLayoutEditorPage = () => import('@/views/desktop/overview/LayoutEditorPage.vue');
const SettingsPageLayout = () => import('@/views/desktop/settings/SettingsPageLayout.vue');
const UserBasicSettingPage = () => import('@/views/desktop/users/UserBasicSettingPage.vue');
const UserSecuritySettingPage = () => import('@/views/desktop/users/UserSecuritySettingPage.vue');
const TwoFactorAuthPage = () => import('@/views/desktop/users/TwoFactorAuthPage.vue');
const DataManagementPage = () => import('@/views/desktop/users/DataManagementPage.vue');
const PreferencesSettingsPage = () => import('@/views/desktop/settings/PreferencesSettingsPage.vue');
const ApplicationLockPage = () => import('@/views/desktop/settings/ApplicationLockPage.vue');
const StatisticsSettingPage = () => import('@/views/desktop/settings/StatisticsSettingPage.vue');
const ApplicationCloudSyncSettingsPage = () => import('@/views/desktop/settings/ApplicationCloudSyncSettingsPage.vue');
const BrowserCacheSettingPage = () => import('@/views/desktop/settings/BrowserCacheSettingPage.vue');
const DeveloperToolsPage = () => import('@/views/desktop/settings/DeveloperToolsPage.vue');
const ExchangeRatesListPage = () => import('@/views/desktop/exchangerates/ListPage.vue');

function checkLogin(): NavigationGuardReturn {
    if (!isUserLogined()) {
        return {
            path: '/login',
            replace: true
        };
    }

    if (!isUserUnlocked()) {
        return {
            path: '/unlock',
            replace: true
        };
    }

    return true;
}

function checkLocked(): NavigationGuardReturn {
    if (!isUserLogined()) {
        return {
            path: '/login',
            replace: true
        };
    }

    if (isUserUnlocked()) {
        return {
            path: '/',
            replace: true
        };
    }

    return true;
}

function checkNotLogin(): NavigationGuardReturn {
    if (isUserLogined() && !isUserUnlocked()) {
        return {
            path: '/unlock',
            replace: true
        };
    }

    if (isUserLogined()) {
        return {
            path: '/',
            replace: true
        };
    }

    return true;
}

const router = createRouter({
    history: createWebHashHistory(),
    routes: [
        {
            path: '/',
            component: HomePage,
            beforeEnter: checkLogin
        },
        {
            path: '/transaction/list',
            component: TransactionListPage,
            beforeEnter: checkLogin,
            props: route => ({
                initPageType: route.query['pageType'],
                initDateType: route.query['dateType'],
                initMaxTime: route.query['maxTime'],
                initMinTime: route.query['minTime'],
                initType: route.query['type'],
                initCategoryIds: route.query['categoryIds'],
                initAccountIds: route.query['accountIds'],
                initTagFilter: route.query['tagFilter'],
                initAmountFilter: route.query['amountFilter'],
                initKeyword: route.query['keyword'],
                initMatchMode: route.query['matchMode']
            })
        },
        {
            path: '/statistics/transaction',
            component: StatisticsTransactionPage,
            beforeEnter: checkLogin,
            props: route => ({
                initAnalysisType: route.query['analysisType'],
                initChartDataType: route.query['chartDataType'],
                initChartType: route.query['chartType'],
                initChartDateType: route.query['chartDateType'],
                initStartTime: route.query['startTime'],
                initEndTime: route.query['endTime'],
                initFilterAccountIds: route.query['filterAccountIds'],
                initFilterCategoryIds: route.query['filterCategoryIds'],
                initTagFilter: route.query['tagFilter'],
                initKeyword: route.query['keyword'],
                initMatchMode: route.query['matchMode'],
                initSortingType: route.query['sortingType'],
                initTrendDateAggregationType: route.query['trendDateAggregationType'],
                initAssetTrendsDateAggregationType: route.query['assetTrendsDateAggregationType']
            })
        },
        {
            path: '/insights/explorer',
            component: InsightsExplorerPage,
            beforeEnter: checkLogin,
            props: route => ({
                initId: route.query['id'],
                initActiveTab: route.query['activeTab'],
                initDateRangeType: route.query['dateRangeType'],
                initStartTime: route.query['startTime'],
                initEndTime: route.query['endTime']
            })
        },
        {
            path: '/account/list',
            component: AccountListPage,
            beforeEnter: checkLogin
        },
        {
            path: '/',
            component: SettingsPageLayout,
            beforeEnter: checkLogin,
            children: [
                {
                    path: '/category/list',
                    component: TransactionCategoryListPage,
                    beforeEnter: checkLogin
                },
                {
                    path: '/tag/list',
                    component: TransactionTagListPage,
                    beforeEnter: checkLogin
                },
                {
                    path: '/template/list',
                    component: TransactionTemplateListPage,
                    beforeEnter: checkLogin,
                    props: {
                        initType: TemplateType.Normal.type
                    }
                },
                {
                    path: '/schedule/list',
                    component: TransactionTemplateListPage,
                    beforeEnter: checkLogin,
                    props: {
                        initType: TemplateType.Schedule.type
                    }
                },
                {
                    path: '/custom_icon/list',
                    component: UserCustomIconListPage,
                    beforeEnter: checkLogin
                },
                {
                    path: '/presets/import/list',
                    component: ImportPresetsListPage,
                    beforeEnter: checkLogin
                },
                {
                    path: '/settings/user/basic',
                    component: UserBasicSettingPage,
                    beforeEnter: checkLogin
                },
                {
                    path: '/settings/user/security',
                    component: UserSecuritySettingPage,
                    beforeEnter: checkLogin
                },
                {
                    path: '/settings/user/two_factor',
                    component: TwoFactorAuthPage,
                    beforeEnter: checkLogin
                },
                {
                    path: '/settings/user/data_management',
                    component: DataManagementPage,
                    beforeEnter: checkLogin
                },
                {
                    path: '/settings/preferences',
                    component: PreferencesSettingsPage,
                    beforeEnter: checkLogin
                },
                {
                    path: '/settings/application_lock',
                    component: ApplicationLockPage,
                    beforeEnter: checkLogin
                },
                {
                    path: '/settings/statistics',
                    component: StatisticsSettingPage,
                    beforeEnter: checkLogin
                },
                {
                    path: '/settings/cloud_sync',
                    component: ApplicationCloudSyncSettingsPage,
                    beforeEnter: checkLogin
                },
                {
                    path: '/settings/browser_cache',
                    component: BrowserCacheSettingPage,
                    beforeEnter: checkLogin
                },
                {
                    path: '/settings/developer_tools',
                    component: DeveloperToolsPage,
                    beforeEnter: checkLogin
                },
                {
                    path: '/exchange_rate',
                    component: ExchangeRatesListPage,
                    beforeEnter: checkLogin
                }
            ]
        },
        {
            path: '/overview/edit',
            component: OverviewLayoutEditorPage,
            beforeEnter: checkLogin
        },
        {
            path: '/login',
            component: LoginPage,
            beforeEnter: checkNotLogin
        },
        {
            path: '/signup',
            component: SignUpPage,
            beforeEnter: checkNotLogin
        },
        {
            path: '/verify_email',
            component: VerifyEmailPage,
            props: route => ({
                email: route.query['email'],
                token: route.query['token'],
                hasValidEmailVerifyToken: route.query['emailSent'] === 'true'
            })
        },
        {
            path: '/forgetpassword',
            component: ForgetPasswordPage,
            beforeEnter: checkNotLogin
        },
        {
            path: '/resetpassword',
            component: ResetPasswordPage,
            props: route => ({
                token: route.query['token']
            })
        },
        {
            path: '/oauth2_callback',
            component: OAuth2CallbackPage,
            props: route => ({
                token: route.query['token'],
                provider: route.query['provider'],
                platform: route.query['platform'],
                userName: route.query['userName'],
                errorCode: route.query['errorCode'],
                errorMessage: route.query['errorMessage']
            })
        },
        {
            path: '/unlock',
            component: UnlockPage,
            beforeEnter: checkLocked
        }
    ],
})

export default router;
