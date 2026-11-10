package models

import (
	"net/url"
	"strings"
)

const CreditCardPictureNewPictureAccountId = int64(0)

// CreditCardPictureInfo represents credit card picture file info stored in database
type CreditCardPictureInfo struct {
	Uid              int64  `xorm:"INDEX(IDX_credit_card_picture_uid_deleted_account_id_picture_id) INDEX(IDX_credit_card_picture_uid_deleted_picture_id) NOT NULL"`
	Deleted          bool   `xorm:"INDEX(IDX_credit_card_picture_uid_deleted_account_id_picture_id) INDEX(IDX_credit_card_picture_uid_deleted_picture_id) NOT NULL"`
	AccountId        int64  `xorm:"INDEX(IDX_credit_card_picture_uid_deleted_account_id_picture_id) NOT NULL"`
	PictureId        int64  `xorm:"PK INDEX(IDX_credit_card_picture_uid_deleted_account_id_picture_id) INDEX(IDX_credit_card_picture_uid_deleted_picture_id)"`
	PictureExtension string `xorm:"VARCHAR(10) NOT NULL"`
	CreatedIp        string `xorm:"VARCHAR(39)"`
	CreatedUnixTime  int64
	UpdatedUnixTime  int64
	DeletedUnixTime  int64
}

// CreditCardPictureUnusedDeleteRequest represents all parameters of unused credit card picture deleting request
type CreditCardPictureUnusedDeleteRequest struct {
	Id int64 `json:"id,string" binding:"required,min=1"`
}

// CreditCardPictureInfoBasicResponse represents a view-object of credit card picture basic info
type CreditCardPictureInfoBasicResponse struct {
	PictureId   int64  `json:"pictureId,string"`
	OriginalUrl string `json:"originalUrl"`
}

// ToCreditCardPictureInfoBasicResponse returns a view-object according to database model
func (p *CreditCardPictureInfo) ToCreditCardPictureInfoBasicResponse(originalUrl string, pictureToken string) *CreditCardPictureInfoBasicResponse {
	return &CreditCardPictureInfoBasicResponse{
		PictureId:   p.PictureId,
		OriginalUrl: AppendCreditCardPictureTokenToUrl(originalUrl, pictureToken),
	}
}

// AppendCreditCardPictureTokenToUrl returns a picture url with the specified access token appended
func AppendCreditCardPictureTokenToUrl(originalUrl string, pictureToken string) string {
	if pictureToken != "" {
		separator := "?"

		if strings.Contains(originalUrl, "?") {
			separator = "&"
		}

		return originalUrl + separator + "token=" + url.QueryEscape(pictureToken)
	}

	return originalUrl
}
