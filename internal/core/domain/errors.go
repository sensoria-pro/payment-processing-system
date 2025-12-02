package domain

import "errors"

var (
	ErrInvalidAmount         = errors.New("amount must be positive")
	ErrInvalidCard           = errors.New("invalid card number")
	ErrIdempotencyKeyUsed    = errors.New("idempotency key already used")
	ErrBrokerUnavailable     = errors.New("kafka broker is unavailable")
	ErrStorageUnavailable    = errors.New("database is unavailable")
)