package http

import "time"

// AuthTimeout bounds one authentication request, its retries and rate limit waits included.
const AuthTimeout = 30 * time.Second
