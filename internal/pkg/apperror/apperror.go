package apperror

import "errors"

var (
	ErrInternalServer     = errors.New("INTERNAL_SERVER_ERROR")
	ErrDuplicatedKey      = errors.New("DUPLICATED_KEY")
	ErrRecordNotFound     = errors.New("RECORD_NOT_FOUND")
	ErrUserNotFound       = errors.New("USER_NOT_FOUND")
	ErrDuplicatedEmail    = errors.New("EMAIL_ALREADY_EXISTS")
	ErrInvalidCredentials = errors.New("INVALID_CREDENTIALS")
	ErrInvalidToken       = errors.New("INVALID_TOKEN")
	ErrExpiredToken       = errors.New("TOKEN_HAS_EXPIRED")
	ErrUnauthorized       = errors.New("UNAUTHORIZED")
)
