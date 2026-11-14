package api

import (
	"slices"
	"sort"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/duplicatechecker"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/services"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

// AccountsApi represents account api
type AccountsApi struct {
	ApiUsingConfig
	ApiUsingDuplicateChecker
	accounts           *services.AccountService
	users              *services.UserService
	icons              *services.UserCustomIconService
	tokens             *services.TokenService
	creditCardPictures *services.CreditCardPictureService
}

// Initialize an account api singleton instance
var (
	Accounts = &AccountsApi{
		ApiUsingConfig: ApiUsingConfig{
			container: settings.Container,
		},
		ApiUsingDuplicateChecker: ApiUsingDuplicateChecker{
			ApiUsingConfig: ApiUsingConfig{
				container: settings.Container,
			},
			duplicateChecker: duplicatechecker.Container,
		},
		accounts:           services.Accounts,
		users:              services.Users,
		icons:              services.UserCustomIcons,
		tokens:             services.Tokens,
		creditCardPictures: services.CreditCardPictures,
	}
)

// AccountListHandler returns accounts list of current user
func (a *AccountsApi) AccountListHandler(c *core.WebContext) (any, *errs.Error) {
	var accountListReq models.AccountListRequest
	err := c.ShouldBindQuery(&accountListReq)

	if err != nil {
		log.Warnf(c, "[accounts.AccountListHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	accounts, err := a.accounts.GetAllAccountsByUid(c, uid)

	if err != nil {
		log.Errorf(c, "[accounts.AccountListHandler] failed to get all accounts for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	userAllAccountResps := make([]*models.AccountInfoResponse, len(accounts))
	userAllAccountRespMap := make(map[int64]*models.AccountInfoResponse)

	for i := 0; i < len(accounts); i++ {
		userAllAccountResps[i] = accounts[i].ToAccountInfoResponse()
		userAllAccountRespMap[userAllAccountResps[i].Id] = userAllAccountResps[i]
	}

	for i := 0; i < len(userAllAccountResps); i++ {
		userAccountResp := userAllAccountResps[i]

		if accountListReq.VisibleOnly && userAccountResp.Hidden {
			continue
		}

		if userAccountResp.ParentId <= models.LevelOneAccountParentId {
			continue
		}

		parentAccount, parentExists := userAllAccountRespMap[userAccountResp.ParentId]

		if !parentExists || parentAccount == nil {
			continue
		}

		parentAccount.SubAccounts = append(parentAccount.SubAccounts, userAccountResp)
	}

	userFinalAccountResps := make(models.AccountInfoResponseSlice, 0, len(userAllAccountResps))

	for i := 0; i < len(userAllAccountResps); i++ {
		if userAllAccountResps[i].ParentId == models.LevelOneAccountParentId && (!accountListReq.VisibleOnly || !userAllAccountResps[i].Hidden) {
			sort.Sort(userAllAccountResps[i].SubAccounts)
			userFinalAccountResps = append(userFinalAccountResps, userAllAccountResps[i])
		}
	}

	sort.Sort(userFinalAccountResps)

	a.fillCreditCardPictureUrls(c, uid, userFinalAccountResps...)
	return userFinalAccountResps, nil
}

// AccountGetHandler returns one specific account of current user
func (a *AccountsApi) AccountGetHandler(c *core.WebContext) (any, *errs.Error) {
	var accountGetReq models.AccountGetRequest
	err := c.ShouldBindQuery(&accountGetReq)

	if err != nil {
		log.Warnf(c, "[accounts.AccountGetHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	accountAndSubAccounts, err := a.accounts.GetAccountAndSubAccountsByAccountId(c, uid, accountGetReq.Id)

	if err != nil {
		log.Errorf(c, "[accounts.AccountGetHandler] failed to get account \"id:%d\" for user \"uid:%d\", because %s", accountGetReq.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	accountRespMap := make(map[int64]*models.AccountInfoResponse)

	for i := 0; i < len(accountAndSubAccounts); i++ {
		accountResp := accountAndSubAccounts[i].ToAccountInfoResponse()
		accountRespMap[accountResp.Id] = accountResp
	}

	accountResp, exists := accountRespMap[accountGetReq.Id]

	if !exists {
		return nil, errs.ErrAccountNotFound
	}

	for i := 0; i < len(accountAndSubAccounts); i++ {
		if accountAndSubAccounts[i].ParentAccountId == accountResp.Id {
			subAccountResp := accountAndSubAccounts[i].ToAccountInfoResponse()
			accountResp.SubAccounts = append(accountResp.SubAccounts, subAccountResp)
		}
	}

	sort.Sort(accountResp.SubAccounts)

	a.fillCreditCardPictureUrls(c, uid, accountResp)
	return accountResp, nil
}

// AccountCreateHandler saves a new account by request parameters for current user
func (a *AccountsApi) AccountCreateHandler(c *core.WebContext) (any, *errs.Error) {
	var accountCreateReq models.AccountCreateRequest
	err := c.ShouldBindJSON(&accountCreateReq)

	if err != nil {
		log.Warnf(c, "[accounts.AccountCreateHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	clientTimezone, err := c.GetClientTimezone()

	if err != nil {
		log.Warnf(c, "[accounts.AccountCreateHandler] cannot get client timezone, because %s", err.Error())
		return nil, errs.ErrClientTimezoneOffsetInvalid
	}

	mainAccountBalance := int64(0)

	if accountCreateReq.Balance != "" {
		mainAccountBalance, err = utils.StringToInt64(accountCreateReq.Balance)

		if err != nil {
			return nil, errs.ErrIncompleteOrIncorrectSubmission
		}
	}

	subAccountBalances := make([]int64, len(accountCreateReq.SubAccounts))

	if accountCreateReq.Category < models.ACCOUNT_CATEGORY_CASH || accountCreateReq.Category > models.ACCOUNT_CATEGORY_CERTIFICATE_OF_DEPOSIT {
		log.Warnf(c, "[accounts.AccountCreateHandler] account category invalid, category is %d", accountCreateReq.Category)
		return nil, errs.ErrAccountCategoryInvalid
	}

	if accountCreateReq.Category != models.ACCOUNT_CATEGORY_CREDIT_CARD && accountCreateReq.CreditCardStatementDate != 0 {
		log.Warnf(c, "[accounts.AccountCreateHandler] cannot set statement date with category \"%d\"", accountCreateReq.Category)
		return nil, errs.ErrCannotSetStatementDateForNonCreditCard
	}

	if accountCreateReq.Category != models.ACCOUNT_CATEGORY_CREDIT_CARD && accountCreateReq.CreditCardLimit != "" {
		log.Warnf(c, "[accounts.AccountCreateHandler] cannot set credit limit with category \"%d\"", accountCreateReq.Category)
		return nil, errs.ErrCannotSetCreditLimitForNonCreditCardAccount
	}

	if accountCreateReq.Category != models.ACCOUNT_CATEGORY_CREDIT_CARD && hasCreditCardProfileFields(accountCreateReq.CreditCardBankName, accountCreateReq.CreditCardCardName, accountCreateReq.CreditCardLastFourDigits, accountCreateReq.CreditCardExpireMonth, accountCreateReq.CreditCardExpireYear, accountCreateReq.CreditCardCardholderName, accountCreateReq.CreditCardPictureId, accountCreateReq.CreditCardStatementDateIncludeTransaction, accountCreateReq.CreditCardRepaymentDateType, accountCreateReq.CreditCardRepaymentDaysAfterStatement, accountCreateReq.CreditCardRepaymentDayOfMonth) {
		log.Warnf(c, "[accounts.AccountCreateHandler] cannot set credit card profile with category \"%d\"", accountCreateReq.Category)
		return nil, errs.ErrCannotSetCreditCardProfileForNonCreditCard
	}

	if accountCreateReq.Category != models.ACCOUNT_CATEGORY_CREDIT_CARD && hasCreditCardEntityFields(accountCreateReq.CreditCardIsEntityCard, accountCreateReq.CreditCardCurrencyLimit) {
		log.Warnf(c, "[accounts.AccountCreateHandler] cannot set credit card entity fields with category \"%d\"", accountCreateReq.Category)
		return nil, errs.ErrCannotSetCreditCardProfileForNonCreditCard
	}

	mainAccountNotificationParams := getCreditCardNotificationParamsFromCreateRequest(&accountCreateReq)

	if accountCreateReq.Category != models.ACCOUNT_CATEGORY_CREDIT_CARD && hasCreditCardNotificationFields(mainAccountNotificationParams) {
		log.Warnf(c, "[accounts.AccountCreateHandler] cannot set credit card notification with category \"%d\"", accountCreateReq.Category)
		return nil, errs.ErrCannotSetCreditCardNotificationForNonCreditCard
	}

	if !isCreditCardNotificationParamsValid(mainAccountNotificationParams) {
		log.Warnf(c, "[accounts.AccountCreateHandler] credit card notification timezone is invalid with category \"%d\"", accountCreateReq.Category)
		return nil, errs.ErrCreditCardNotificationTimezoneInvalid
	}

	creditLimitForCreditCard := int64(0)

	if accountCreateReq.CreditCardLimit != "" {
		creditLimitForCreditCard, err = utils.StringToInt64(accountCreateReq.CreditCardLimit)

		if err != nil || creditLimitForCreditCard <= 0 {
			return nil, errs.ErrIncompleteOrIncorrectSubmission
		}
	}

	if accountCreateReq.Type == models.ACCOUNT_TYPE_SINGLE_ACCOUNT {
		if len(accountCreateReq.SubAccounts) > 0 {
			log.Warnf(c, "[accounts.AccountCreateHandler] account cannot have any sub-accounts")
			return nil, errs.ErrAccountCannotHaveSubAccounts
		}

		if accountCreateReq.Currency == core.AccountCurrencyNotSetValue {
			log.Warnf(c, "[accounts.AccountCreateHandler] account must set currency")
			return nil, errs.ErrAccountCurrencyInvalid
		}

		if mainAccountBalance != 0 && accountCreateReq.BalanceTime <= 0 {
			log.Warnf(c, "[accounts.AccountCreateHandler] account balance time is not set")
			return nil, errs.ErrAccountBalanceTimeNotSet
		}
	} else if accountCreateReq.Type == models.ACCOUNT_TYPE_MULTI_SUB_ACCOUNTS {
		if len(accountCreateReq.SubAccounts) < 1 {
			log.Warnf(c, "[accounts.AccountCreateHandler] account does not have any sub-accounts")
			return nil, errs.ErrAccountHaveNoSubAccount
		}

		if accountCreateReq.Category == models.ACCOUNT_CATEGORY_CREDIT_CARD {
			if accountCreateReq.Currency == core.AccountCurrencyNotSetValue && creditLimitForCreditCard > 0 {
				log.Warnf(c, "[accounts.AccountCreateHandler] parent account must set currency when set credit limit")
				return nil, errs.ErrMustSetParentAccountCurrencyWhenSetCreditLimit
			}
		} else {
			if accountCreateReq.Currency != core.AccountCurrencyNotSetValue {
				log.Warnf(c, "[accounts.AccountCreateHandler] parent account cannot set currency")
				return nil, errs.ErrParentAccountCannotSetCurrency
			}
		}

		if mainAccountBalance != 0 {
			log.Warnf(c, "[accounts.AccountCreateHandler] parent account cannot set balance")
			return nil, errs.ErrParentAccountCannotSetBalance
		}

		for i := 0; i < len(accountCreateReq.SubAccounts); i++ {
			subAccount := accountCreateReq.SubAccounts[i]
			subAccountBalance := int64(0)

			if subAccount.Balance != "" {
				subAccountBalance, err = utils.StringToInt64(subAccount.Balance)

				if err != nil {
					return nil, errs.ErrIncompleteOrIncorrectSubmission
				}
			}

			subAccountBalances[i] = subAccountBalance

			if subAccount.Category != accountCreateReq.Category {
				log.Warnf(c, "[accounts.AccountCreateHandler] category of sub-account#%d not equals to parent", i)
				return nil, errs.ErrSubAccountCategoryNotEqualsToParent
			}

			if subAccount.Type != models.ACCOUNT_TYPE_SINGLE_ACCOUNT {
				log.Warnf(c, "[accounts.AccountCreateHandler] sub-account#%d type invalid", i)
				return nil, errs.ErrSubAccountTypeInvalid
			}

			if subAccount.Currency == core.AccountCurrencyNotSetValue {
				log.Warnf(c, "[accounts.AccountCreateHandler] sub-account#%d must set currency", i)
				return nil, errs.ErrAccountCurrencyInvalid
			}

			if subAccountBalance != 0 && subAccount.BalanceTime <= 0 {
				log.Warnf(c, "[accounts.AccountCreateHandler] sub-account#%d balance time is not set", i)
				return nil, errs.ErrAccountBalanceTimeNotSet
			}

			// A credit card sub-account represents its own physical card, so it keeps its own
			// statement date, card profile and notification settings. The credit limit still
			// belongs to the shared pool on the level-one account only.
			if subAccount.CreditCardLimit != "" {
				log.Warnf(c, "[accounts.AccountCreateHandler] sub-account#%d cannot set credit limit", i)
				return nil, errs.ErrCannotSetCreditLimitForSubAccount
			}

			if !isCreditCardNotificationParamsValid(getCreditCardNotificationParamsFromCreateRequest(subAccount)) {
				log.Warnf(c, "[accounts.AccountCreateHandler] sub-account#%d credit card notification timezone is invalid", i)
				return nil, errs.ErrCreditCardNotificationTimezoneInvalid
			}
		}
	} else {
		log.Warnf(c, "[accounts.AccountCreateHandler] account type invalid, type is %d", accountCreateReq.Type)
		return nil, errs.ErrAccountTypeInvalid
	}

	uid := c.GetCurrentUid()
	maxOrderId, err := a.accounts.GetMaxDisplayOrder(c, uid, accountCreateReq.Category)

	if err != nil {
		log.Errorf(c, "[accounts.AccountCreateHandler] failed to get max display order for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	mainAccount := a.createNewAccountModel(uid, &accountCreateReq, mainAccountBalance, &creditLimitForCreditCard, false, maxOrderId+1)
	childrenAccounts, childrenAccountBalanceTimes := a.createSubAccountModels(uid, &accountCreateReq, subAccountBalances)
	iconTypeValid := a.isAccountsIconTypeValid(c, uid, slices.Concat([]*models.Account{mainAccount}, childrenAccounts))

	if !iconTypeValid {
		log.Warnf(c, "[accounts.AccountCreateHandler] icon type invalid for user \"uid:%d\"", uid)
		return nil, errs.ErrAccountIconInvalid
	}

	if accountCreateReq.CreditCardPictureId > 0 {
		err = a.creditCardPictures.CheckPictureCanBeBound(c, uid, 0, accountCreateReq.CreditCardPictureId)

		if err != nil {
			log.Warnf(c, "[accounts.AccountCreateHandler] credit card picture \"id:%d\" cannot be used for user \"uid:%d\", because %s", accountCreateReq.CreditCardPictureId, uid, err.Error())
			return nil, errs.Or(err, errs.ErrCreditCardPictureIdInvalid)
		}
	}

	for i := 0; i < len(accountCreateReq.SubAccounts); i++ {
		pictureId := accountCreateReq.SubAccounts[i].CreditCardPictureId

		if pictureId > 0 {
			err = a.creditCardPictures.CheckPictureCanBeBound(c, uid, 0, pictureId)

			if err != nil {
				log.Warnf(c, "[accounts.AccountCreateHandler] sub-account#%d credit card picture \"id:%d\" cannot be used for user \"uid:%d\", because %s", i, pictureId, uid, err.Error())
				return nil, errs.Or(err, errs.ErrCreditCardPictureIdInvalid)
			}
		}
	}

	if a.CurrentConfig().EnableDuplicateSubmissionsCheck && accountCreateReq.ClientSessionId != "" {
		found, remark := a.GetSubmissionRemark(duplicatechecker.DUPLICATE_CHECKER_TYPE_NEW_ACCOUNT, uid, accountCreateReq.ClientSessionId)

		if found {
			log.Infof(c, "[accounts.AccountCreateHandler] another account \"id:%s\" has been created for user \"uid:%d\"", remark, uid)
			accountId, err := utils.StringToInt64(remark)

			if err == nil {
				accountAndSubAccounts, err := a.accounts.GetAccountAndSubAccountsByAccountId(c, uid, accountId)

				if err != nil {
					log.Errorf(c, "[accounts.AccountCreateHandler] failed to get existed account \"id:%d\" for user \"uid:%d\", because %s", accountId, uid, err.Error())
					return nil, errs.Or(err, errs.ErrOperationFailed)
				}

				accountMap := a.accounts.GetAccountMapByList(accountAndSubAccounts)
				mainAccount, exists := accountMap[accountId]

				if !exists {
					return nil, errs.ErrOperationFailed
				}

				accountInfoResp := mainAccount.ToAccountInfoResponse()

				for i := 0; i < len(accountAndSubAccounts); i++ {
					if accountAndSubAccounts[i].ParentAccountId == mainAccount.AccountId {
						subAccountResp := accountAndSubAccounts[i].ToAccountInfoResponse()
						accountInfoResp.SubAccounts = append(accountInfoResp.SubAccounts, subAccountResp)
					}
				}

				a.fillCreditCardPictureUrls(c, uid, accountInfoResp)
				return accountInfoResp, nil
			}
		}
	}

	err = a.accounts.CreateAccounts(c, mainAccount, accountCreateReq.BalanceTime, childrenAccounts, childrenAccountBalanceTimes, clientTimezone)

	if err != nil {
		log.Errorf(c, "[accounts.AccountCreateHandler] failed to create account \"id:%d\" for user \"uid:%d\", because %s", mainAccount.AccountId, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	if accountCreateReq.CreditCardPictureId > 0 {
		err = a.creditCardPictures.BindPictureToAccount(c, uid, mainAccount.AccountId, accountCreateReq.CreditCardPictureId)

		if err != nil {
			log.Errorf(c, "[accounts.AccountCreateHandler] failed to bind credit card picture \"id:%d\" to account \"id:%d\" for user \"uid:%d\", because %s", accountCreateReq.CreditCardPictureId, mainAccount.AccountId, uid, err.Error())
			return nil, errs.Or(err, errs.ErrOperationFailed)
		}
	}

	for i := 0; i < len(childrenAccounts); i++ {
		pictureId := getCreditCardPictureId(childrenAccounts[i].Extend)

		if pictureId > 0 {
			err = a.creditCardPictures.BindPictureToAccount(c, uid, childrenAccounts[i].AccountId, pictureId)

			if err != nil {
				log.Errorf(c, "[accounts.AccountCreateHandler] failed to bind credit card picture \"id:%d\" to sub-account \"id:%d\" for user \"uid:%d\", because %s", pictureId, childrenAccounts[i].AccountId, uid, err.Error())
				return nil, errs.Or(err, errs.ErrOperationFailed)
			}
		}
	}

	log.Infof(c, "[accounts.AccountCreateHandler] user \"uid:%d\" has created a new account \"id:%d\" successfully", uid, mainAccount.AccountId)

	a.resetCreditCardNotificationScan(c, uid, mainAccount.Category)

	a.SetSubmissionRemarkIfEnable(duplicatechecker.DUPLICATE_CHECKER_TYPE_NEW_ACCOUNT, uid, accountCreateReq.ClientSessionId, utils.Int64ToString(mainAccount.AccountId))
	accountInfoResp := mainAccount.ToAccountInfoResponse()

	if len(childrenAccounts) > 0 {
		accountInfoResp.SubAccounts = make([]*models.AccountInfoResponse, len(childrenAccounts))

		for i := 0; i < len(childrenAccounts); i++ {
			accountInfoResp.SubAccounts[i] = childrenAccounts[i].ToAccountInfoResponse()
		}
	}

	a.fillCreditCardPictureUrls(c, uid, accountInfoResp)
	return accountInfoResp, nil
}

// AccountModifyHandler saves an existed account by request parameters for current user
func (a *AccountsApi) AccountModifyHandler(c *core.WebContext) (any, *errs.Error) {
	var accountModifyReq models.AccountModifyRequest
	err := c.ShouldBindJSON(&accountModifyReq)

	if err != nil {
		log.Warnf(c, "[accounts.AccountModifyHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	if accountModifyReq.Id <= 0 {
		return nil, errs.ErrAccountIdInvalid
	}

	clientTimezone, err := c.GetClientTimezone()

	if err != nil {
		log.Warnf(c, "[accounts.AccountModifyHandler] cannot get client timezone, because %s", err.Error())
		return nil, errs.ErrClientTimezoneOffsetInvalid
	}

	if accountModifyReq.Category < models.ACCOUNT_CATEGORY_CASH || accountModifyReq.Category > models.ACCOUNT_CATEGORY_CERTIFICATE_OF_DEPOSIT {
		log.Warnf(c, "[accounts.AccountModifyHandler] account category invalid, category is %d", accountModifyReq.Category)
		return nil, errs.ErrAccountCategoryInvalid
	}

	if accountModifyReq.Category != models.ACCOUNT_CATEGORY_CREDIT_CARD && accountModifyReq.CreditCardStatementDate != 0 {
		log.Warnf(c, "[accounts.AccountModifyHandler] cannot set statement date with category \"%d\"", accountModifyReq.Category)
		return nil, errs.ErrCannotSetStatementDateForNonCreditCard
	}

	if accountModifyReq.Category != models.ACCOUNT_CATEGORY_CREDIT_CARD && accountModifyReq.CreditCardLimit != "" {
		log.Warnf(c, "[accounts.AccountModifyHandler] cannot set credit limit with category \"%d\"", accountModifyReq.Category)
		return nil, errs.ErrCannotSetCreditLimitForNonCreditCardAccount
	}

	if accountModifyReq.Category != models.ACCOUNT_CATEGORY_CREDIT_CARD && hasCreditCardProfileFields(accountModifyReq.CreditCardBankName, accountModifyReq.CreditCardCardName, accountModifyReq.CreditCardLastFourDigits, accountModifyReq.CreditCardExpireMonth, accountModifyReq.CreditCardExpireYear, accountModifyReq.CreditCardCardholderName, accountModifyReq.CreditCardPictureId, accountModifyReq.CreditCardStatementDateIncludeTransaction, accountModifyReq.CreditCardRepaymentDateType, accountModifyReq.CreditCardRepaymentDaysAfterStatement, accountModifyReq.CreditCardRepaymentDayOfMonth) {
		log.Warnf(c, "[accounts.AccountModifyHandler] cannot set credit card profile with category \"%d\"", accountModifyReq.Category)
		return nil, errs.ErrCannotSetCreditCardProfileForNonCreditCard
	}

	if accountModifyReq.Category != models.ACCOUNT_CATEGORY_CREDIT_CARD && hasCreditCardEntityFields(accountModifyReq.CreditCardIsEntityCard, accountModifyReq.CreditCardCurrencyLimit) {
		log.Warnf(c, "[accounts.AccountModifyHandler] cannot set credit card entity fields with category \"%d\"", accountModifyReq.Category)
		return nil, errs.ErrCannotSetCreditCardProfileForNonCreditCard
	}

	mainAccountNotificationParams := getCreditCardNotificationParamsFromModifyRequest(&accountModifyReq)

	if accountModifyReq.Category != models.ACCOUNT_CATEGORY_CREDIT_CARD && hasCreditCardNotificationFields(mainAccountNotificationParams) {
		log.Warnf(c, "[accounts.AccountModifyHandler] cannot set credit card notification with category \"%d\"", accountModifyReq.Category)
		return nil, errs.ErrCannotSetCreditCardNotificationForNonCreditCard
	}

	if !isCreditCardNotificationParamsValid(mainAccountNotificationParams) {
		log.Warnf(c, "[accounts.AccountModifyHandler] credit card notification timezone is invalid with category \"%d\"", accountModifyReq.Category)
		return nil, errs.ErrCreditCardNotificationTimezoneInvalid
	}

	creditLimitForCreditCard := int64(0)

	if accountModifyReq.CreditCardLimit != "" {
		creditLimitForCreditCard, err = utils.StringToInt64(accountModifyReq.CreditCardLimit)

		if err != nil || creditLimitForCreditCard <= 0 {
			return nil, errs.ErrIncompleteOrIncorrectSubmission
		}
	}

	uid := c.GetCurrentUid()
	user, err := a.users.GetUserById(c, uid)

	if err != nil {
		if !errs.IsCustomError(err) {
			log.Errorf(c, "[accounts.AccountModifyHandler] failed to get user, because %s", err.Error())
		}

		return nil, errs.ErrUserNotFound
	}

	accountAndSubAccounts, err := a.accounts.GetAccountAndSubAccountsByAccountId(c, uid, accountModifyReq.Id)

	if err != nil {
		log.Errorf(c, "[accounts.AccountModifyHandler] failed to get account \"id:%d\" for user \"uid:%d\", because %s", accountModifyReq.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	accountMap := a.accounts.GetAccountMapByList(accountAndSubAccounts)
	mainAccount, exists := accountMap[accountModifyReq.Id]
	subAccountBalances := make([]int64, len(accountModifyReq.SubAccounts))

	if !exists {
		return nil, errs.ErrAccountNotFound
	}

	if accountModifyReq.Balance != nil {
		return nil, errs.ErrNotSupportedChangeBalance
	}

	if accountModifyReq.BalanceTime != nil {
		return nil, errs.ErrNotSupportedChangeBalanceTime
	}

	updateMainAccountCurrency := false

	if mainAccount.Type == models.ACCOUNT_TYPE_SINGLE_ACCOUNT {
		if len(accountModifyReq.SubAccounts) > 0 {
			log.Warnf(c, "[accounts.AccountModifyHandler] account cannot have any sub-accounts")
			return nil, errs.ErrAccountCannotHaveSubAccounts
		}

		if accountModifyReq.Currency != nil && mainAccount.Currency != *accountModifyReq.Currency {
			return nil, errs.ErrNotSupportedChangeCurrency
		}
	} else if mainAccount.Type == models.ACCOUNT_TYPE_MULTI_SUB_ACCOUNTS {
		if len(accountModifyReq.SubAccounts) < 1 {
			log.Warnf(c, "[accounts.AccountModifyHandler] account does not have any sub-accounts")
			return nil, errs.ErrAccountHaveNoSubAccount
		}

		if accountModifyReq.Category == models.ACCOUNT_CATEGORY_CREDIT_CARD {
			if ((accountModifyReq.Currency == nil && mainAccount.Currency == core.AccountCurrencyNotSetValue) || (accountModifyReq.Currency != nil && *accountModifyReq.Currency == core.AccountCurrencyNotSetValue)) && creditLimitForCreditCard > 0 {
				log.Warnf(c, "[accounts.AccountModifyHandler] parent account must set currency when set credit limit")
				return nil, errs.ErrMustSetParentAccountCurrencyWhenSetCreditLimit
			} else if accountModifyReq.Currency != nil && mainAccount.Currency != *accountModifyReq.Currency {
				log.Infof(c, "[accounts.AccountModifyHandler] will change parent account curreny from %s to %s", mainAccount.Currency, *accountModifyReq.Currency)
				updateMainAccountCurrency = true
			}
		} else {
			if accountModifyReq.Currency != nil && mainAccount.Currency != *accountModifyReq.Currency {
				return nil, errs.ErrNotSupportedChangeCurrency
			}
		}

		for i := 0; i < len(accountModifyReq.SubAccounts); i++ {
			subAccountReq := accountModifyReq.SubAccounts[i]
			subAccountBalances[i] = 0

			if subAccountReq.Category != accountModifyReq.Category {
				log.Warnf(c, "[accounts.AccountModifyHandler] category of sub-account#%d not equals to parent", i)
				return nil, errs.ErrSubAccountCategoryNotEqualsToParent
			}

			if subAccountReq.Id == 0 { // create new sub-account
				if subAccountReq.Currency == nil {
					log.Warnf(c, "[accounts.AccountModifyHandler] sub-account#%d not set currency", i)
					return nil, errs.ErrAccountCurrencyInvalid
				} else if subAccountReq.Currency != nil && *subAccountReq.Currency == core.AccountCurrencyNotSetValue {
					log.Warnf(c, "[accounts.AccountModifyHandler] sub-account#%d must set currency", i)
					return nil, errs.ErrAccountCurrencyInvalid
				}

				subAccountBalance := int64(0)

				if subAccountReq.Balance != nil && *subAccountReq.Balance != "" {
					subAccountBalance, err = utils.StringToInt64(*subAccountReq.Balance)

					if err != nil {
						return nil, errs.ErrIncompleteOrIncorrectSubmission
					}
				}

				subAccountBalances[i] = subAccountBalance

				if subAccountBalance == 0 {
					defaultBalanceTime := int64(0)
					subAccountReq.BalanceTime = &defaultBalanceTime
				}

				if subAccountBalance != 0 && (subAccountReq.BalanceTime == nil || *subAccountReq.BalanceTime <= 0) {
					log.Warnf(c, "[accounts.AccountModifyHandler] sub-account#%d balance time is not set", i)
					return nil, errs.ErrAccountBalanceTimeNotSet
				}
			} else { // modify existed sub-account
				subAccount, exists := accountMap[subAccountReq.Id]

				if !exists {
					return nil, errs.ErrAccountNotFound
				}

				if subAccountReq.Currency != nil && subAccount.Currency != *subAccountReq.Currency {
					return nil, errs.ErrNotSupportedChangeCurrency
				}

				if subAccountReq.Balance != nil {
					return nil, errs.ErrNotSupportedChangeBalance
				}

				if subAccountReq.BalanceTime != nil {
					return nil, errs.ErrNotSupportedChangeBalanceTime
				}
			}

			// A credit card sub-account represents its own physical card, so it keeps its own
			// statement date, card profile and notification settings. The credit limit still
			// belongs to the shared pool on the level-one account only.
			if subAccountReq.CreditCardLimit != "" {
				log.Warnf(c, "[accounts.AccountModifyHandler] sub-account#%d cannot set credit limit", i)
				return nil, errs.ErrCannotSetCreditLimitForSubAccount
			}

			if !isCreditCardNotificationParamsValid(getCreditCardNotificationParamsFromModifyRequest(subAccountReq)) {
				log.Warnf(c, "[accounts.AccountModifyHandler] sub-account#%d credit card notification timezone is invalid", i)
				return nil, errs.ErrCreditCardNotificationTimezoneInvalid
			}
		}
	}

	anythingUpdate := false
	var toUpdateAccounts []*models.Account
	var toAddAccounts []*models.Account
	var toAddAccountBalanceTimes []int64
	var toDeleteAccountIds []int64

	if accountModifyReq.CreditCardPictureId > 0 {
		err = a.creditCardPictures.CheckPictureCanBeBound(c, uid, mainAccount.AccountId, accountModifyReq.CreditCardPictureId)

		if err != nil {
			log.Warnf(c, "[accounts.AccountModifyHandler] credit card picture \"id:%d\" cannot be used for user \"uid:%d\", because %s", accountModifyReq.CreditCardPictureId, uid, err.Error())
			return nil, errs.Or(err, errs.ErrCreditCardPictureIdInvalid)
		}
	}

	for i := 0; i < len(accountModifyReq.SubAccounts); i++ {
		subAccountReq := accountModifyReq.SubAccounts[i]

		if subAccountReq.CreditCardPictureId > 0 {
			err = a.creditCardPictures.CheckPictureCanBeBound(c, uid, subAccountReq.Id, subAccountReq.CreditCardPictureId)

			if err != nil {
				log.Warnf(c, "[accounts.AccountModifyHandler] sub-account#%d credit card picture \"id:%d\" cannot be used for user \"uid:%d\", because %s", i, subAccountReq.CreditCardPictureId, uid, err.Error())
				return nil, errs.Or(err, errs.ErrCreditCardPictureIdInvalid)
			}
		}
	}

	toUpdateAccount, err := a.getToUpdateAccount(user, &accountModifyReq, &creditLimitForCreditCard, mainAccount, false)

	if err != nil {
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	if toUpdateAccount != nil {
		if toUpdateAccount.Category != mainAccount.Category {
			maxOrderId, err := a.accounts.GetMaxDisplayOrder(c, uid, toUpdateAccount.Category)

			if err != nil {
				log.Errorf(c, "[accounts.AccountModifyHandler] failed to get max display order for user \"uid:%d\", because %s", uid, err.Error())
				return nil, errs.Or(err, errs.ErrOperationFailed)
			}

			toUpdateAccount.DisplayOrder = maxOrderId + 1
		}

		anythingUpdate = true
		toUpdateAccounts = append(toUpdateAccounts, toUpdateAccount)
	}

	toDeleteAccountIds = a.getToDeleteSubAccountIds(&accountModifyReq, mainAccount, accountAndSubAccounts)

	if len(toDeleteAccountIds) > 0 {
		anythingUpdate = true
	}

	maxOrderId := int32(0)

	for i := 0; i < len(accountAndSubAccounts); i++ {
		account := accountAndSubAccounts[i]

		if account.AccountId != mainAccount.AccountId && account.DisplayOrder > maxOrderId {
			maxOrderId = account.DisplayOrder
		}
	}

	for i := 0; i < len(accountModifyReq.SubAccounts); i++ {
		subAccountReq := accountModifyReq.SubAccounts[i]

		if _, exists := accountMap[subAccountReq.Id]; !exists {
			anythingUpdate = true
			maxOrderId = maxOrderId + 1
			newSubAccount := a.createNewSubAccountModelForModify(uid, mainAccount.Type, subAccountReq, subAccountBalances[i], maxOrderId)
			toAddAccounts = append(toAddAccounts, newSubAccount)

			if subAccountReq.BalanceTime != nil {
				toAddAccountBalanceTimes = append(toAddAccountBalanceTimes, *subAccountReq.BalanceTime)
			} else {
				toAddAccountBalanceTimes = append(toAddAccountBalanceTimes, 0)
			}
		} else {
			toUpdateSubAccount, err := a.getToUpdateAccount(user, subAccountReq, nil, accountMap[subAccountReq.Id], true)

			if err != nil {
				return nil, errs.Or(err, errs.ErrOperationFailed)
			}

			if toUpdateSubAccount != nil {
				anythingUpdate = true
				toUpdateAccounts = append(toUpdateAccounts, toUpdateSubAccount)
			}
		}
	}

	if !anythingUpdate {
		return nil, errs.ErrNothingWillBeUpdated
	}

	iconTypeValid := a.isAccountsIconTypeValid(c, uid, slices.Concat(toAddAccounts, toUpdateAccounts))

	if !iconTypeValid {
		log.Warnf(c, "[accounts.AccountModifyHandler] icon type invalid for user \"uid:%d\"", uid)
		return nil, errs.ErrAccountIconInvalid
	}

	if len(toAddAccounts) > 0 && a.CurrentConfig().EnableDuplicateSubmissionsCheck && accountModifyReq.ClientSessionId != "" {
		found, remark := a.GetSubmissionRemark(duplicatechecker.DUPLICATE_CHECKER_TYPE_NEW_SUBACCOUNT, uid, accountModifyReq.ClientSessionId)

		if found {
			log.Infof(c, "[accounts.AccountModifyHandler] another account \"id:%s\" modification has been created for user \"uid:%d\"", remark, uid)
			accountId, err := utils.StringToInt64(remark)

			if err == nil {
				accountAndSubAccounts, err := a.accounts.GetAccountAndSubAccountsByAccountId(c, uid, accountId)

				if err != nil {
					log.Errorf(c, "[accounts.AccountModifyHandler] failed to get existed account \"id:%d\" for user \"uid:%d\", because %s", accountId, uid, err.Error())
					return nil, errs.Or(err, errs.ErrOperationFailed)
				}

				accountMap := a.accounts.GetAccountMapByList(accountAndSubAccounts)
				mainAccount, exists := accountMap[accountId]

				if !exists {
					return nil, errs.ErrOperationFailed
				}

				accountInfoResp := mainAccount.ToAccountInfoResponse()

				for i := 0; i < len(accountAndSubAccounts); i++ {
					if accountAndSubAccounts[i].ParentAccountId == mainAccount.AccountId {
						subAccountResp := accountAndSubAccounts[i].ToAccountInfoResponse()
						accountInfoResp.SubAccounts = append(accountInfoResp.SubAccounts, subAccountResp)
					}
				}

				a.fillCreditCardPictureUrls(c, uid, accountInfoResp)
				return accountInfoResp, nil
			}
		}
	}

	err = a.accounts.ModifyAccounts(c, mainAccount, toUpdateAccounts, toAddAccounts, toAddAccountBalanceTimes, toDeleteAccountIds, updateMainAccountCurrency, clientTimezone)

	if err != nil {
		log.Errorf(c, "[accounts.AccountModifyHandler] failed to update account \"id:%d\" for user \"uid:%d\", because %s", accountModifyReq.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	if accountModifyReq.CreditCardPictureId != getCreditCardPictureId(mainAccount.Extend) {
		err = a.creditCardPictures.BindPictureToAccount(c, uid, accountModifyReq.Id, accountModifyReq.CreditCardPictureId)

		if err != nil {
			log.Errorf(c, "[accounts.AccountModifyHandler] failed to bind credit card picture \"id:%d\" to account \"id:%d\" for user \"uid:%d\", because %s", accountModifyReq.CreditCardPictureId, accountModifyReq.Id, uid, err.Error())
			return nil, errs.Or(err, errs.ErrOperationFailed)
		}
	}

	for i := 0; i < len(toUpdateAccounts); i++ {
		subAccount := toUpdateAccounts[i]

		if subAccount.AccountId == mainAccount.AccountId || subAccount.Category != models.ACCOUNT_CATEGORY_CREDIT_CARD {
			continue
		}

		oldSubAccount := accountMap[subAccount.AccountId]

		if oldSubAccount == nil {
			continue
		}

		pictureId := getCreditCardPictureId(subAccount.Extend)

		if pictureId != getCreditCardPictureId(oldSubAccount.Extend) {
			err = a.creditCardPictures.BindPictureToAccount(c, uid, subAccount.AccountId, pictureId)

			if err != nil {
				log.Errorf(c, "[accounts.AccountModifyHandler] failed to bind credit card picture \"id:%d\" to sub-account \"id:%d\" for user \"uid:%d\", because %s", pictureId, subAccount.AccountId, uid, err.Error())
				return nil, errs.Or(err, errs.ErrOperationFailed)
			}
		}
	}

	for i := 0; i < len(toAddAccounts); i++ {
		subAccount := toAddAccounts[i]
		pictureId := getCreditCardPictureId(subAccount.Extend)

		if pictureId > 0 {
			err = a.creditCardPictures.BindPictureToAccount(c, uid, subAccount.AccountId, pictureId)

			if err != nil {
				log.Errorf(c, "[accounts.AccountModifyHandler] failed to bind credit card picture \"id:%d\" to sub-account \"id:%d\" for user \"uid:%d\", because %s", pictureId, subAccount.AccountId, uid, err.Error())
				return nil, errs.Or(err, errs.ErrOperationFailed)
			}
		}
	}

	log.Infof(c, "[accounts.AccountModifyHandler] user \"uid:%d\" has updated account \"id:%d\" successfully", uid, accountModifyReq.Id)

	a.resetCreditCardNotificationScan(c, uid, accountModifyReq.Category)

	if len(toAddAccounts) > 0 {
		a.SetSubmissionRemarkIfEnable(duplicatechecker.DUPLICATE_CHECKER_TYPE_NEW_SUBACCOUNT, uid, accountModifyReq.ClientSessionId, utils.Int64ToString(mainAccount.AccountId))
	}

	accountRespMap := make(map[int64]*models.AccountInfoResponse)

	for i := 0; i < len(toUpdateAccounts); i++ {
		account := toUpdateAccounts[i]
		oldAccount := accountMap[account.AccountId]

		account.Type = oldAccount.Type
		account.ParentAccountId = oldAccount.ParentAccountId
		account.Balance = oldAccount.Balance

		accountResp := account.ToAccountInfoResponse()
		accountRespMap[accountResp.Id] = accountResp
	}

	for i := 0; i < len(toAddAccounts); i++ {
		account := toAddAccounts[i]
		accountResp := account.ToAccountInfoResponse()
		accountRespMap[accountResp.Id] = accountResp
	}

	deletedAccountIds := make(map[int64]bool)

	for i := 0; i < len(toDeleteAccountIds); i++ {
		deletedAccountIds[toDeleteAccountIds[i]] = true
	}

	for i := 0; i < len(accountAndSubAccounts); i++ {
		oldAccount := accountAndSubAccounts[i]
		_, exists := accountRespMap[oldAccount.AccountId]

		if !exists && !deletedAccountIds[oldAccount.AccountId] {
			oldAccountResp := oldAccount.ToAccountInfoResponse()
			accountRespMap[oldAccountResp.Id] = oldAccountResp
		}
	}

	accountResp := accountRespMap[accountModifyReq.Id]

	for i := 0; i < len(accountAndSubAccounts); i++ {
		account := accountAndSubAccounts[i]

		if account.ParentAccountId == accountResp.Id && !deletedAccountIds[account.AccountId] {
			subAccountResp := accountRespMap[account.AccountId]
			accountResp.SubAccounts = append(accountResp.SubAccounts, subAccountResp)
		}
	}

	for i := 0; i < len(toAddAccounts); i++ {
		account := toAddAccounts[i]

		if account.ParentAccountId == accountResp.Id {
			subAccountResp := accountRespMap[account.AccountId]
			accountResp.SubAccounts = append(accountResp.SubAccounts, subAccountResp)
		}
	}

	sort.Sort(accountResp.SubAccounts)

	a.fillCreditCardPictureUrls(c, uid, accountResp)
	return accountResp, nil
}

// AccountUpdateLastReconciledTimeHandler updates last reconciled time of an existed account by request parameters for current user
func (a *AccountsApi) AccountUpdateLastReconciledTimeHandler(c *core.WebContext) (any, *errs.Error) {
	var accountUpdateReq models.AccountUpdateLastReconciledTimeRequest
	err := c.ShouldBindJSON(&accountUpdateReq)

	if err != nil {
		log.Warnf(c, "[accounts.AccountUpdateLastReconciledTimeHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	if accountUpdateReq.Id <= 0 {
		return nil, errs.ErrAccountIdInvalid
	}

	uid := c.GetCurrentUid()
	user, err := a.users.GetUserById(c, uid)

	if err != nil {
		if !errs.IsCustomError(err) {
			log.Errorf(c, "[accounts.AccountUpdateLastReconciledTimeHandler] failed to get user, because %s", err.Error())
		}

		return nil, errs.ErrUserNotFound
	}

	if !user.UseLastReconciledTime {
		return nil, errs.ErrLastReconciledTimeIsNotEnabled
	}

	account, err := a.accounts.GetAccountByAccountId(c, uid, accountUpdateReq.Id)

	if err != nil {
		log.Errorf(c, "[accounts.AccountUpdateLastReconciledTimeHandler] failed to get account \"id:%d\" for user \"uid:%d\", because %s", accountUpdateReq.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	if account.Type == models.ACCOUNT_TYPE_MULTI_SUB_ACCOUNTS {
		return nil, errs.ErrParentAccountCannotSetLastReconciledTime
	}

	if account.Extend == nil {
		account.Extend = &models.AccountExtend{}
	}

	if account.Extend.LastReconciledTime != nil && accountUpdateReq.LastReconciledTime < *account.Extend.LastReconciledTime {
		return nil, errs.ErrCannotSetLastReconciledTimeBeforeCurrent
	} else if account.Extend.LastReconciledTime != nil && accountUpdateReq.LastReconciledTime == *account.Extend.LastReconciledTime {
		return nil, errs.ErrNothingWillBeUpdated
	}

	account.Extend.LastReconciledTime = &accountUpdateReq.LastReconciledTime

	err = a.accounts.UpdateAccountExtend(c, uid, account)

	if err != nil {
		log.Errorf(c, "[accounts.AccountUpdateLastReconciledTimeHandler] failed to update last reconciled time for account \"id:%d\" of user \"uid:%d\", because %s", account.AccountId, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[accounts.AccountUpdateLastReconciledTimeHandler] user \"uid:%d\" has updated last reconciled time \"%d\" for account \"id:%d\"", uid, account.Extend.LastReconciledTime, account.AccountId)
	return true, nil
}

// AccountHideHandler hides an existed account by request parameters for current user
func (a *AccountsApi) AccountHideHandler(c *core.WebContext) (any, *errs.Error) {
	var accountHideReq models.AccountHideRequest
	err := c.ShouldBindJSON(&accountHideReq)

	if err != nil {
		log.Warnf(c, "[accounts.AccountHideHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	err = a.accounts.HideAccount(c, uid, []int64{accountHideReq.Id}, accountHideReq.Hidden)

	if err != nil {
		log.Errorf(c, "[accounts.AccountHideHandler] failed to hide account \"id:%d\" for user \"uid:%d\", because %s", accountHideReq.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[accounts.AccountHideHandler] user \"uid:%d\" has hidden account \"id:%d\"", uid, accountHideReq.Id)
	return true, nil
}

// AccountMoveHandler moves display order of existed accounts by request parameters for current user
func (a *AccountsApi) AccountMoveHandler(c *core.WebContext) (any, *errs.Error) {
	var accountMoveReq models.AccountMoveRequest
	err := c.ShouldBindJSON(&accountMoveReq)

	if err != nil {
		log.Warnf(c, "[accounts.AccountMoveHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	accounts := make([]*models.Account, len(accountMoveReq.NewDisplayOrders))

	for i := 0; i < len(accountMoveReq.NewDisplayOrders); i++ {
		newDisplayOrder := accountMoveReq.NewDisplayOrders[i]
		account := &models.Account{
			Uid:          uid,
			AccountId:    newDisplayOrder.Id,
			DisplayOrder: newDisplayOrder.DisplayOrder,
		}

		accounts[i] = account
	}

	err = a.accounts.ModifyAccountDisplayOrders(c, uid, accounts)

	if err != nil {
		log.Errorf(c, "[accounts.AccountMoveHandler] failed to move accounts for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[accounts.AccountMoveHandler] user \"uid:%d\" has moved accounts", uid)
	return true, nil
}

// AccountDeleteHandler deletes an existed account by request parameters for current user
func (a *AccountsApi) AccountDeleteHandler(c *core.WebContext) (any, *errs.Error) {
	var accountDeleteReq models.AccountDeleteRequest
	err := c.ShouldBindJSON(&accountDeleteReq)

	if err != nil {
		log.Warnf(c, "[accounts.AccountDeleteHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	err = a.accounts.DeleteAccount(c, uid, accountDeleteReq.Id)

	if err != nil {
		log.Errorf(c, "[accounts.AccountDeleteHandler] failed to delete account \"id:%d\" for user \"uid:%d\", because %s", accountDeleteReq.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[accounts.AccountDeleteHandler] user \"uid:%d\" has deleted account \"id:%d\"", uid, accountDeleteReq.Id)

	err = a.creditCardPictures.RemovePicturesByAccountId(c, uid, accountDeleteReq.Id)

	if err != nil {
		log.Warnf(c, "[accounts.AccountDeleteHandler] failed to remove credit card pictures bound to account \"id:%d\" for user \"uid:%d\", because %s", accountDeleteReq.Id, uid, err.Error())
	}

	return true, nil
}

// SubAccountDeleteHandler deletes an existed sub-account by request parameters for current user
func (a *AccountsApi) SubAccountDeleteHandler(c *core.WebContext) (any, *errs.Error) {
	var accountDeleteReq models.AccountDeleteRequest
	err := c.ShouldBindJSON(&accountDeleteReq)

	if err != nil {
		log.Warnf(c, "[accounts.SubAccountDeleteHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	err = a.accounts.DeleteSubAccount(c, uid, accountDeleteReq.Id)

	if err != nil {
		log.Errorf(c, "[accounts.SubAccountDeleteHandler] failed to delete sub-account \"id:%d\" for user \"uid:%d\", because %s", accountDeleteReq.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[accounts.SubAccountDeleteHandler] user \"uid:%d\" has deleted sub-account \"id:%d\"", uid, accountDeleteReq.Id)
	return true, nil
}

// AccountReshapeHandler reshapes the account structure by either grouping level-one single
// accounts into a new multi-account parent, or moving selected sub-accounts of an existing group
// into a new multi-account parent. It is a structural-only operation and never migrates
// transactions, so it is only supported for non credit card accounts.
func (a *AccountsApi) AccountReshapeHandler(c *core.WebContext) (any, *errs.Error) {
	var accountReshapeReq models.AccountReshapeRequest
	err := c.ShouldBindJSON(&accountReshapeReq)

	if err != nil {
		log.Warnf(c, "[accounts.AccountReshapeHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()

	if accountReshapeReq.Operation == models.ACCOUNT_RESHAPE_OPERATION_GROUP {
		return a.reshapeGroupAccounts(c, uid, &accountReshapeReq)
	} else if accountReshapeReq.Operation == models.ACCOUNT_RESHAPE_OPERATION_SPLIT {
		return a.reshapeSplitAccounts(c, uid, &accountReshapeReq)
	}

	log.Warnf(c, "[accounts.AccountReshapeHandler] account reshape operation invalid, operation is %d", accountReshapeReq.Operation)
	return nil, errs.ErrAccountReshapeOperationInvalid
}

func (a *AccountsApi) reshapeGroupAccounts(c *core.WebContext, uid int64, accountReshapeReq *models.AccountReshapeRequest) (any, *errs.Error) {
	accountIds, err := utils.StringArrayToInt64Array(accountReshapeReq.AccountIds)

	if err != nil {
		return nil, errs.ErrAccountIdInvalid
	}

	if len(accountIds) < 1 {
		return nil, errs.ErrAccountReshapeAccountCountInvalid
	}

	accounts, err := a.accounts.GetAccountsByAccountIds(c, uid, accountIds)

	if err != nil {
		log.Errorf(c, "[accounts.AccountReshapeHandler] failed to get accounts for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	sourceAccounts := make([]*models.Account, 0, len(accountIds))
	var category models.AccountCategory
	seenAccountIds := make(map[int64]bool, len(accountIds))

	for i := 0; i < len(accountIds); i++ {
		account, exists := accounts[accountIds[i]]

		if !exists {
			return nil, errs.ErrAccountNotFound
		}

		if seenAccountIds[account.AccountId] {
			return nil, errs.ErrAccountReshapeAccountCountInvalid
		}

		seenAccountIds[account.AccountId] = true

		if account.ParentAccountId != models.LevelOneAccountParentId || account.Type != models.ACCOUNT_TYPE_SINGLE_ACCOUNT {
			log.Warnf(c, "[accounts.AccountReshapeHandler] source account \"id:%d\" is not a level-one single account", account.AccountId)
			return nil, errs.ErrAccountReshapeSourceAccountInvalid
		}

		if account.Category == models.ACCOUNT_CATEGORY_CREDIT_CARD {
			return nil, errs.ErrAccountReshapeNotSupportedForCreditCard
		}

		if i == 0 {
			category = account.Category
		} else if account.Category != category {
			return nil, errs.ErrAccountReshapeCategoryNotEquals
		}

		sourceAccounts = append(sourceAccounts, account)
	}

	firstAccount := sourceAccounts[0]
	newParentName := firstAccount.Name

	if accountReshapeReq.Name != "" {
		newParentName = accountReshapeReq.Name
	}

	newParentAccount := &models.Account{
		Category:     category,
		Name:         newParentName,
		Icon:         firstAccount.Icon,
		IconType:     firstAccount.IconType,
		Color:        firstAccount.Color,
		Comment:      "",
		DisplayOrder: firstAccount.DisplayOrder,
		Hidden:       firstAccount.Hidden,
	}

	err = a.accounts.ReshapeAccounts(c, uid, newParentAccount, sourceAccounts)

	if err != nil {
		log.Errorf(c, "[accounts.AccountReshapeHandler] failed to group accounts for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[accounts.AccountReshapeHandler] user \"uid:%d\" has grouped %d accounts into new parent \"id:%d\"", uid, len(sourceAccounts), newParentAccount.AccountId)
	return true, nil
}

func (a *AccountsApi) reshapeSplitAccounts(c *core.WebContext, uid int64, accountReshapeReq *models.AccountReshapeRequest) (any, *errs.Error) {
	if accountReshapeReq.ParentAccountId <= 0 {
		return nil, errs.ErrAccountIdInvalid
	}

	subAccountIds, err := utils.StringArrayToInt64Array(accountReshapeReq.SubAccountIds)

	if err != nil {
		return nil, errs.ErrAccountIdInvalid
	}

	if len(subAccountIds) < 1 {
		return nil, errs.ErrAccountReshapeAccountCountInvalid
	}

	parentAccount, err := a.accounts.GetAccountByAccountId(c, uid, accountReshapeReq.ParentAccountId)

	if err != nil {
		log.Errorf(c, "[accounts.AccountReshapeHandler] failed to get parent account \"id:%d\" for user \"uid:%d\", because %s", accountReshapeReq.ParentAccountId, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	if parentAccount.Type != models.ACCOUNT_TYPE_MULTI_SUB_ACCOUNTS {
		return nil, errs.ErrAccountReshapeSourceAccountInvalid
	}

	if parentAccount.Category == models.ACCOUNT_CATEGORY_CREDIT_CARD {
		return nil, errs.ErrAccountReshapeNotSupportedForCreditCard
	}

	allSubAccounts, err := a.accounts.GetSubAccountsByAccountId(c, uid, accountReshapeReq.ParentAccountId)

	if err != nil {
		log.Errorf(c, "[accounts.AccountReshapeHandler] failed to get sub-accounts of parent \"id:%d\" for user \"uid:%d\", because %s", accountReshapeReq.ParentAccountId, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	// the original parent must keep at least one sub-account
	if len(subAccountIds) >= len(allSubAccounts) {
		return nil, errs.ErrAccountHaveNoSubAccount
	}

	subAccountMap := a.accounts.GetAccountMapByList(allSubAccounts)
	sourceAccounts := make([]*models.Account, 0, len(subAccountIds))
	seenSubAccountIds := make(map[int64]bool, len(subAccountIds))

	for i := 0; i < len(subAccountIds); i++ {
		subAccount, exists := subAccountMap[subAccountIds[i]]

		if !exists {
			return nil, errs.ErrSubAccountNotFound
		}

		if seenSubAccountIds[subAccount.AccountId] {
			return nil, errs.ErrAccountReshapeAccountCountInvalid
		}

		seenSubAccountIds[subAccount.AccountId] = true
		sourceAccounts = append(sourceAccounts, subAccount)
	}

	firstAccount := sourceAccounts[0]
	newParentName := firstAccount.Name

	if accountReshapeReq.Name != "" {
		newParentName = accountReshapeReq.Name
	}

	maxOrderId, err := a.accounts.GetMaxDisplayOrder(c, uid, parentAccount.Category)

	if err != nil {
		log.Errorf(c, "[accounts.AccountReshapeHandler] failed to get max display order for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	newParentAccount := &models.Account{
		Category:     parentAccount.Category,
		Name:         newParentName,
		Icon:         firstAccount.Icon,
		IconType:     firstAccount.IconType,
		Color:        firstAccount.Color,
		Comment:      "",
		DisplayOrder: maxOrderId + 1,
		Hidden:       firstAccount.Hidden,
	}

	err = a.accounts.ReshapeAccounts(c, uid, newParentAccount, sourceAccounts)

	if err != nil {
		log.Errorf(c, "[accounts.AccountReshapeHandler] failed to split accounts for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[accounts.AccountReshapeHandler] user \"uid:%d\" has split %d sub-accounts from parent \"id:%d\" into new parent \"id:%d\"", uid, len(sourceAccounts), parentAccount.AccountId, newParentAccount.AccountId)
	return true, nil
}

func (a *AccountsApi) createNewAccountModel(uid int64, accountCreateReq *models.AccountCreateRequest, balance int64, creditLimitForCreditCard *int64, isSubAccount bool, order int32) *models.Account {
	accountExtend := &models.AccountExtend{}

	if accountCreateReq.Category == models.ACCOUNT_CATEGORY_CREDIT_CARD {
		if !isSubAccount || accountCreateReq.CreditCardStatementDate > 0 {
			accountExtend.CreditCardStatementDate = &accountCreateReq.CreditCardStatementDate
		}
		setCreditCardProfileFields(accountExtend, accountCreateReq.CreditCardBankName, accountCreateReq.CreditCardCardName, accountCreateReq.CreditCardLastFourDigits, accountCreateReq.CreditCardExpireMonth, accountCreateReq.CreditCardExpireYear, accountCreateReq.CreditCardCardholderName, accountCreateReq.CreditCardPictureId, accountCreateReq.CreditCardStatementDateIncludeTransaction, accountCreateReq.CreditCardRepaymentDateType, accountCreateReq.CreditCardRepaymentDaysAfterStatement, accountCreateReq.CreditCardRepaymentDayOfMonth)
		setCreditCardNotificationFields(accountExtend, getCreditCardNotificationParamsFromCreateRequest(accountCreateReq))
		setCreditCardEntityFields(accountExtend, accountCreateReq.CreditCardIsEntityCard, accountCreateReq.CreditCardCurrencyLimit, isSubAccount)

		// the credit limit belongs to the whole shared pool, never to a sub-account
		if !isSubAccount {
			accountExtend.CreditCardLimit = creditLimitForCreditCard
		}
	}

	return &models.Account{
		Uid:          uid,
		Name:         accountCreateReq.Name,
		DisplayOrder: order,
		Category:     accountCreateReq.Category,
		Type:         accountCreateReq.Type,
		Icon:         accountCreateReq.Icon,
		IconType:     accountCreateReq.IconType,
		Color:        accountCreateReq.Color,
		Currency:     accountCreateReq.Currency,
		Balance:      balance,
		Comment:      accountCreateReq.Comment,
		Extend:       accountExtend,
	}
}

func (a *AccountsApi) createNewSubAccountModelForModify(uid int64, accountType models.AccountType, accountModifyReq *models.AccountModifyRequest, balance int64, order int32) *models.Account {
	accountExtend := &models.AccountExtend{}

	if accountModifyReq.Category == models.ACCOUNT_CATEGORY_CREDIT_CARD {
		if accountModifyReq.CreditCardStatementDate > 0 {
			accountExtend.CreditCardStatementDate = &accountModifyReq.CreditCardStatementDate
		}
		setCreditCardProfileFields(accountExtend, accountModifyReq.CreditCardBankName, accountModifyReq.CreditCardCardName, accountModifyReq.CreditCardLastFourDigits, accountModifyReq.CreditCardExpireMonth, accountModifyReq.CreditCardExpireYear, accountModifyReq.CreditCardCardholderName, accountModifyReq.CreditCardPictureId, accountModifyReq.CreditCardStatementDateIncludeTransaction, accountModifyReq.CreditCardRepaymentDateType, accountModifyReq.CreditCardRepaymentDaysAfterStatement, accountModifyReq.CreditCardRepaymentDayOfMonth)
		setCreditCardNotificationFields(accountExtend, getCreditCardNotificationParamsFromModifyRequest(accountModifyReq))
		setCreditCardEntityFields(accountExtend, accountModifyReq.CreditCardIsEntityCard, accountModifyReq.CreditCardCurrencyLimit, true)
	}

	return &models.Account{
		Uid:          uid,
		Name:         accountModifyReq.Name,
		DisplayOrder: order,
		Category:     accountModifyReq.Category,
		Type:         accountType,
		Icon:         accountModifyReq.Icon,
		IconType:     accountModifyReq.IconType,
		Color:        accountModifyReq.Color,
		Currency:     *accountModifyReq.Currency,
		Balance:      balance,
		Comment:      accountModifyReq.Comment,
		Extend:       accountExtend,
	}
}

func (a *AccountsApi) createSubAccountModels(uid int64, accountCreateReq *models.AccountCreateRequest, balances []int64) ([]*models.Account, []int64) {
	if len(accountCreateReq.SubAccounts) <= 0 {
		return nil, nil
	}

	childrenAccounts := make([]*models.Account, len(accountCreateReq.SubAccounts))
	childrenAccountBalanceTimes := make([]int64, len(accountCreateReq.SubAccounts))

	for i := int32(0); i < int32(len(accountCreateReq.SubAccounts)); i++ {
		childrenAccounts[i] = a.createNewAccountModel(uid, accountCreateReq.SubAccounts[i], balances[i], nil, true, i+1)
		childrenAccountBalanceTimes[i] = accountCreateReq.SubAccounts[i].BalanceTime
	}

	return childrenAccounts, childrenAccountBalanceTimes
}

func (a *AccountsApi) getToUpdateAccount(user *models.User, accountModifyReq *models.AccountModifyRequest, creditLimitForCreditCard *int64, oldAccount *models.Account, isSubAccount bool) (*models.Account, error) {
	newAccountExtend := &models.AccountExtend{}
	newAccountExtend.LastReconciledTime = accountModifyReq.LastReconciledTime

	if accountModifyReq.Category == models.ACCOUNT_CATEGORY_CREDIT_CARD {
		if !isSubAccount || accountModifyReq.CreditCardStatementDate > 0 {
			newAccountExtend.CreditCardStatementDate = &accountModifyReq.CreditCardStatementDate
		}
		setCreditCardProfileFields(newAccountExtend, accountModifyReq.CreditCardBankName, accountModifyReq.CreditCardCardName, accountModifyReq.CreditCardLastFourDigits, accountModifyReq.CreditCardExpireMonth, accountModifyReq.CreditCardExpireYear, accountModifyReq.CreditCardCardholderName, accountModifyReq.CreditCardPictureId, accountModifyReq.CreditCardStatementDateIncludeTransaction, accountModifyReq.CreditCardRepaymentDateType, accountModifyReq.CreditCardRepaymentDaysAfterStatement, accountModifyReq.CreditCardRepaymentDayOfMonth)
		setCreditCardNotificationFields(newAccountExtend, getCreditCardNotificationParamsFromModifyRequest(accountModifyReq))
		setCreditCardEntityFields(newAccountExtend, accountModifyReq.CreditCardIsEntityCard, accountModifyReq.CreditCardCurrencyLimit, isSubAccount)

		// the credit limit belongs to the whole shared pool, never to a sub-account
		if !isSubAccount {
			newAccountExtend.CreditCardLimit = creditLimitForCreditCard
		}
	}

	newAccount := &models.Account{
		AccountId:    oldAccount.AccountId,
		Uid:          user.Uid,
		Name:         accountModifyReq.Name,
		DisplayOrder: oldAccount.DisplayOrder,
		Category:     accountModifyReq.Category,
		Icon:         accountModifyReq.Icon,
		IconType:     accountModifyReq.IconType,
		Color:        accountModifyReq.Color,
		Currency:     oldAccount.Currency,
		Comment:      accountModifyReq.Comment,
		Extend:       newAccountExtend,
		Hidden:       accountModifyReq.Hidden,
	}

	if !isSubAccount && accountModifyReq.Currency != nil {
		newAccount.Currency = *accountModifyReq.Currency
	}

	if newAccount.Name != oldAccount.Name ||
		newAccount.Category != oldAccount.Category ||
		newAccount.Icon != oldAccount.Icon ||
		newAccount.IconType != oldAccount.IconType ||
		newAccount.Color != oldAccount.Color ||
		newAccount.Currency != oldAccount.Currency ||
		newAccount.Comment != oldAccount.Comment ||
		newAccount.Hidden != oldAccount.Hidden {
		return newAccount, nil
	}

	oldAccountExtend := oldAccount.Extend

	if (newAccountExtend.LastReconciledTime != nil && (oldAccountExtend == nil || oldAccountExtend.LastReconciledTime == nil)) ||
		(newAccountExtend.LastReconciledTime == nil && oldAccountExtend != nil && oldAccountExtend.LastReconciledTime != nil) ||
		(newAccountExtend.LastReconciledTime != nil && oldAccountExtend != nil && oldAccountExtend.LastReconciledTime != nil && *newAccountExtend.LastReconciledTime != *oldAccountExtend.LastReconciledTime) {
		if !user.UseLastReconciledTime {
			return nil, errs.ErrLastReconciledTimeIsNotEnabled
		}

		return newAccount, nil
	}

	if (newAccountExtend.CreditCardStatementDate != nil && (oldAccountExtend == nil || oldAccountExtend.CreditCardStatementDate == nil)) ||
		(newAccountExtend.CreditCardStatementDate == nil && oldAccountExtend != nil && oldAccountExtend.CreditCardStatementDate != nil) ||
		(newAccountExtend.CreditCardStatementDate != nil && oldAccountExtend != nil && oldAccountExtend.CreditCardStatementDate != nil && *newAccountExtend.CreditCardStatementDate != *oldAccountExtend.CreditCardStatementDate) {
		return newAccount, nil
	}

	if (newAccountExtend.CreditCardLimit != nil && (oldAccountExtend == nil || oldAccountExtend.CreditCardLimit == nil)) ||
		(newAccountExtend.CreditCardLimit == nil && oldAccountExtend != nil && oldAccountExtend.CreditCardLimit != nil) ||
		(newAccountExtend.CreditCardLimit != nil && oldAccountExtend != nil && oldAccountExtend.CreditCardLimit != nil && *newAccountExtend.CreditCardLimit != *oldAccountExtend.CreditCardLimit) {
		return newAccount, nil
	}

	if isCreditCardProfileChanged(newAccountExtend, oldAccountExtend) {
		return newAccount, nil
	}

	if isCreditCardNotificationChanged(newAccountExtend, oldAccountExtend) {
		return newAccount, nil
	}

	if isCreditCardEntityChanged(newAccountExtend, oldAccountExtend) {
		return newAccount, nil
	}

	return nil, nil
}

func (a *AccountsApi) getToDeleteSubAccountIds(accountModifyReq *models.AccountModifyRequest, mainAccount *models.Account, accountAndSubAccounts []*models.Account) []int64 {
	newSubAccountIds := make(map[int64]bool, len(accountModifyReq.SubAccounts))

	for i := 0; i < len(accountModifyReq.SubAccounts); i++ {
		newSubAccountIds[accountModifyReq.SubAccounts[i].Id] = true
	}

	toDeleteAccountIds := make([]int64, 0)

	for i := 0; i < len(accountAndSubAccounts); i++ {
		subAccount := accountAndSubAccounts[i]

		if subAccount.AccountId == mainAccount.AccountId {
			continue
		}

		if _, exists := newSubAccountIds[subAccount.AccountId]; !exists {
			toDeleteAccountIds = append(toDeleteAccountIds, subAccount.AccountId)
		}
	}

	return toDeleteAccountIds
}

func (a *AccountsApi) isAccountsIconTypeValid(c *core.WebContext, uid int64, accounts []*models.Account) bool {
	iconIds := make([]int64, 0)

	for _, account := range accounts {
		if !account.IconType.IsValid() {
			return false
		}

		if account.IconType == core.ICON_TYPE_USER_CUSTOM {
			iconIds = append(iconIds, account.Icon)
		}
	}

	if len(iconIds) < 1 {
		return true
	}

	iconExists, err := a.icons.ExistsCustomIcons(c, uid, iconIds)

	if err != nil {
		log.Errorf(c, "[accounts.isAccountsIconTypeValid] failed to check custom icons for user \"uid:%d\", because %s", uid, err.Error())
		return false
	}

	return iconExists
}

// setCreditCardEntityFields stores per-card identity information. A nil isEntityCard means the
// account keeps its legacy behaviour (a physical card). Only a level-one account may be marked
// as a pure credit limit pool, so an explicit false is ignored for sub-accounts.
// currencyLimit is the shared pool limit stated by the bank in the account's own currency; the
// client uses the ratio of the stated limits to share the used limit between all cards of a pool.
func setCreditCardEntityFields(extend *models.AccountExtend, isEntityCard *bool, currencyLimit string, isSubAccount bool) {
	if isEntityCard != nil && !isSubAccount {
		extend.CreditCardIsEntityCard = isEntityCard
	}

	if currencyLimit != "" {
		value := currencyLimit
		extend.CreditCardCurrencyLimit = &value
	}
}

func hasCreditCardEntityFields(isEntityCard *bool, currencyLimit string) bool {
	return isEntityCard != nil || currencyLimit != ""
}

func isCreditCardEntityChanged(newExtend, oldExtend *models.AccountExtend) bool {
	return isBoolPtrChanged(newExtend.CreditCardIsEntityCard, oldExtend, func(e *models.AccountExtend) *bool { return e.CreditCardIsEntityCard }) ||
		isStringPtrChanged(newExtend.CreditCardCurrencyLimit, oldExtend, func(e *models.AccountExtend) *string { return e.CreditCardCurrencyLimit })
}

func hasCreditCardProfileFields(bankName, cardName, lastFourDigits string, expireMonth, expireYear int, cardholderName string, pictureId int64, statementDateIncludeTransaction bool, repaymentDateType, repaymentDaysAfterStatement, repaymentDayOfMonth int) bool {
	return bankName != "" || cardName != "" || lastFourDigits != "" || expireMonth != 0 || expireYear != 0 || cardholderName != "" || pictureId != 0 || statementDateIncludeTransaction || repaymentDateType != 0 || repaymentDaysAfterStatement != 0 || repaymentDayOfMonth != 0
}

func setCreditCardProfileFields(extend *models.AccountExtend, bankName, cardName, lastFourDigits string, expireMonth, expireYear int, cardholderName string, pictureId int64, statementDateIncludeTransaction bool, repaymentDateType, repaymentDaysAfterStatement, repaymentDayOfMonth int) {
	if bankName != "" {
		extend.CreditCardBankName = &bankName
	}
	if cardName != "" {
		extend.CreditCardCardName = &cardName
	}
	if lastFourDigits != "" {
		extend.CreditCardLastFourDigits = &lastFourDigits
	}
	if expireMonth != 0 {
		extend.CreditCardExpireMonth = &expireMonth
	}
	if expireYear != 0 {
		extend.CreditCardExpireYear = &expireYear
	}
	if cardholderName != "" {
		extend.CreditCardCardholderName = &cardholderName
	}
	if pictureId != 0 {
		extend.CreditCardPictureId = &pictureId
	}
	if statementDateIncludeTransaction {
		extend.CreditCardStatementDateIncludeTransaction = &statementDateIncludeTransaction
	}
	if repaymentDateType != 0 {
		extend.CreditCardRepaymentDateType = &repaymentDateType
	}
	if repaymentDaysAfterStatement != 0 {
		extend.CreditCardRepaymentDaysAfterStatement = &repaymentDaysAfterStatement
	}
	if repaymentDayOfMonth != 0 {
		extend.CreditCardRepaymentDayOfMonth = &repaymentDayOfMonth
	}
}

func isCreditCardProfileChanged(newExtend, oldExtend *models.AccountExtend) bool {
	return isStringPtrChanged(newExtend.CreditCardBankName, oldExtend, func(e *models.AccountExtend) *string { return e.CreditCardBankName }) ||
		isStringPtrChanged(newExtend.CreditCardCardName, oldExtend, func(e *models.AccountExtend) *string { return e.CreditCardCardName }) ||
		isStringPtrChanged(newExtend.CreditCardLastFourDigits, oldExtend, func(e *models.AccountExtend) *string { return e.CreditCardLastFourDigits }) ||
		isIntPtrChanged(newExtend.CreditCardExpireMonth, oldExtend, func(e *models.AccountExtend) *int { return e.CreditCardExpireMonth }) ||
		isIntPtrChanged(newExtend.CreditCardExpireYear, oldExtend, func(e *models.AccountExtend) *int { return e.CreditCardExpireYear }) ||
		isStringPtrChanged(newExtend.CreditCardCardholderName, oldExtend, func(e *models.AccountExtend) *string { return e.CreditCardCardholderName }) ||
		isInt64PtrChanged(newExtend.CreditCardPictureId, oldExtend, func(e *models.AccountExtend) *int64 { return e.CreditCardPictureId }) ||
		isBoolPtrChanged(newExtend.CreditCardStatementDateIncludeTransaction, oldExtend, func(e *models.AccountExtend) *bool { return e.CreditCardStatementDateIncludeTransaction }) ||
		isIntPtrChanged(newExtend.CreditCardRepaymentDateType, oldExtend, func(e *models.AccountExtend) *int { return e.CreditCardRepaymentDateType }) ||
		isIntPtrChanged(newExtend.CreditCardRepaymentDaysAfterStatement, oldExtend, func(e *models.AccountExtend) *int { return e.CreditCardRepaymentDaysAfterStatement }) ||
		isIntPtrChanged(newExtend.CreditCardRepaymentDayOfMonth, oldExtend, func(e *models.AccountExtend) *int { return e.CreditCardRepaymentDayOfMonth })
}

func isStringPtrChanged(newVal *string, oldExtend *models.AccountExtend, getter func(*models.AccountExtend) *string) bool {
	var oldVal *string
	if oldExtend != nil {
		oldVal = getter(oldExtend)
	}
	if newVal == nil && oldVal == nil {
		return false
	}
	if newVal == nil || oldVal == nil {
		return true
	}
	return *newVal != *oldVal
}

func isIntPtrChanged(newVal *int, oldExtend *models.AccountExtend, getter func(*models.AccountExtend) *int) bool {
	var oldVal *int
	if oldExtend != nil {
		oldVal = getter(oldExtend)
	}
	if newVal == nil && oldVal == nil {
		return false
	}
	if newVal == nil || oldVal == nil {
		return true
	}
	return *newVal != *oldVal
}

func isInt64PtrChanged(newVal *int64, oldExtend *models.AccountExtend, getter func(*models.AccountExtend) *int64) bool {
	var oldVal *int64
	if oldExtend != nil {
		oldVal = getter(oldExtend)
	}
	if newVal == nil && oldVal == nil {
		return false
	}
	if newVal == nil || oldVal == nil {
		return true
	}
	return *newVal != *oldVal
}

func isBoolPtrChanged(newVal *bool, oldExtend *models.AccountExtend, getter func(*models.AccountExtend) *bool) bool {
	var oldVal *bool
	if oldExtend != nil {
		oldVal = getter(oldExtend)
	}
	if newVal == nil && oldVal == nil {
		return false
	}
	if newVal == nil || oldVal == nil {
		return true
	}
	return *newVal != *oldVal
}

// creditCardNotificationParams represents credit card notification settings in an account request
type creditCardNotificationParams struct {
	statementReminderEnabled      bool
	statementEmailEnabled         bool
	repaymentReminderEnabled      bool
	repaymentReminderDaysBefore   int
	notificationMinutesOfDay      *int
	notificationTimezoneName      *string
	notificationTimezoneUtcOffset *int16
}

func getCreditCardNotificationParamsFromCreateRequest(accountCreateReq *models.AccountCreateRequest) creditCardNotificationParams {
	return creditCardNotificationParams{
		statementReminderEnabled:      accountCreateReq.CreditCardStatementReminderEnabled,
		statementEmailEnabled:         accountCreateReq.CreditCardStatementEmailEnabled,
		repaymentReminderEnabled:      accountCreateReq.CreditCardRepaymentReminderEnabled,
		repaymentReminderDaysBefore:   accountCreateReq.CreditCardRepaymentReminderDaysBefore,
		notificationMinutesOfDay:      accountCreateReq.CreditCardNotificationMinutesOfDay,
		notificationTimezoneName:      accountCreateReq.CreditCardNotificationTimezoneName,
		notificationTimezoneUtcOffset: accountCreateReq.CreditCardNotificationTimezoneUtcOffset,
	}
}

func getCreditCardNotificationParamsFromModifyRequest(accountModifyReq *models.AccountModifyRequest) creditCardNotificationParams {
	return creditCardNotificationParams{
		statementReminderEnabled:      accountModifyReq.CreditCardStatementReminderEnabled,
		statementEmailEnabled:         accountModifyReq.CreditCardStatementEmailEnabled,
		repaymentReminderEnabled:      accountModifyReq.CreditCardRepaymentReminderEnabled,
		repaymentReminderDaysBefore:   accountModifyReq.CreditCardRepaymentReminderDaysBefore,
		notificationMinutesOfDay:      accountModifyReq.CreditCardNotificationMinutesOfDay,
		notificationTimezoneName:      accountModifyReq.CreditCardNotificationTimezoneName,
		notificationTimezoneUtcOffset: accountModifyReq.CreditCardNotificationTimezoneUtcOffset,
	}
}

func hasCreditCardNotificationFields(notificationParams creditCardNotificationParams) bool {
	return notificationParams.statementReminderEnabled || notificationParams.statementEmailEnabled ||
		notificationParams.repaymentReminderEnabled || notificationParams.repaymentReminderDaysBefore != 0 ||
		notificationParams.notificationMinutesOfDay != nil || notificationParams.notificationTimezoneName != nil ||
		notificationParams.notificationTimezoneUtcOffset != nil
}

func setCreditCardNotificationFields(extend *models.AccountExtend, notificationParams creditCardNotificationParams) {
	if notificationParams.statementReminderEnabled {
		extend.CreditCardStatementReminderEnabled = &notificationParams.statementReminderEnabled
	}
	if notificationParams.statementEmailEnabled {
		extend.CreditCardStatementEmailEnabled = &notificationParams.statementEmailEnabled
	}
	if notificationParams.repaymentReminderEnabled {
		extend.CreditCardRepaymentReminderEnabled = &notificationParams.repaymentReminderEnabled
	}
	if notificationParams.repaymentReminderDaysBefore != 0 {
		extend.CreditCardRepaymentReminderDaysBefore = &notificationParams.repaymentReminderDaysBefore
	}
	if notificationParams.notificationMinutesOfDay != nil {
		extend.CreditCardNotificationMinutesOfDay = notificationParams.notificationMinutesOfDay
	}
	if notificationParams.notificationTimezoneName != nil {
		extend.CreditCardNotificationTimezoneName = notificationParams.notificationTimezoneName
	}
	if notificationParams.notificationTimezoneUtcOffset != nil {
		extend.CreditCardNotificationTimezoneUtcOffset = notificationParams.notificationTimezoneUtcOffset
	}
}

func isCreditCardNotificationParamsValid(notificationParams creditCardNotificationParams) bool {
	if !notificationParams.statementReminderEnabled && !notificationParams.statementEmailEnabled && !notificationParams.repaymentReminderEnabled {
		return true
	}

	if notificationParams.notificationTimezoneName == nil {
		return false
	}

	_, err := time.LoadLocation(*notificationParams.notificationTimezoneName)

	return err == nil
}

func isCreditCardNotificationChanged(newExtend, oldExtend *models.AccountExtend) bool {
	return isBoolPtrChanged(newExtend.CreditCardStatementReminderEnabled, oldExtend, func(e *models.AccountExtend) *bool { return e.CreditCardStatementReminderEnabled }) ||
		isBoolPtrChanged(newExtend.CreditCardStatementEmailEnabled, oldExtend, func(e *models.AccountExtend) *bool { return e.CreditCardStatementEmailEnabled }) ||
		isBoolPtrChanged(newExtend.CreditCardRepaymentReminderEnabled, oldExtend, func(e *models.AccountExtend) *bool { return e.CreditCardRepaymentReminderEnabled }) ||
		isIntPtrChanged(newExtend.CreditCardRepaymentReminderDaysBefore, oldExtend, func(e *models.AccountExtend) *int { return e.CreditCardRepaymentReminderDaysBefore }) ||
		isIntPtrChanged(newExtend.CreditCardNotificationMinutesOfDay, oldExtend, func(e *models.AccountExtend) *int { return e.CreditCardNotificationMinutesOfDay }) ||
		isStringPtrChanged(newExtend.CreditCardNotificationTimezoneName, oldExtend, func(e *models.AccountExtend) *string { return e.CreditCardNotificationTimezoneName }) ||
		isValuePtrChanged(newExtend.CreditCardNotificationTimezoneUtcOffset, oldExtend, func(e *models.AccountExtend) *int16 { return e.CreditCardNotificationTimezoneUtcOffset })
}

func isValuePtrChanged[T comparable](newVal *T, oldExtend *models.AccountExtend, getter func(*models.AccountExtend) *T) bool {
	var oldVal *T
	if oldExtend != nil {
		oldVal = getter(oldExtend)
	}
	if newVal == nil && oldVal == nil {
		return false
	}
	if newVal == nil || oldVal == nil {
		return true
	}
	return *newVal != *oldVal
}

// resetCreditCardNotificationScan makes the notification settings just saved picked up on the next
// cron tick, instead of after the whole scan interval chosen by the user
func (a *AccountsApi) resetCreditCardNotificationScan(c *core.WebContext, uid int64, category models.AccountCategory) {
	if category != models.ACCOUNT_CATEGORY_CREDIT_CARD {
		return
	}

	if err := a.users.ResetUserCreditCardNotificationLastScanTime(c, uid); err != nil {
		log.Warnf(c, "[accounts.resetCreditCardNotificationScan] failed to reset credit card notification scan time for user \"uid:%d\", because %s", uid, err.Error())
	}
}

// fillCreditCardPictureUrls adds the url of the credit card picture to the given account responses
func (a *AccountsApi) fillCreditCardPictureUrls(c *core.WebContext, uid int64, accountResps ...*models.AccountInfoResponse) {
	// sub-accounts may carry their own card picture as well
	allAccountResps := make([]*models.AccountInfoResponse, 0, len(accountResps))

	for i := 0; i < len(accountResps); i++ {
		allAccountResps = append(allAccountResps, accountResps[i])

		for j := 0; j < len(accountResps[i].SubAccounts); j++ {
			allAccountResps = append(allAccountResps, accountResps[i].SubAccounts[j])
		}
	}

	accountResps = allAccountResps
	pictureIds := make([]int64, 0, len(accountResps))

	for i := 0; i < len(accountResps); i++ {
		if accountResps[i].CreditCardPictureId != nil && *accountResps[i].CreditCardPictureId > 0 {
			pictureIds = append(pictureIds, *accountResps[i].CreditCardPictureId)
		}
	}

	if len(pictureIds) == 0 {
		return
	}

	pictureInfoMap, err := a.creditCardPictures.GetPictureInfoMapByPictureIds(c, uid, pictureIds)

	if err != nil {
		log.Errorf(c, "[accounts.fillCreditCardPictureUrls] failed to get credit card pictures for user \"uid:%d\", because %s", uid, err.Error())
		return
	}

	pictureToken, err := a.getCreditCardPictureToken(c, uid)

	if err != nil {
		log.Errorf(c, "[accounts.fillCreditCardPictureUrls] failed to get credit card picture token for user \"uid:%d\", because %s", uid, err.Error())
		return
	}

	for i := 0; i < len(accountResps); i++ {
		accountResp := accountResps[i]

		if accountResp.CreditCardPictureId == nil {
			continue
		}

		pictureInfo, exists := pictureInfoMap[*accountResp.CreditCardPictureId]

		if !exists {
			accountResp.CreditCardPictureId = nil
			continue
		}

		accountResp.CreditCardPictureUrl = a.GetCreditCardPictureInfoResponse(pictureInfo, pictureToken).OriginalUrl
	}
}

func (a *AccountsApi) getCreditCardPictureToken(c *core.WebContext, uid int64) (string, error) {
	claims := c.GetTokenClaims()

	if claims == nil {
		log.Warnf(c, "[accounts.getCreditCardPictureToken] token is invalid, because claims is nil")
		return "", errs.ErrInvalidToken
	}

	if claims.Type != core.USER_TOKEN_TYPE_NORMAL {
		return "", nil
	}

	pictureToken, err := a.tokens.DeriveCreditCardPictureToken(c, claims, c.GetTokenRecord())

	if err != nil {
		log.Errorf(c, "[accounts.getCreditCardPictureToken] failed to derive credit card picture token for user \"uid:%d\", because %s", uid, err.Error())
		return "", errs.Or(err, errs.ErrTokenGenerating)
	}

	return pictureToken, nil
}

func getCreditCardPictureId(extend *models.AccountExtend) int64 {
	if extend == nil || extend.CreditCardPictureId == nil {
		return 0
	}

	return *extend.CreditCardPictureId
}
