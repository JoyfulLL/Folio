package validators

import (
	"github.com/go-playground/validator/v10"

	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// ValidateCreditCardNotificationInterval validates if a credit card notification scan interval in
// minutes is one of the values the user can choose
func ValidateCreditCardNotificationInterval(fl validator.FieldLevel) bool {
	interval, ok := fl.Field().Interface().(int16)
	if !ok {
		return false
	}

	return models.IsValidCreditCardNotificationInterval(interval)
}
