package apperror

import (
	"net/http"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

type AppError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
	Status  int               `json:"-"` // internal, bukan buat ikut ke response body
}

func (e *AppError) Error() string { return e.Message }

var (
	// DB
	ErrInternalServer      = &AppError{Code: "INTERNAL_SERVER_ERROR", Message: "something went wrong", Status: http.StatusInternalServerError}
	ErrDuplicatedKey       = &AppError{Code: "DUPLICATED_KEY", Message: "duplicated key", Status: http.StatusConflict}
	ErrRecordNotFound      = &AppError{Code: "RECORD_NOT_FOUND", Message: "record not found", Status: http.StatusNotFound}
	ErrInvalidID           = &AppError{Code: "INVALID_ID", Message: "invalid id", Status: http.StatusBadRequest}
	ErrBadRequest          = &AppError{Code: "BAD_REQUEST", Message: "bad request", Status: http.StatusBadRequest}
	ErrNoActiveTransaction = &AppError{Code: "NO_ACTIVE_TRANSACTION", Message: "no active transaction", Status: http.StatusInternalServerError}

	// Auth
	ErrDuplicatedEmail    = &AppError{Code: "EMAIL_ALREADY_EXISTS", Message: "email already exists", Status: http.StatusConflict}
	ErrUserNotFound       = &AppError{Code: "USER_NOT_FOUND", Message: "user not found", Status: http.StatusNotFound}
	ErrInvalidCredentials = &AppError{Code: "INVALID_CREDENTIALS", Message: "wrong email or password", Status: http.StatusUnauthorized}
	ErrInvalidToken       = &AppError{Code: "INVALID_TOKEN", Message: "invalid token", Status: http.StatusUnauthorized}
	ErrExpiredToken       = &AppError{Code: "TOKEN_HAS_EXPIRED", Message: "token has expired", Status: http.StatusUnauthorized}
	ErrUnauthorized       = &AppError{Code: "UNAUTHORIZED", Message: "unauthorized", Status: http.StatusUnauthorized}

	// Wallet
	ErrWalletNotFound         = &AppError{Code: "WALLET_NOT_FOUND", Message: "wallet not found", Status: http.StatusNotFound}
	ErrInsufficientBalance    = &AppError{Code: "INSUFFICIENT_BALANCE", Message: "insufficient balance", Status: http.StatusUnprocessableEntity}
	ErrInvalidAmount          = &AppError{Code: "INVALID_AMOUNT", Message: "amount must be greater than zero", Status: http.StatusBadRequest}
	ErrSelfTransferNotAllowed = &AppError{Code: "SELF_TRANSFER_NOT_ALLOWED", Message: "cannot transfer to your own wallet", Status: http.StatusUnprocessableEntity}
	ErrRecipientNotFound      = &AppError{Code: "RECIPIENT_NOT_FOUND", Message: "recipient not found", Status: http.StatusNotFound}
	ErrUserHasWalletAlready   = &AppError{Code: "USER_HAS_WALLET_ALREADY", Message: "user already has a wallet", Status: http.StatusUnprocessableEntity}

	// Transaction
	ErrTransactionNotFound     = &AppError{Code: "TRANSACTION_NOT_FOUND", Message: "transaction not found", Status: http.StatusNotFound}
	ErrTransactionAccessDenied = &AppError{Code: "TRANSACTION_ACCESS_DENIED", Message: "transaction not found", Status: http.StatusNotFound}

	// Idempotency
	ErrMissingIdempotencyKey  = &AppError{Code: "MISSING_IDEMPOTENCY_KEY", Message: "missing idempotency key", Status: http.StatusBadRequest}
	ErrIdempotencyKeyConflict = &AppError{Code: "IDEMPOTENCY_KEY_CONFLICT", Message: "idempotency key conflict", Status: http.StatusConflict}
	ErrRequestInProgress      = &AppError{Code: "REQUEST_IN_PROGRESS", Message: "request in progress", Status: http.StatusConflict}
	ErrPreviousAttemptFailed  = &AppError{Code: "PREVIOUS_ATTEMPT_FAILED", Message: "previous attempt failed", Status: http.StatusUnprocessableEntity}
)

func ExtractValidationErrors(err error) *AppError {
	errorReport := make(map[string]string)

	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		errorReport = TranslateValidationError(validationErrors)
	}

	return &AppError{
		Code:    "VALIDATION_ERROR",
		Message: "one or more fields are invalid",
		Fields:  errorReport,
		Status:  http.StatusBadRequest,
	}
}

func TranslateValidationError(valErr validator.ValidationErrors) map[string]string {
	fieldError := make(map[string]string)

	for _, e := range valErr {
		fieldError[strings.ToLower(e.Field())] = translateTag(e)
	}

	return fieldError
}

func translateTag(e validator.FieldError) string {
	switch e.Tag() {
	case "required":
		return "must be filled"
	case "email":
		return "must be a valid email"
	case "min":
		if isNumericKind(e.Kind()) {
			return "must be at least " + e.Param()
		}
		return "must be at least " + e.Param() + " characters long"
	case "max":
		if isNumericKind(e.Kind()) {
			return "must be at most " + e.Param()
		}
		return "must be at most " + e.Param() + " characters long"
	case "gt":
		return "must be greater than " + e.Param()
	case "gte":
		return "must be greater than or equal to " + e.Param()
	case "lt":
		return "must be less than " + e.Param()
	case "lte":
		return "must be less than or equal to " + e.Param()
	case "oneof":
		return "must be one of: " + e.Param()
	case "datetime":
		return "must be a valid date in format " + e.Param()
	case "ltefield":
		return "must be less than or equal to " + strings.ToLower(e.Param())
	case "gtefield":
		return "must be greater than or equal to " + strings.ToLower(e.Param())
	default:
		return "invalid input value"
	}
}

func isNumericKind(k reflect.Kind) bool {
	switch k {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	}
	return false
}
