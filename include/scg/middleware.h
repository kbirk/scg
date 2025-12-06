#pragma once

#include <functional>

#include "scg/error.h"
#include "scg/context.h"
#include "scg/message.h"

#include <cstdint>

namespace scg {
namespace middleware {

using Handler = std::function<std::pair<scg::type::Message*, scg::error::Error>(scg::context::Context&, const scg::type::Message&)>;
using Middleware = std::function<std::pair<scg::type::Message*, scg::error::Error>(scg::context::Context&, const scg::type::Message&, Handler)>;

inline Handler buildHandlerFunction(const std::vector<Middleware>& middleware, Handler final) {
	// start with the final handler
	Handler chain = final;

	// loop backwards through the middleware vector
	for (auto it = middleware.rbegin(); it != middleware.rend(); ++it) {
		// capture the current middleware handler
		Middleware m = *it;

		// wrap the current chain with the current middleware
		Handler next = chain;
		chain = [m, next](scg::context::Context& ctx, const scg::type::Message& req) -> std::pair<scg::type::Message*, scg::error::Error> {
			return m(ctx, req, next);
		};
	}

	// return the fully chained handler
	return chain;
}

inline std::pair<scg::type::Message*, scg::error::Error> applyHandlerChain(scg::context::Context& ctx, const scg::type::Message& req, const std::vector<Middleware>& middleware, Handler final) {
	Handler fn = buildHandlerFunction(middleware, final);
	return fn(ctx, req);
}

}
}
