package services

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/errs"
)

func TestDecodeCreditCardPictureDataUrl_Png(t *testing.T) {
	pictureData := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	dataUrl := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pictureData)

	actualData, fileExtension, err := decodeCreditCardPictureDataUrl(dataUrl)

	assert.Nil(t, err)
	assert.Equal(t, pictureData, actualData)
	assert.Equal(t, "png", fileExtension)
}

func TestDecodeCreditCardPictureDataUrl_JpegUsesJpgExtension(t *testing.T) {
	pictureData := []byte{0xff, 0xd8, 0xff, 0xe0}
	dataUrl := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(pictureData)

	actualData, fileExtension, err := decodeCreditCardPictureDataUrl(dataUrl)

	assert.Nil(t, err)
	assert.Equal(t, pictureData, actualData)
	assert.Equal(t, "jpg", fileExtension)
}

func TestDecodeCreditCardPictureDataUrl_WithoutDataUrlPrefix(t *testing.T) {
	actualData, fileExtension, err := decodeCreditCardPictureDataUrl("iVBORw0KGgo=")

	assert.Equal(t, errs.ErrCreditCardPictureInvalid, err)
	assert.Nil(t, actualData)
	assert.Equal(t, "", fileExtension)
}

func TestDecodeCreditCardPictureDataUrl_WithoutBase64Separator(t *testing.T) {
	actualData, fileExtension, err := decodeCreditCardPictureDataUrl("data:image/png,iVBORw0KGgo=")

	assert.Equal(t, errs.ErrCreditCardPictureInvalid, err)
	assert.Nil(t, actualData)
	assert.Equal(t, "", fileExtension)
}

func TestDecodeCreditCardPictureDataUrl_UnsupportedImageType(t *testing.T) {
	actualData, fileExtension, err := decodeCreditCardPictureDataUrl("data:image/bmp;base64,AAAA")

	assert.Equal(t, errs.ErrImageTypeNotSupported, err)
	assert.Nil(t, actualData)
	assert.Equal(t, "", fileExtension)
}

func TestDecodeCreditCardPictureDataUrl_InvalidBase64Data(t *testing.T) {
	actualData, fileExtension, err := decodeCreditCardPictureDataUrl("data:image/png;base64,@@@@")

	assert.Equal(t, errs.ErrCreditCardPictureInvalid, err)
	assert.Nil(t, actualData)
	assert.Equal(t, "", fileExtension)
}
