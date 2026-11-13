package subscription

import "errors"

var (
	ErrInvalidChannelReference = errors.New("invalid channel reference")
	ErrChannelResolution       = errors.New("channel resolution failed")
	ErrChannelFeedUnavailable  = errors.New("channel feed unavailable")
	ErrVideoTypesRequired      = errors.New("at least one video type must be enabled")
)
