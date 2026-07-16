package apperror

import (
	"errors"
	"strings"

	"github.com/go-playground/validator/v10"
)

var (
	// DB
	ErrInternalServer      = errors.New("INTERNAL_SERVER_ERROR")
	ErrDuplicatedKey       = errors.New("DUPLICATED_KEY")
	ErrRecordNotFound      = errors.New("RECORD_NOT_FOUND")
	ErrInvalidID           = errors.New("INVALID_ID")
	ErrBadRequest          = errors.New("BAD_REQUEST")
	ErrNoActiveTransaction = errors.New("NO_ACTIVE_TRANSACTION")

	// Auth
	ErrDuplicatedEmail    = errors.New("EMAIL_ALREADY_EXISTS")
	ErrUserNotFound       = errors.New("USER_NOT_FOUND")
	ErrInvalidCredentials = errors.New("INVALID_CREDENTIALS")
	ErrInvalidToken       = errors.New("INVALID_TOKEN")
	ErrExpiredToken       = errors.New("TOKEN_HAS_EXPIRED")
	ErrUnauthorized       = errors.New("UNAUTHORIZED")

	// Wallet
	ErrWalletNotFound         = errors.New("WALLET_NOT_FOUND")
	ErrInsufficientBalance    = errors.New("INSUFFICIENT_BALANCE")
	ErrInvalidAmount          = errors.New("INVALID_AMOUNT")
	ErrSelfTransferNotAllowed = errors.New("SELF_TRANSFER_NOT_ALLOWED")
	ErrRecipientNotFound      = errors.New("RECIPIENT_NOT_FOUND")
	ErrUserHasWalletAlready   = errors.New("USER_HAS_WALLET_ALREADY")

	// Transaction
	ErrTransactionNotFound     = errors.New("TRANSACTION_NOT_FOUND")
	ErrTransactionAccessDenied = errors.New("TRANSACTION_ACCESS_DENIED") // GET /transactions/:id milik user lain

	// Idempotency
	ErrMissingIdempotencyKey  = errors.New("MISSING_IDEMPOTENCY_KEY")
	ErrIdempotencyKeyConflict = errors.New("IDEMPOTENCY_KEY_CONFLICT")
	ErrRequestInProgress      = errors.New("REQUEST_IN_PROGRESS")
	ErrPreviousAttemptFailed  = errors.New("PREVIOUS_ATTEMPT_FAILED")
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
