#pragma once

#include <functional>
#include <memory>
#include <vector>

#include "scg/error.h"
#include "scg/context.h"
#include "scg/message.h"

#include <cstdint>

namespace scg {
namespace middleware {

using Handler = std::function<std::pair<std::shared_ptr<scg::type::Message>, scg::error::Error>(scg::context::Context&, const scg::type::Message&)>;
using Middleware = std::function<std::pair<std::shared_ptr<scg::type::Message>, scg::error::Error>(scg::context::Context&, const scg::type::Message&, Handler)>;

inline Handler buildHandlerFunction(const std::vector<Middleware>& middleware, Handler final) {
	// start with the final handler
	Handler chain = final;

	// loop backwards through the middleware vector
	for (auto it = middleware.rbegin(); it != middleware.rend(); ++it) {
		// capture the current middleware handler
		Middleware m = *it;

		// wrap the current chain with the current middleware
		Handler next = chain;
		chain = [m, next](scg::context::Context& ctx, const scg::type::Message& req) -> std::pair<std::shared_ptr<scg::type::Message>, scg::error::Error> {
			return m(ctx, req, next);
		};
	}

	// return the fully chained handler
	return chain;
}

namespace detail {

// ChainState walks the middleware vector by index. Each `next` handler handed to
// a middleware captures only a pointer and an index (fits std::function's
// small-buffer storage), so running the chain does not copy the middleware
// functions or heap-allocate per layer.
struct ChainState {
	const std::vector<Middleware>& middleware;
	const Handler& final;

	std::pair<std::shared_ptr<scg::type::Message>, scg::error::Error> run(size_t index, scg::context::Context& ctx, const scg::type::Message& req) const
	{
		if (index >= middleware.size()) {
			return final(ctx, req);
		}
		const ChainState* self = this;
		Handler next = [self, index](scg::context::Context& c, const scg::type::Message& r) {
			return self->run(index + 1, c, r);
		};
		return middleware[index](ctx, req, next);
	}
};

}

inline std::pair<std::shared_ptr<scg::type::Message>, scg::error::Error> applyHandlerChain(scg::context::Context& ctx, const scg::type::Message& req, const std::vector<Middleware>& middleware, const Handler& final) {
	if (middleware.empty()) {
		return final(ctx, req);
	}
	detail::ChainState state{middleware, final};
	return state.run(0, ctx, req);
}

}
}
