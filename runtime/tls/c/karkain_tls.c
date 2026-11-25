#include "karkain_tls.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

// The public net runtime publishes karkain_net_connect / karkain_net_error;
// this translation-unit-shaped block is appended after it, so those symbols
// are already defined. TLS keeps its own last-error buffer so a handshake
// failure is distinguishable from a plain TCP failure.
static char _karkain_tls_error[512] = "";

static void karkain_tls_err(const char* msg) {
    snprintf(_karkain_tls_error, sizeof(_karkain_tls_error), "%s", msg ? msg : "tls failure");
}

Value karkain_tls_last_error(void) {
    return make_string(_karkain_tls_error);
}

static int karkain_tls_url(char* out, size_t cap, const char* host, int port, const char* path) {
    if (!host || !path || port < 1 || port > 65535) return -1;
    int n = snprintf(out, cap, "https://%s:%d%s", host, port, path);
    if (n < 0 || (size_t)n >= cap) return -1;
    return 0;
}

Value karkain_https_request(Value method, Value host, Value port, Value path, Value body, Value verify) {
    _karkain_tls_error[0] = '\0';
    if (method.type != TYPE_STRING || host.type != TYPE_STRING ||
        port.type != TYPE_INT || path.type != TYPE_STRING ||
        body.type != TYPE_STRING || verify.type != TYPE_INT) {
        karkain_tls_err("https: invalid argument");
        return make_string("");
    }
    const char* m  = method.strVal ? method.strVal : "GET";
    const char* h  = host.strVal ? host.strVal : "";
    const char* p  = path.strVal ? path.strVal : "/";
    const char* bd = body.strVal ? body.strVal : "";
    int pn = (int)port.intVal;
    int do_verify = verify.intVal != 0 ? 1 : 0;
    if (pn < 1 || pn > 65535) {
        karkain_tls_err("https: invalid address");
        return make_string("");
    }

    char req[8192];
    int rn = snprintf(req, sizeof(req),
        "%s %s HTTP/1.1\r\nHost: %s:%d\r\nUser-Agent: karkain/1.2.17\r\n"
        "Accept: */*\r\nConnection: close\r\nContent-Length: %zu\r\n\r\n%s",
        m, p, h, pn, strlen(bd), bd);
    if (rn < 0 || (size_t)rn >= sizeof(req)) {
        karkain_tls_err("https: request too large");
        return make_string("");
    }

    size_t cap = 262144;
    size_t used = 0;
    char* out = (char*)malloc(cap + 1);
    if (!out) {
        karkain_tls_err("https: out of memory");
        return make_string("");
    }
    char url[4096];
    if (karkain_tls_url(url, sizeof(url), h, pn, p) != 0) {
        karkain_tls_err("https: invalid address");
        free(out);
        return make_string("");
    }

#ifdef _WIN32
    // WinHTTP performs the TLS handshake, SNI, certificate-chain and host
    // verification. verify=0 explicitly disables only the certificate checks
    // (for loopback self-signed servers); encryption is still negotiated.
    WCHAR wUrl[8192], wMethod[64], wAgent[64];
    MultiByteToWideChar(CP_UTF8, 0, url, -1, wUrl, 8192);
    MultiByteToWideChar(CP_UTF8, 0, m, -1, wMethod, 64);
    MultiByteToWideChar(CP_UTF8, 0, "karkain/1.2.17", -1, wAgent, 64);
    HINTERNET session = WinHttpOpen(wAgent, WINHTTP_ACCESS_TYPE_NO_PROXY,
        WINHTTP_NO_PROXY_NAME, WINHTTP_NO_PROXY_BYPASS, 0);
    if (!session) {
        snprintf(_karkain_tls_error, sizeof(_karkain_tls_error), "https: winhttp open failed (%lu)", (unsigned long)GetLastError());
        free(out); return make_string("");
    }
    URL_COMPONENTS uc;
    memset(&uc, 0, sizeof(uc));
    uc.dwStructSize = sizeof(uc);
    WCHAR wHost[512], wPath[4096];
    uc.lpszHostName = wHost; uc.dwHostNameLength = 512;
    uc.lpszUrlPath = wPath;  uc.dwUrlPathLength = 4096;
    if (!WinHttpCrackUrl(wUrl, 0, 0, &uc)) {
        snprintf(_karkain_tls_error, sizeof(_karkain_tls_error), "https: invalid url (%lu)", (unsigned long)GetLastError());
        WinHttpCloseHandle(session); free(out); return make_string("");
    }
    HINTERNET conn = WinHttpConnect(session, wHost, (INTERNET_PORT)pn, 0);
    if (!conn) {
        snprintf(_karkain_tls_error, sizeof(_karkain_tls_error), "https: connect failed (%lu)", (unsigned long)GetLastError());
        WinHttpCloseHandle(session); free(out); return make_string("");
    }
    HINTERNET reqh = WinHttpOpenRequest(conn, wMethod, uc.lpszUrlPath, NULL,
        WINHTTP_NO_REFERER, WINHTTP_DEFAULT_ACCEPT_TYPES, WINHTTP_FLAG_SECURE);
    if (!reqh) {
        snprintf(_karkain_tls_error, sizeof(_karkain_tls_error), "https: request failed (%lu)", (unsigned long)GetLastError());
        WinHttpCloseHandle(conn); WinHttpCloseHandle(session); free(out); return make_string("");
    }
    if (!do_verify) {
        DWORD flags = SECURITY_FLAG_IGNORE_UNKNOWN_CA |
            SECURITY_FLAG_IGNORE_CERT_WRONG_USAGE |
            SECURITY_FLAG_IGNORE_CERT_CN_INVALID |
            SECURITY_FLAG_IGNORE_CERT_DATE_INVALID;
        WinHttpSetOption(reqh, WINHTTP_OPTION_SECURITY_FLAGS, &flags, sizeof(flags));
    }
    DWORD bodyLen = (DWORD)strlen(bd);
    if (!WinHttpSendRequest(reqh, WINHTTP_NO_ADDITIONAL_HEADERS, 0,
            (LPVOID)bd, bodyLen, bodyLen, 0) ||
        !WinHttpReceiveResponse(reqh, NULL)) {
        snprintf(_karkain_tls_error, sizeof(_karkain_tls_error), "https: handshake failed (%lu)", (unsigned long)GetLastError());
        WinHttpCloseHandle(reqh); WinHttpCloseHandle(conn); WinHttpCloseHandle(session);
        free(out); return make_string("");
    }
    char buf[8192];
    for (;;) {
        DWORD avail = 0;
        if (!WinHttpQueryDataAvailable(reqh, &avail) || avail == 0) break;
        DWORD take = avail > sizeof(buf) ? (DWORD)sizeof(buf) : avail;
        DWORD got = 0;
        if (!WinHttpReadData(reqh, buf, take, &got) || got == 0) break;
        if (used + got > cap) got = (DWORD)(cap - used);
        memcpy(out + used, buf, got);
        used += got;
        if (used >= cap) break;
    }
    WinHttpCloseHandle(reqh);
    WinHttpCloseHandle(conn);
    WinHttpCloseHandle(session);
#else
    // POSIX: OpenSSL provides the handshake, SNI and (when requested) chain +
    // hostname verification. The TCP socket comes from the existing net
    // runtime so both engines share one resolver/connect path.
    SSL_library_init();
    SSL_CTX* ctx = SSL_CTX_new(TLS_client_method());
    if (!ctx) {
        karkain_tls_err("https: ssl context failed");
        free(out); return make_string("");
    }
    if (do_verify) {
        SSL_CTX_set_default_verify_paths(ctx);
        SSL_CTX_set_verify(ctx, SSL_VERIFY_PEER, NULL);
    } else {
        SSL_CTX_set_verify(ctx, SSL_VERIFY_NONE, NULL);
    }
    Value fdv = karkain_net_connect(host, port);
    if (fdv.type != TYPE_INT || fdv.intVal < 0) {
        karkain_tls_err("https: connection failed");
        SSL_CTX_free(ctx); free(out); return make_string("");
    }
    SSL* ssl = SSL_new(ctx);
    if (!ssl) {
        karkain_tls_err("https: ssl object failed");
        closesocket((KarkainSock)fdv.intVal);
        SSL_CTX_free(ctx); free(out); return make_string("");
    }
    SSL_set_fd(ssl, (int)fdv.intVal);
    SSL_set_tlsext_host_name(ssl, h);
    if (do_verify) X509_VERIFY_PARAM_set1_host(SSL_get0_param(ssl), h, 0);
    if (SSL_connect(ssl) <= 0) {
        snprintf(_karkain_tls_error, sizeof(_karkain_tls_error), "https: handshake failed (%lu)", ERR_get_error());
        SSL_free(ssl); closesocket((KarkainSock)fdv.intVal);
        SSL_CTX_free(ctx); free(out); return make_string("");
    }
    int written = SSL_write(ssl, req, (int)strlen(req));
    if (written <= 0) {
        karkain_tls_err("https: write failed");
        SSL_shutdown(ssl); SSL_free(ssl); closesocket((KarkainSock)fdv.intVal);
        SSL_CTX_free(ctx); free(out); return make_string("");
    }
    char buf[8192];
    while (used < cap) {
        int n = SSL_read(ssl, buf, (int)sizeof(buf));
        if (n <= 0) break;
        if (used + (size_t)n > cap) n = (int)(cap - used);
        memcpy(out + used, buf, (size_t)n);
        used += (size_t)n;
    }
    SSL_shutdown(ssl);
    SSL_free(ssl);
    closesocket((KarkainSock)fdv.intVal);
    SSL_CTX_free(ctx);
#endif

    out[used] = '\0';
    // WinHTTP hands back the entity body; OpenSSL reads the raw HTTP stream.
    // Normalize both engines to the body so a program prints the same bytes
    // regardless of platform.
    char* payload = out;
    char* sep = strstr(out, "\r\n\r\n");
    if (sep) { payload = sep + 4; }
    Value result = make_string(payload);
    free(out);
    return result;
}
