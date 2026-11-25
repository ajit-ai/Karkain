#ifndef KARKAIN_TLS_H
#define KARKAIN_TLS_H

// v1.2.17: C23-only transport TLS. Windows uses WinHTTP (Schannel underneath),
// POSIX uses OpenSSL. The generated preamble defines Value/make_string and the
// net runtime before this header's bodies are appended, so both are in scope.
#ifdef _WIN32
#include <windows.h>
#include <winhttp.h>
#else
#include <openssl/ssl.h>
#include <openssl/err.h>
#include <openssl/x509v3.h>
#endif

Value karkain_https_request(Value method, Value host, Value port, Value path, Value body, Value verify);
Value karkain_tls_last_error(void);

#endif
