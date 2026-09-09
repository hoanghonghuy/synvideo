package ingestvalidation

import "errors"

func IsUnsupported(err error) bool    { return errors.Is(err, ErrUnsupported) }
func IsMalformed(err error) bool      { return errors.Is(err, ErrMalformed) }
func IsMismatch(err error) bool       { return errors.Is(err, ErrMismatch) }
func IsTimeout(err error) bool        { return errors.Is(err, ErrTimeout) }
func IsInfrastructure(err error) bool { return errors.Is(err, ErrInfrastructure) }
func IsTooLarge(err error) bool       { return errors.Is(err, ErrTooLarge) }
