package apperror

import (
	"errors"
	"strings"

	"github.com/go-playground/validator/v10"
)

var (
	ErrInternalServer     = errors.New("INTERNAL_SERVER_ERROR")
	ErrDuplicatedKey      = errors.New("DUPLICATED_KEY")
	ErrDuplicatedEmail    = errors.New("EMAIL_ALREADY_EXISTS")
	ErrRecordNotFound     = errors.New("RECORD_NOT_FOUND")
	ErrUserNotFound       = errors.New("USER_NOT_FOUND")
	ErrInvalidCredentials = errors.New("INVALID_CREDENTIALS")
	ErrInvalidToken       = errors.New("INVALID_TOKEN")
	ErrExpiredToken       = errors.New("TOKEN_HAS_EXPIRED")
	ErrUnauthorized       = errors.New("UNAUTHORIZED")
	ErrInvalidID          = errors.New("INVALID_ID")
	ErrBadRequest         = errors.New("BAD_REQUEST")
)

func ExtractValidationErrors(err error) map[string]string {
	errorReport := make(map[string]string)

	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		errorReport = TranslateValidationError(validationErrors)
	}

	return errorReport
}

func TranslateValidationError(valErr validator.ValidationErrors) map[string]string {
	fieldError := make(map[string]string)

	for _, e := range valErr {
		var message string
		switch e.Tag() {
		case "required":
			message = "must be filled"
		case "email":
			message = "must be a valid email"
		case "min":
			message = "must be at least " + e.Param() + " characters long"
		case "max":
			message = "must be at most " + e.Param() + " characters long"
		default:
			message = "invalid input value"
		}
		fieldError[strings.ToLower(e.Field())] = message
	}

	return fieldError
}
