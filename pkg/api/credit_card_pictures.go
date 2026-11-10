package api

import (
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/duplicatechecker"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/services"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

// CreditCardPicturesApi represents credit card pictures api
type CreditCardPicturesApi struct {
	ApiUsingConfig
	ApiUsingDuplicateChecker
	users              *services.UserService
	tokens             *services.TokenService
	creditCardPictures *services.CreditCardPictureService
}

// Initialize a credit card picture api singleton instance
var (
	CreditCardPictures = &CreditCardPicturesApi{
		ApiUsingConfig: ApiUsingConfig{
			container: settings.Container,
		},
		ApiUsingDuplicateChecker: ApiUsingDuplicateChecker{
			ApiUsingConfig: ApiUsingConfig{
				container: settings.Container,
			},
			duplicateChecker: duplicatechecker.Container,
		},
		users:              services.Users,
		tokens:             services.Tokens,
		creditCardPictures: services.CreditCardPictures,
	}
)

// CreditCardPictureUploadHandler saves credit card picture by request parameters for current user
func (a *CreditCardPicturesApi) CreditCardPictureUploadHandler(c *core.WebContext) (any, *errs.Error) {
	uid := c.GetCurrentUid()
	form, err := c.MultipartForm()

	if err != nil {
		log.Errorf(c, "[credit_card_pictures.CreditCardPictureUploadHandler] failed to get multi-part form data for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.ErrParameterInvalid
	}

	pictureFiles := form.File["picture"]

	if len(pictureFiles) < 1 {
		log.Warnf(c, "[credit_card_pictures.CreditCardPictureUploadHandler] there is no credit card picture in request for user \"uid:%d\"", uid)
		return nil, errs.ErrNoCreditCardPicture
	}

	if pictureFiles[0].Size < 1 {
		log.Warnf(c, "[credit_card_pictures.CreditCardPictureUploadHandler] the size of credit card picture in request is zero for user \"uid:%d\"", uid)
		return nil, errs.ErrCreditCardPictureIsEmpty
	}

	if pictureFiles[0].Size > int64(a.CurrentConfig().MaxCreditCardPictureFileSize) {
		log.Warnf(c, "[credit_card_pictures.CreditCardPictureUploadHandler] the upload file size \"%d\" exceeds the maximum size \"%d\" of credit card picture for user \"uid:%d\"", pictureFiles[0].Size, a.CurrentConfig().MaxCreditCardPictureFileSize, uid)
		return nil, errs.ErrExceedMaxCreditCardPictureFileSize
	}

	fileExtension := utils.GetFileNameExtension(pictureFiles[0].Filename)

	if utils.GetImageContentType(fileExtension) == "" {
		log.Warnf(c, "[credit_card_pictures.CreditCardPictureUploadHandler] the file extension \"%s\" of credit card picture in request is not supported for user \"uid:%d\"", fileExtension, uid)
		return nil, errs.ErrImageTypeNotSupported
	}

	pictureFile, err := pictureFiles[0].Open()

	if err != nil {
		log.Errorf(c, "[credit_card_pictures.CreditCardPictureUploadHandler] failed to get credit card picture file from request for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.ErrOperationFailed
	}

	pictureInfo := a.createNewPictureInfoModel(uid, fileExtension, c.ClientIP())

	clientSessionIds := form.Value["clientSessionId"]
	clientSessionId := ""

	if len(clientSessionIds) > 0 {
		clientSessionId = clientSessionIds[0]
	}

	if a.CurrentConfig().EnableDuplicateSubmissionsCheck && clientSessionId != "" {
		found, remark := a.GetSubmissionRemark(duplicatechecker.DUPLICATE_CHECKER_TYPE_NEW_CREDIT_CARD_PICTURE, uid, clientSessionId)

		if found {
			log.Infof(c, "[credit_card_pictures.CreditCardPictureUploadHandler] another credit card picture \"id:%s\" has been uploaded for user \"uid:%d\"", remark, uid)
			pictureId, err := utils.StringToInt64(remark)

			if err == nil {
				pictureInfo, err = a.creditCardPictures.GetPictureInfoByPictureId(c, uid, pictureId)

				if err != nil {
					log.Errorf(c, "[credit_card_pictures.CreditCardPictureUploadHandler] failed to get existed credit card picture \"id:%d\" for user \"uid:%d\", because %s", pictureId, uid, err.Error())
					return nil, errs.Or(err, errs.ErrOperationFailed)
				}

				return a.getCreditCardPictureInfoResponse(c, uid, pictureInfo)
			}
		}
	}

	err = a.creditCardPictures.UploadPicture(c, pictureInfo, pictureFile)

	if err != nil {
		log.Errorf(c, "[credit_card_pictures.CreditCardPictureUploadHandler] failed to upload credit card picture for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	a.SetSubmissionRemarkIfEnable(duplicatechecker.DUPLICATE_CHECKER_TYPE_NEW_CREDIT_CARD_PICTURE, uid, clientSessionId, utils.Int64ToString(pictureInfo.PictureId))

	return a.getCreditCardPictureInfoResponse(c, uid, pictureInfo)
}

// CreditCardPictureGetHandler returns credit card picture data for current user
func (a *CreditCardPicturesApi) CreditCardPictureGetHandler(c *core.WebContext) ([]byte, string, *errs.Error) {
	fileName := c.Param("fileName")
	fileExtension := utils.GetFileNameExtension(fileName)
	contentType := utils.GetImageContentType(fileExtension)

	if contentType == "" {
		return nil, "", errs.ErrImageTypeNotSupported
	}

	fileBaseName := utils.GetFileNameWithoutExtension(fileName)
	pictureId, err := utils.StringToInt64(fileBaseName)

	if err != nil {
		return nil, "", errs.ErrCreditCardPictureIdInvalid
	}

	uid := c.GetCurrentUid()
	pictureData, err := a.creditCardPictures.GetPictureByPictureId(c, uid, pictureId, fileExtension)

	if err != nil {
		if !errs.IsCustomError(err) {
			log.Errorf(c, "[credit_card_pictures.CreditCardPictureGetHandler] failed to get credit card picture, because %s", err.Error())
		}

		return nil, "", errs.Or(err, errs.ErrOperationFailed)
	}

	return pictureData, contentType, nil
}

// CreditCardPictureRemoveUnusedHandler removes unused credit card picture by request parameters for current user
func (a *CreditCardPicturesApi) CreditCardPictureRemoveUnusedHandler(c *core.WebContext) (any, *errs.Error) {
	var pictureDeleteReq models.CreditCardPictureUnusedDeleteRequest
	err := c.ShouldBindJSON(&pictureDeleteReq)

	if err != nil {
		log.Warnf(c, "[credit_card_pictures.CreditCardPictureRemoveUnusedHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	err = a.creditCardPictures.RemoveUnusedCreditCardPicture(c, uid, pictureDeleteReq.Id)

	if err != nil {
		log.Errorf(c, "[credit_card_pictures.CreditCardPictureRemoveUnusedHandler] failed to remove unused credit card picture for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return true, nil
}

func (a *CreditCardPicturesApi) getCreditCardPictureInfoResponse(c *core.WebContext, uid int64, pictureInfo *models.CreditCardPictureInfo) (any, *errs.Error) {
	pictureToken, err := a.GetCreditCardPictureToken(c, uid)

	if err != nil {
		log.Errorf(c, "[credit_card_pictures.CreditCardPictureUploadHandler] failed to get credit card picture token for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrTokenGenerating)
	}

	return a.GetCreditCardPictureInfoResponse(pictureInfo, pictureToken), nil
}

func (a *CreditCardPicturesApi) GetCreditCardPictureToken(c *core.WebContext, uid int64) (string, error) {
	claims := c.GetTokenClaims()

	if claims == nil {
		log.Warnf(c, "[credit_card_pictures.GetCreditCardPictureToken] token is invalid, because claims is nil or token type is not normal")
		return "", errs.ErrInvalidToken
	}

	if claims.Type != core.USER_TOKEN_TYPE_NORMAL {
		return "", nil
	}

	pictureToken, err := a.tokens.DeriveCreditCardPictureToken(c, claims, c.GetTokenRecord())

	if err != nil {
		log.Errorf(c, "[credit_card_pictures.GetCreditCardPictureToken] failed to derive credit card picture token for user \"uid:%d\", because %s", uid, err.Error())
		return "", errs.Or(err, errs.ErrTokenGenerating)
	}

	return pictureToken, nil
}

func (a *CreditCardPicturesApi) createNewPictureInfoModel(uid int64, fileExtension string, clientIp string) *models.CreditCardPictureInfo {
	return &models.CreditCardPictureInfo{
		Uid:              uid,
		AccountId:        models.CreditCardPictureNewPictureAccountId,
		PictureExtension: fileExtension,
		CreatedIp:        clientIp,
	}
}
