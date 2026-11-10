package services

import (
	"encoding/base64"
	"io"
	"mime/multipart"
	"strings"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/storage"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
	"github.com/mayswind/ezbookkeeping/pkg/uuid"
)

const creditCardPictureDataUrlPrefix = "data:image/"

// CreditCardPictureService represents credit card picture service
type CreditCardPictureService struct {
	ServiceUsingDB
	ServiceUsingUuid
	ServiceUsingStorage
}

// Initialize a credit card picture service singleton instance
var (
	CreditCardPictures = &CreditCardPictureService{
		ServiceUsingDB: ServiceUsingDB{
			container: datastore.Container,
		},
		ServiceUsingUuid: ServiceUsingUuid{
			container: uuid.Container,
		},
		ServiceUsingStorage: ServiceUsingStorage{
			container: storage.Container,
		},
	}
)

// GetPictureInfoByPictureId returns a credit card picture info model according to credit card picture id
func (s *CreditCardPictureService) GetPictureInfoByPictureId(c core.Context, uid int64, pictureId int64) (*models.CreditCardPictureInfo, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	if pictureId <= 0 {
		return nil, errs.ErrCreditCardPictureIdInvalid
	}

	pictureInfo := &models.CreditCardPictureInfo{}
	has, err := s.UserDataDB(uid).NewSession(c).ID(pictureId).Where("uid=? AND deleted=?", uid, false).Get(pictureInfo)

	if err != nil {
		return nil, err
	} else if !has {
		return nil, errs.ErrCreditCardPictureNotFound
	}

	return pictureInfo, nil
}

// GetPictureByPictureId returns the credit card picture data according to credit card picture id
func (s *CreditCardPictureService) GetPictureByPictureId(c core.Context, uid int64, pictureId int64, fileExtension string) ([]byte, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	if pictureId <= 0 {
		return nil, errs.ErrCreditCardPictureIdInvalid
	}

	pictureInfo := &models.CreditCardPictureInfo{}
	has, err := s.UserDataDB(uid).NewSession(c).ID(pictureId).Where("uid=? AND deleted=?", uid, false).Get(pictureInfo)

	if err != nil {
		return nil, err
	} else if !has {
		return nil, errs.ErrCreditCardPictureNotFound
	}

	if pictureInfo.PictureExtension == "" {
		return nil, errs.ErrCreditCardPictureNotFound
	}

	if pictureInfo.PictureExtension != fileExtension {
		return nil, errs.ErrCreditCardPictureExtensionInvalid
	}

	pictureFile, exists, err := s.ReadCreditCardPicture(c, pictureInfo.Uid, pictureInfo.PictureId, pictureInfo.PictureExtension)

	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, errs.ErrCreditCardPictureNoExists
	}

	defer pictureFile.Close()

	pictureData, err := io.ReadAll(pictureFile)

	if err != nil {
		return nil, err
	}

	return pictureData, nil
}

// GetPictureInfoMapByPictureIds returns a credit card picture info map according to credit card picture ids
func (s *CreditCardPictureService) GetPictureInfoMapByPictureIds(c core.Context, uid int64, pictureIds []int64) (map[int64]*models.CreditCardPictureInfo, error) {
	pictureInfoMap := make(map[int64]*models.CreditCardPictureInfo)

	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	if len(pictureIds) < 1 {
		return pictureInfoMap, nil
	}

	var pictureInfos []*models.CreditCardPictureInfo
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=?", uid, false).In("picture_id", pictureIds).OrderBy("picture_id asc").Find(&pictureInfos)

	if err != nil {
		return nil, err
	}

	for i := 0; i < len(pictureInfos); i++ {
		pictureInfoMap[pictureInfos[i].PictureId] = pictureInfos[i]
	}

	return pictureInfoMap, nil
}

// UploadPicture uploads the credit card picture for specified user
func (s *CreditCardPictureService) UploadPicture(c core.Context, pictureInfo *models.CreditCardPictureInfo, pictureFile multipart.File) error {
	if pictureInfo.Uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	defer pictureFile.Close()

	now := time.Now().Unix()
	pictureInfo.PictureId = s.GenerateUuid(uuid.UUID_TYPE_CREDIT_CARD_PICTURE)

	if pictureInfo.PictureId < 1 {
		return errs.ErrSystemIsBusy
	}

	pictureInfo.AccountId = models.CreditCardPictureNewPictureAccountId
	pictureInfo.Deleted = false
	pictureInfo.CreatedUnixTime = now
	pictureInfo.UpdatedUnixTime = now

	err := s.SaveCreditCardPicture(c, pictureInfo.Uid, pictureInfo.PictureId, pictureFile, pictureInfo.PictureExtension)

	if err != nil {
		return err
	}

	return s.UserDataDB(pictureInfo.Uid).DoTransaction(c, func(sess datastore.DatabaseSession) error {
		_, err := sess.Insert(pictureInfo)
		return err
	})
}

// CheckPictureCanBeBound checks whether the specified credit card picture can be bound to specified account
func (s *CreditCardPictureService) CheckPictureCanBeBound(c core.Context, uid int64, accountId int64, pictureId int64) error {
	if pictureId <= 0 {
		return errs.ErrCreditCardPictureIdInvalid
	}

	pictureInfo, err := s.GetPictureInfoByPictureId(c, uid, pictureId)

	if err != nil {
		return err
	}

	if pictureInfo.AccountId != models.CreditCardPictureNewPictureAccountId && pictureInfo.AccountId != accountId {
		return errs.ErrCreditCardPictureIdInvalid
	}

	return nil
}

// BindPictureToAccount binds a credit card picture to specified account and removes the picture bound to this account before
func (s *CreditCardPictureService) BindPictureToAccount(c core.Context, uid int64, accountId int64, pictureId int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if accountId <= 0 {
		return errs.ErrAccountIdInvalid
	}

	now := time.Now().Unix()

	return s.UserDataDB(uid).DoTransaction(c, func(sess datastore.DatabaseSession) error {
		deleteUpdateModel := &models.CreditCardPictureInfo{
			Deleted:         true,
			DeletedUnixTime: now,
		}

		_, err := sess.Cols("deleted", "deleted_unix_time").Where("uid=? AND deleted=? AND account_id=?", uid, false, accountId).NotIn("picture_id", pictureId).Update(deleteUpdateModel)

		if err != nil {
			return err
		}

		if pictureId <= 0 {
			return nil
		}

		bindUpdateModel := &models.CreditCardPictureInfo{
			AccountId:       accountId,
			UpdatedUnixTime: now,
		}

		_, err = sess.ID(pictureId).Cols("account_id", "updated_unix_time").Where("uid=? AND deleted=? AND account_id=?", uid, false, models.CreditCardPictureNewPictureAccountId).Update(bindUpdateModel)
		return err
	})
}

// RemoveUnusedCreditCardPicture removes the unused credit card picture of specified user
func (s *CreditCardPictureService) RemoveUnusedCreditCardPicture(c core.Context, uid int64, pictureId int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if pictureId <= 0 {
		return errs.ErrCreditCardPictureIdInvalid
	}

	now := time.Now().Unix()

	updateModel := &models.CreditCardPictureInfo{
		Deleted:         true,
		DeletedUnixTime: now,
	}

	deletedRows, err := s.UserDataDB(uid).NewSession(c).ID(pictureId).Cols("deleted", "deleted_unix_time").Where("uid=? AND deleted=? AND account_id=?", uid, false, models.CreditCardPictureNewPictureAccountId).Update(updateModel)

	if err != nil {
		return err
	} else if deletedRows < 1 {
		return errs.ErrCreditCardPictureNotFound
	}

	return nil
}

// RemovePicturesByAccountId removes all credit card pictures bound to specified account
func (s *CreditCardPictureService) RemovePicturesByAccountId(c core.Context, uid int64, accountId int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if accountId <= 0 {
		return errs.ErrAccountIdInvalid
	}

	now := time.Now().Unix()

	updateModel := &models.CreditCardPictureInfo{
		Deleted:         true,
		DeletedUnixTime: now,
	}

	_, err := s.UserDataDB(uid).NewSession(c).Cols("deleted", "deleted_unix_time").Where("uid=? AND deleted=? AND account_id=?", uid, false, accountId).Update(updateModel)
	return err
}

// MigrateBase64Pictures converts all credit card pictures stored in account extend data to picture files
func (s *CreditCardPictureService) MigrateBase64Pictures(c core.Context) error {
	var users []*models.User
	err := s.UserDB().NewSession(c).Select("uid").Where("deleted=?", false).Find(&users)

	if err != nil {
		return err
	}

	for i := 0; i < len(users); i++ {
		uid := users[i].Uid
		var accounts []*models.Account
		err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND category=?", uid, false, models.ACCOUNT_CATEGORY_CREDIT_CARD).OrderBy("account_id asc").Find(&accounts)

		if err != nil {
			log.Errorf(c, "[credit_card_pictures.MigrateBase64Pictures] failed to get accounts for user \"uid:%d\", because %s", uid, err.Error())
			continue
		}

		for j := 0; j < len(accounts); j++ {
			account := accounts[j]

			if account.Extend == nil || account.Extend.CreditCardPicture == nil || *account.Extend.CreditCardPicture == "" {
				continue
			}

			err = s.migrateBase64Picture(c, uid, account)

			if err != nil {
				log.Errorf(c, "[credit_card_pictures.MigrateBase64Pictures] failed to migrate credit card picture for account \"id:%d\" of user \"uid:%d\", because %s", account.AccountId, uid, err.Error())
			}
		}
	}

	return nil
}

func (s *CreditCardPictureService) migrateBase64Picture(c core.Context, uid int64, account *models.Account) error {
	pictureData, fileExtension, err := decodeCreditCardPictureDataUrl(*account.Extend.CreditCardPicture)

	if err != nil {
		return err
	}

	now := time.Now().Unix()
	pictureId := s.GenerateUuid(uuid.UUID_TYPE_CREDIT_CARD_PICTURE)

	if pictureId < 1 {
		return errs.ErrSystemIsBusy
	}

	pictureInfo := &models.CreditCardPictureInfo{
		Uid:              uid,
		AccountId:        account.AccountId,
		PictureId:        pictureId,
		PictureExtension: fileExtension,
		CreatedUnixTime:  now,
		UpdatedUnixTime:  now,
	}

	err = s.SaveCreditCardPicture(c, uid, pictureId, storage.NewByteSliceObject(pictureData), fileExtension)

	if err != nil {
		return err
	}

	err = s.UserDataDB(uid).DoTransaction(c, func(sess datastore.DatabaseSession) error {
		_, err := sess.Insert(pictureInfo)
		return err
	})

	if err != nil {
		return err
	}

	account.Extend.CreditCardPicture = nil
	account.Extend.CreditCardPictureId = &pictureId

	return s.UserDataDB(uid).DoTransaction(c, func(sess datastore.DatabaseSession) error {
		account.UpdatedUnixTime = now
		_, err := sess.ID(account.AccountId).Cols("extend", "updated_unix_time").Where("uid=? AND deleted=?", uid, false).Update(account)
		return err
	})
}

func decodeCreditCardPictureDataUrl(dataUrl string) ([]byte, string, error) {
	if !strings.HasPrefix(dataUrl, creditCardPictureDataUrlPrefix) {
		return nil, "", errs.ErrCreditCardPictureInvalid
	}

	index := strings.Index(dataUrl[len(creditCardPictureDataUrlPrefix):], ";base64,")

	if index < 0 {
		return nil, "", errs.ErrCreditCardPictureInvalid
	}

	fileExtension := dataUrl[len(creditCardPictureDataUrlPrefix) : len(creditCardPictureDataUrlPrefix)+index]

	if fileExtension == "jpeg" {
		fileExtension = "jpg"
	}

	if utils.GetImageContentType(fileExtension) == "" {
		return nil, "", errs.ErrImageTypeNotSupported
	}

	base64Data := dataUrl[len(creditCardPictureDataUrlPrefix)+index+len(";base64,"):]
	pictureData, err := base64.StdEncoding.DecodeString(base64Data)

	if err != nil {
		return nil, "", errs.ErrCreditCardPictureInvalid
	}

	return pictureData, fileExtension, nil
}
