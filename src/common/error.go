package common

import "errors"

var ErrPersistenceNotInitialized = errors.New("persistence not initialized")
var ErrNoRoutesPersisted = errors.New("routes.json does not exist yet")
var ErrPersistenceChecksumMismatch = errors.New("config checksum mismatch")
