#pragma once

// Test-only: generates a throwaway self-signed "localhost" certificate and key
// in-process (OpenSSL API), so no private key is checked in and no openssl CLI
// is needed. The certificate has CN=localhost and a single SAN, DNS:localhost
// (no IP SAN), so connecting to "127.0.0.1" with verification enabled must fail
// host-name verification while "localhost" succeeds. It is its own CA.

#include <openssl/bio.h>
#include <openssl/evp.h>
#include <openssl/ec.h>
#include <openssl/pem.h>
#include <openssl/x509.h>
#include <openssl/x509v3.h>

#include <cstdio>
#include <filesystem>
#include <memory>
#include <stdexcept>
#include <string>
#include <system_error>

#if defined(_WIN32)
#include <process.h>
#define SCG_TEST_GETPID _getpid
#else
#include <unistd.h>
#define SCG_TEST_GETPID getpid
#endif

namespace testcerts {

struct LocalhostCert {
	std::string dir;
	std::string certFile;
	std::string keyFile;
};

namespace detail {

inline void check(bool ok, const char* what)
{
	if (!ok) {
		throw std::runtime_error(std::string("test certificate generation failed: ") + what);
	}
}

inline void writeFiles(const std::string& certPath, const std::string& keyPath)
{
	// P-256 key: fast to generate, accepted by every TLS 1.2+ stack.
	std::unique_ptr<EVP_PKEY_CTX, decltype(&EVP_PKEY_CTX_free)> kctx(EVP_PKEY_CTX_new_id(EVP_PKEY_EC, nullptr), EVP_PKEY_CTX_free);
	check(kctx != nullptr, "EVP_PKEY_CTX_new_id");
	check(EVP_PKEY_keygen_init(kctx.get()) == 1, "EVP_PKEY_keygen_init");
	check(EVP_PKEY_CTX_set_ec_paramgen_curve_nid(kctx.get(), NID_X9_62_prime256v1) == 1, "set curve");
	EVP_PKEY* rawKey = nullptr;
	check(EVP_PKEY_keygen(kctx.get(), &rawKey) == 1, "EVP_PKEY_keygen");
	std::unique_ptr<EVP_PKEY, decltype(&EVP_PKEY_free)> key(rawKey, EVP_PKEY_free);

	std::unique_ptr<X509, decltype(&X509_free)> cert(X509_new(), X509_free);
	check(cert != nullptr, "X509_new");
	check(X509_set_version(cert.get(), 2) == 1, "X509_set_version"); // v3
	check(ASN1_INTEGER_set(X509_get_serialNumber(cert.get()), 1) == 1, "serial");
	check(X509_gmtime_adj(X509_getm_notBefore(cert.get()), -3600) != nullptr, "notBefore");
	check(X509_gmtime_adj(X509_getm_notAfter(cert.get()), 60L * 60 * 24 * 365) != nullptr, "notAfter");
	check(X509_set_pubkey(cert.get(), key.get()) == 1, "X509_set_pubkey");

	X509_NAME* name = X509_get_subject_name(cert.get());
	check(X509_NAME_add_entry_by_txt(name, "CN", MBSTRING_ASC, reinterpret_cast<const unsigned char*>("localhost"), -1, -1, 0) == 1, "CN");
	check(X509_set_issuer_name(cert.get(), name) == 1, "issuer");

	X509V3_CTX v3;
	X509V3_set_ctx_nodb(&v3);
	X509V3_set_ctx(&v3, cert.get(), cert.get(), nullptr, nullptr, 0);
	const struct {
		int nid;
		const char* value;
	} exts[] = {
		{NID_subject_alt_name, "DNS:localhost"},
		{NID_basic_constraints, "critical,CA:TRUE"},
		{NID_key_usage, "critical,digitalSignature,keyCertSign"},
	};
	for (const auto& e : exts) {
		X509_EXTENSION* ext = X509V3_EXT_conf_nid(nullptr, &v3, e.nid, e.value);
		check(ext != nullptr, "X509V3_EXT_conf_nid");
		int ok = X509_add_ext(cert.get(), ext, -1);
		X509_EXTENSION_free(ext);
		check(ok == 1, "X509_add_ext");
	}
	check(X509_sign(cert.get(), key.get(), EVP_sha256()) > 0, "X509_sign");

	// BIOs rather than FILE*: OpenSSL DLL builds on Windows abort when handed a
	// FILE* from a different C runtime.
	std::unique_ptr<BIO, decltype(&BIO_free)> certOut(BIO_new_file(certPath.c_str(), "wb"), BIO_free);
	check(certOut != nullptr, "open cert file");
	check(PEM_write_bio_X509(certOut.get(), cert.get()) == 1, "PEM_write_bio_X509");

	std::unique_ptr<BIO, decltype(&BIO_free)> keyOut(BIO_new_file(keyPath.c_str(), "wb"), BIO_free);
	check(keyOut != nullptr, "open key file");
	check(PEM_write_bio_PrivateKey(keyOut.get(), key.get(), nullptr, nullptr, 0, nullptr, nullptr) == 1, "PEM_write_bio_PrivateKey");
}

// Owner generates the files on construction and removes them (with their
// directory) when the process exits. Not copyable, so no copy can delete them
// early.
struct Owner {
	LocalhostCert cert;

	Owner()
	{
		// A directory this process creates itself, never one that already
		// exists (which could be stale, or another user's on a shared host).
		const auto base = std::filesystem::temp_directory_path();
		const std::string prefix = "scg-test-certs-" + std::to_string(SCG_TEST_GETPID()) + "-";
		std::filesystem::path dir;
		for (int attempt = 0; dir.empty(); ++attempt) {
			check(attempt < 100, "create a temporary directory");
			auto candidate = base / (prefix + std::to_string(attempt));
			std::error_code ec;
			if (std::filesystem::create_directory(candidate, ec)) {
				dir = candidate;
			}
		}
		cert.dir = dir.string();
		cert.certFile = (dir / "localhost.crt").string();
		cert.keyFile = (dir / "localhost.key").string();
		try {
			writeFiles(cert.certFile, cert.keyFile);
		} catch (...) {
			std::error_code ec;
			std::filesystem::remove_all(dir, ec);
			throw;
		}
	}

	Owner(const Owner&) = delete;
	Owner& operator=(const Owner&) = delete;

	~Owner()
	{
		std::error_code ec;
		std::filesystem::remove_all(cert.dir, ec);
	}
};

} // namespace detail

// localhostCert returns the paths of a certificate/key pair generated on first
// use in a per-process temporary directory (removed at exit).
inline const LocalhostCert& localhostCert()
{
	static detail::Owner owner;
	return owner.cert;
}

} // namespace testcerts
