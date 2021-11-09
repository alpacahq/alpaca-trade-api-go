package stream

import "errors"

// ErrConnectCalledMultipleTimes is returned when Connect has been called multiple times on a single client
var ErrConnectCalledMultipleTimes = errors.New("tried to call Connect multiple times")

// ErrNoConnected is returned when the client did not receive the welcome
// message from the server
var ErrNoConnected = errors.New("did not receive connected message")

//ErrBadAuthResponse is returned when the client could not successfully authenticate
var ErrBadAuthResponse = errors.New("did not receive authenticated message")

// ErrSubResponse is returned when the client's subscription request was not
// acknowledged
var ErrSubResponse = errors.New("did not receive subscribed message")

// ErrSubscriptionChangeBeforeConnect is returned when the client attempts to change subscriptions before
// calling Connect
var ErrSubscriptionChangeBeforeConnect = errors.New("subscription change attempted before calling Connect")

// ErrSubscriptionChangeAfterTerminated is returned when client attempts to change subscriptions after
// the client has been terminated
var ErrSubscriptionChangeAfterTerminated = errors.New("subscription change after client termination")

// ErrSubscriptionChangeAlreadyInProgress is returned when a subscription change is called concurrently
// with another
var ErrSubscriptionChangeAlreadyInProgress = errors.New("subscription change already in progress")

// ErrSubscriptionChangeInterrupted is returned when a subscription change was in progress when the client
// has terminated
var ErrSubscriptionChangeInterrupted = errors.New("subscription change interrupted by client termination")

// ErrSubscriptionChangeTimeout is returned when the server does not return a proper
// subscription response after a subscription change request.
var ErrSubscriptionChangeTimeout = errors.New("subscription change timeout")

// The following errors are returned when the client receives an error message from the server

// ErrInvalidCredentials is returned when invalid credentials have been sent by the user.
var ErrInvalidCredentials error = errorMessage{msg: "auth failed", code: 402}

// ErrSymbolLimitExceeded is returned when the client has subscribed to too many symbols
var ErrSymbolLimitExceeded error = errorMessage{msg: "symbol limit exceeded", code: 405}

// ErrConnectionLimitExceeded is returned when the client has exceeded their connection limit
var ErrConnectionLimitExceeded error = errorMessage{msg: "connection limit exceeded", code: 406}

// ErrSlowClient is returned when the server has detected a slow client. In this case there's no guarantee
// that all prior messages are sent to the server so a subscription acknowledgement may not arrive
var ErrSlowClient error = errorMessage{msg: "slow client", code: 407}

// ErrInsufficientSubscription is returned when the user does not have proper
// subscription for the requested feed (e.g. SIP)
var ErrInsufficientSubscription error = errorMessage{msg: "insufficient subscription", code: 409}

// ErrSubscriptionChangeTimeout is returned when a subscription change is invalid for the feed.
var ErrSubscriptionChangeInvalidForFeed error = errorMessage{msg: "invalid subscribe action for this feed", code: 410}

// ErrInsufficientScope is returned when the token used by the user doesn't have proper scopes
// for data stream
var ErrInsufficientScope error = errorMessage{msg: "insufficient scope", code: 411}
