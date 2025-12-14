# TODO:

# why did we add SupportsMaxMessageSize?

`SupportsMaxMessageSize` shouldn't be necessary. Shouldn't we enforce that in each transport? The

# Why was the concurrent test failing?

Please re-evaluate adding

```
if t.closed {
	return nil // Already closed
}
```

to the ServerTransport Close method. Why would the trans

# Test cleanup prompt:

Why does the golang test suite have a single implementation for the client and server and a single test file per transport while the C++ one has a separate suite for client and server and for each transport type per server?

Is it possible to implement those tests the same way the golang tests work, where the test suite runs the client / server in separate threads, rather than as separate processes?

Ideally the C++ and Go testing suites should be as similar as possible.

# Generic test suites for go / C++ that we can use for each transport impl.

# limit what values c++ can serialize into context

-   ex. int, can't directly deserialize that on the go client.

# C++ `scg` namespaces and naming

Should we drop the intermediate namespaces: `scg::rpc`, `scg::error`, `scg::serialize`, `scg::context`

Option A)

-   three namespaces:
    -   `scg::type::Error`, `scg::type::UUID`, `scg::Type::Timestamp`
    -   `scg::rpc::Context`, `scg::rpc::ClientTLS`
    -   `scg::serialize::Writer`

Option B)

-   rename everything with just the root package:
    -   `scg::ClientTLS`
    -   `scg::ClientNoTLS`
    -   `scg::Error` -` scg::Context`
    -   `scg::UUID`
    -   `scg::Timestamp`
    -   `scg::Reader`
    -   `scg::Writer`

# `Declaration` vs `Definition`, pick one

# Split the parsing code from the actual Definitions

ex `def.Message`, `def.Typedef`

```

```
