package validators

import (
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
)

type creditCardNotificationIntervalContainer struct {
	Interval int16 `validate:"validCreditCardNotificationInterval"`
}

func TestValidateCreditCardNotificationInterval_ValidValues(t *testing.T) {
	validate := validator.New()
	err := validate.RegisterValidation("validCreditCardNotificationInterval", ValidateCreditCardNotificationInterval)
	assert.Nil(t, err)

	for _, interval := range []int16{15, 30, 60, 180, 360, 720} {
		container := creditCardNotificationIntervalContainer{Interval: interval}
		assert.Nil(t, validate.Struct(container))
	}
}

func TestValidateCreditCardNotificationInterval_InvalidValues(t *testing.T) {
	validate := validator.New()
	err := validate.RegisterValidation("validCreditCardNotificationInterval", ValidateCreditCardNotificationInterval)
	assert.Nil(t, err)

	for _, interval := range []int16{0, 1, 14, 45, 90, 721, 1440} {
		container := creditCardNotificationIntervalContainer{Interval: interval}
		assert.NotNil(t, validate.Struct(container))
	}
}
