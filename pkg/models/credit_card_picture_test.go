package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreditCardPictureInfo_ToCreditCardPictureInfoBasicResponse_WithToken(t *testing.T) {
	pictureInfo := &CreditCardPictureInfo{
		PictureId: 123,
	}

	expectedValue := &CreditCardPictureInfoBasicResponse{
		PictureId:   123,
		OriginalUrl: "https://foo.bar/credit_card_pictures/123.png?token=media%2Btoken",
	}
	actualValue := pictureInfo.ToCreditCardPictureInfoBasicResponse("https://foo.bar/credit_card_pictures/123.png", "media+token")
	assert.Equal(t, expectedValue, actualValue)
}

func TestCreditCardPictureInfo_ToCreditCardPictureInfoBasicResponse_WithTokenAndQueryString(t *testing.T) {
	pictureInfo := &CreditCardPictureInfo{
		PictureId: 123,
	}

	expectedValue := &CreditCardPictureInfoBasicResponse{
		PictureId:   123,
		OriginalUrl: "https://foo.bar/credit_card_pictures/123.png?size=64&token=media%2Btoken",
	}
	actualValue := pictureInfo.ToCreditCardPictureInfoBasicResponse("https://foo.bar/credit_card_pictures/123.png?size=64", "media+token")
	assert.Equal(t, expectedValue, actualValue)
}

func TestCreditCardPictureInfo_ToCreditCardPictureInfoBasicResponse_WithoutTokenAndWithQueryString(t *testing.T) {
	pictureInfo := &CreditCardPictureInfo{
		PictureId: 123,
	}

	expectedValue := &CreditCardPictureInfoBasicResponse{
		PictureId:   123,
		OriginalUrl: "https://foo.bar/credit_card_pictures/123.png?size=64",
	}
	actualValue := pictureInfo.ToCreditCardPictureInfoBasicResponse("https://foo.bar/credit_card_pictures/123.png?size=64", "")
	assert.Equal(t, expectedValue, actualValue)
}
