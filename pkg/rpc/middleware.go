package rpc

import (
	"context"
)

type Handler func(context.Context, Message) (Message, error)
type Middleware func(context.Context, Message, Handler) (Message, error)

func buildHandlerFunction(middleware []Middleware, final Handler) Handler {

	// apply middleware from parent down

	// start with the final handler
	chain := final

	// loop backwards through the middleware slice
	for i := len(middleware) - 1; i >= 0; i-- {
		// capture the current middleware handler
		m := middleware[i]

		// wrap the current chain with the current middleware
		next := chain
		chain = func(ctx context.Context, req Message) (Message, error) {
			return m(ctx, req, next)
		}
	}

	// return the fully chained handler
	return chain
}

func ApplyHandlerChain(ctx context.Context, req Message, middleware []Middleware, final Handler) (Message, error) {
	fn := buildHandlerFunction(middleware, final)
	return fn(ctx, req)
}
