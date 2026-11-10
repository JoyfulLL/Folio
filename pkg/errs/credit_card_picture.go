package errs

import "net/http"

// Error codes related to credit card pictures
var (
	ErrCreditCardPictureIdInvalid         = NewNormalError(NormalSubcategoryPicture, 7, http.StatusBadRequest, "credit card picture id is invalid")
	ErrCreditCardPictureNotFound          = NewNormalError(NormalSubcategoryPicture, 8, http.StatusBadRequest, "credit card picture not found")
	ErrNoCreditCardPicture                = NewNormalError(NormalSubcategoryPicture, 9, http.StatusBadRequest, "no credit card picture")
	ErrCreditCardPictureIsEmpty           = NewNormalError(NormalSubcategoryPicture, 10, http.StatusBadRequest, "credit card picture is empty")
	ErrCreditCardPictureNoExists          = NewNormalError(NormalSubcategoryPicture, 11, http.StatusNotFound, "credit card picture not exists")
	ErrCreditCardPictureExtensionInvalid  = NewNormalError(NormalSubcategoryPicture, 12, http.StatusNotFound, "credit card picture file extension invalid")
	ErrExceedMaxCreditCardPictureFileSize = NewNormalError(NormalSubcategoryPicture, 13, http.StatusBadRequest, "exceed the maximum size of credit card picture file")
)
