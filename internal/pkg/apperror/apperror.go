package apperror

import "errors"

var (
	ErrDuplicatedKey      = errors.New("ERROR_DUPLICATED_KEY")
	ErrRecordNotFound     = errors.New("ERROR_RECORD_NOT_FOUND")
	ErrDuplicatedEmail    = errors.New("EMAIL_ALREADY_EXISTS")
	ErrInvalidCredentials = errors.New("INVALID_CREDENTIALS")
)
