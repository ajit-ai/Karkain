#include "karkain_udpws.h"

#include <ctype.h>
#include <limits.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#define KARKAIN_WS_GUID "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

static char _karkain_udpws_error[256] = "";

static void karkain_udpws_err(const char* msg) {
    snprintf(_karkain_udpws_error, sizeof(_karkain_udpws_error), "%s", msg ? msg : "udp/ws failure");
}

Value karkain_udp_last_error(void) { return make_string(_karkain_udpws_error); }
Value karkain_ws_last_error(void) { return make_string(_karkain_udpws_error); }

static int karkain_udp_resolve(const char* host, int port, struct sockaddr_storage* out, socklen_t* outlen) {
    karkain_net_init();
    char pbuf[16];
    struct addrinfo hints;
    struct addrinfo* res = NULL;
    memset(&hints, 0, sizeof(hints));
    hints.ai_family = AF_UNSPEC;
    hints.ai_socktype = SOCK_DGRAM;
    snprintf(pbuf, sizeof(pbuf), "%d", port);
    int rc = getaddrinfo(host, pbuf, &hints, &res);
    if (rc != 0 || res == NULL) {
        snprintf(_karkain_udpws_error, sizeof(_karkain_udpws_error), "udp: invalid address (%d)", rc);
        return -1;
    }
    memcpy(out, res->ai_addr, res->ai_addrlen);
    *outlen = (socklen_t)res->ai_addrlen;
    freeaddrinfo(res);
    return 0;
}

static void karkain_udp_timeout(KarkainSock fd) {
#ifdef _WIN32
    unsigned long tmo = 5000;
    setsockopt(fd, SOL_SOCKET, SO_RCVTIMEO, (const char*)&tmo, sizeof(tmo));
#else
    struct timeval tv;
    tv.tv_sec = 5;
    tv.tv_usec = 0;
    setsockopt(fd, SOL_SOCKET, SO_RCVTIMEO, &tv, sizeof(tv));
#endif
}

static int karkain_send_all(KarkainSock fd, const unsigned char* data, size_t total) {
    size_t sent = 0;
    while (sent < total) {
        size_t chunk = total - sent > 0x4000 ? 0x4000 : total - sent;
        int n = send(fd, (const char*)data + sent, (int)chunk, 0);
        if (n <= 0) return -1;
        sent += (size_t)n;
    }
    return 0;
}

static int karkain_recv_exact(KarkainSock fd, unsigned char* out, size_t total) {
    size_t got = 0;
    while (got < total) {
        int n = recv(fd, (char*)out + got, (int)(total - got), 0);
        if (n <= 0) return -1;
        got += (size_t)n;
    }
    return 0;
}

Value karkain_udp_send(Value host, Value port, Value data) {
    _karkain_udpws_error[0] = '\0';
    if (host.type != TYPE_STRING || port.type != TYPE_INT || data.type != TYPE_STRING) {
        karkain_udpws_err("udp: invalid argument");
        return make_int(-1);
    }
    int p = (int)port.intVal;
    if (p < 1 || p > 65535) { karkain_udpws_err("udp: invalid port"); return make_int(-1); }
    struct sockaddr_storage addr;
    socklen_t addrlen = 0;
    if (karkain_udp_resolve(host.strVal ? host.strVal : "", p, &addr, &addrlen) != 0) return make_int(-1);
    KarkainSock fd = socket(addr.ss_family, SOCK_DGRAM, 0);
    if (fd == KARKAIN_INVALID_SOCKET) { karkain_udpws_err("udp: socket failed"); return make_int(-1); }
    const char* text = data.strVal ? data.strVal : "";
    int n = sendto(fd, text, (int)strlen(text), 0, (struct sockaddr*)&addr, addrlen);
    if (n <= 0) {
        karkain_udpws_err("udp: send failed");
        closesocket(fd);
        return make_int(-1);
    }
    closesocket(fd);
    return make_int((long long)n);
}

Value karkain_udp_send_on(Value fd, Value host, Value port, Value data) {
    _karkain_udpws_error[0] = '\0';
    if (fd.type != TYPE_INT || host.type != TYPE_STRING || port.type != TYPE_INT || data.type != TYPE_STRING) {
        karkain_udpws_err("udp: invalid argument");
        return make_int(-1);
    }
    int p = (int)port.intVal;
    if (p < 1 || p > 65535) { karkain_udpws_err("udp: invalid port"); return make_int(-1); }
    struct sockaddr_storage addr;
    socklen_t addrlen = 0;
    if (karkain_udp_resolve(host.strVal ? host.strVal : "", p, &addr, &addrlen) != 0) return make_int(-1);
    const char* text = data.strVal ? data.strVal : "";
    int n = sendto((KarkainSock)fd.intVal, text, (int)strlen(text), 0, (struct sockaddr*)&addr, addrlen);
    if (n <= 0) { karkain_udpws_err("udp: send failed"); return make_int(-1); }
    return make_int((long long)n);
}

Value karkain_udp_bind(Value host, Value port) {
    _karkain_udpws_error[0] = '\0';
    if (host.type != TYPE_STRING || port.type != TYPE_INT) {
        karkain_udpws_err("udp: invalid argument");
        return make_int(-1);
    }
    int p = (int)port.intVal;
    if (p < 0 || p > 65535) { karkain_udpws_err("udp: invalid port"); return make_int(-1); }
    struct sockaddr_storage addr;
    socklen_t addrlen = 0;
    if (karkain_udp_resolve(host.strVal ? host.strVal : "", p, &addr, &addrlen) != 0) return make_int(-1);
    KarkainSock fd = socket(addr.ss_family, SOCK_DGRAM, 0);
    if (fd == KARKAIN_INVALID_SOCKET) { karkain_udpws_err("udp: socket failed"); return make_int(-1); }
    int one = 1;
    setsockopt(fd, SOL_SOCKET, SO_REUSEADDR, (const char*)&one, sizeof(one));
    if (bind(fd, (struct sockaddr*)&addr, addrlen) != 0) {
        karkain_udpws_err("udp: bind failed");
        closesocket(fd);
        return make_int(-1);
    }
    karkain_udp_timeout(fd);
    return make_int((long long)fd);
}

Value karkain_udp_recv(Value fdv, Value maxv) {
    _karkain_udpws_error[0] = '\0';
    if (fdv.type != TYPE_INT || maxv.type != TYPE_INT) {
        karkain_udpws_err("udp: invalid argument");
        return make_string("");
    }
    long long maxn = maxv.intVal;
    if (maxn <= 0 || maxn > 65536) maxn = 65536;
    char* buf = (char*)malloc((size_t)maxn + 1);
    if (!buf) { karkain_udpws_err("udp: out of memory"); return make_string(""); }
    struct sockaddr_storage peer;
    socklen_t plen = sizeof(peer);
    int n = recvfrom((KarkainSock)fdv.intVal, buf, (int)maxn, 0, (struct sockaddr*)&peer, &plen);
    if (n <= 0) {
        free(buf);
        karkain_udpws_err(n == 0 ? "udp: closed" : "udp: receive timeout");
        return make_string("");
    }
    buf[n] = '\0';
    Value r = make_string(buf);
    free(buf);
    return r;
}

/* Minimal SHA-1 for RFC 6455 Sec-WebSocket-Accept. */
typedef struct {
    uint32_t h[5];
    unsigned char block[64];
    size_t used;
    uint64_t bits;
} KSha1;

static void ksha_init(KSha1* s) {
    s->h[0] = 0x67452301u; s->h[1] = 0xEFCDAB89u; s->h[2] = 0x98BADCFEu;
    s->h[3] = 0x10325476u; s->h[4] = 0xC3D2E1F0u;
    s->used = 0; s->bits = 0;
}

static uint32_t ksha_rol(uint32_t v, int n) { return (v << n) | (v >> (32 - n)); }

static void ksha_block(KSha1* s, const unsigned char* p) {
    uint32_t w[80];
    int i;
    for (i = 0; i < 16; i++) {
        w[i] = ((uint32_t)p[i*4] << 24) | ((uint32_t)p[i*4+1] << 16) |
               ((uint32_t)p[i*4+2] << 8) | (uint32_t)p[i*4+3];
    }
    for (i = 16; i < 80; i++) w[i] = ksha_rol(w[i-3] ^ w[i-8] ^ w[i-14] ^ w[i-16], 1);
    uint32_t a = s->h[0], b = s->h[1], c = s->h[2], d = s->h[3], e = s->h[4];
    for (i = 0; i < 80; i++) {
        uint32_t f, k;
        if (i < 20) { f = (b & c) | ((~b) & d); k = 0x5A827999u; }
        else if (i < 40) { f = b ^ c ^ d; k = 0x6ED9EBA1u; }
        else if (i < 60) { f = (b & c) | (b & d) | (c & d); k = 0x8F1BBCDCu; }
        else { f = b ^ c ^ d; k = 0xCA62C1D6u; }
        uint32_t temp = ksha_rol(a, 5) + f + e + k + w[i];
        e = d; d = c; c = ksha_rol(b, 30); b = a; a = temp;
    }
    s->h[0] += a; s->h[1] += b; s->h[2] += c; s->h[3] += d; s->h[4] += e;
}

static void ksha_update(KSha1* s, const unsigned char* data, size_t len) {
    s->bits += (uint64_t)len * 8;
    while (len) {
        size_t take = 64 - s->used;
        if (take > len) take = len;
        memcpy(s->block + s->used, data, take);
        s->used += take;
        data += take; len -= take;
        if (s->used == 64) { ksha_block(s, s->block); s->used = 0; }
    }
}

static void ksha_final(KSha1* s, unsigned char out[20]) {
    uint64_t bits = s->bits;
    unsigned char pad = 0x80;
    ksha_update(s, &pad, 1);
    pad = 0;
    while (s->used != 56) ksha_update(s, &pad, 1);
    unsigned char tail[8];
    int i;
    for (i = 0; i < 8; i++) tail[i] = (unsigned char)(bits >> (56 - i * 8));
    ksha_update(s, tail, 8);
    for (i = 0; i < 5; i++) {
        out[i*4] = (unsigned char)(s->h[i] >> 24);
        out[i*4+1] = (unsigned char)(s->h[i] >> 16);
        out[i*4+2] = (unsigned char)(s->h[i] >> 8);
        out[i*4+3] = (unsigned char)s->h[i];
    }
}

static char* karkain_b64(const unsigned char* in, size_t n) {
    static const char* tab = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    size_t outlen = ((n + 2) / 3) * 4;
    char* out = (char*)malloc(outlen + 1);
    if (!out) return NULL;
    size_t i = 0, j = 0;
    while (i + 2 < n) {
        unsigned v = ((unsigned)in[i] << 16) | ((unsigned)in[i+1] << 8) | in[i+2];
        out[j++] = tab[(v >> 18) & 63]; out[j++] = tab[(v >> 12) & 63];
        out[j++] = tab[(v >> 6) & 63]; out[j++] = tab[v & 63];
        i += 3;
    }
    if (i < n) {
        unsigned v = ((unsigned)in[i] << 16);
        if (i + 1 < n) v |= ((unsigned)in[i+1] << 8);
        out[j++] = tab[(v >> 18) & 63]; out[j++] = tab[(v >> 12) & 63];
        out[j++] = (i + 1 < n) ? tab[(v >> 6) & 63] : '=';
        out[j++] = '=';
    }
    out[j] = '\0';
    return out;
}

Value karkain_ws_accept(Value key) {
    _karkain_udpws_error[0] = '\0';
    if (key.type != TYPE_STRING) { karkain_udpws_err("ws: key must be a string"); return make_string(""); }
    const char* k = key.strVal ? key.strVal : "";
    size_t n = strlen(k) + strlen(KARKAIN_WS_GUID);
    char* joined = (char*)malloc(n + 1);
    if (!joined) { karkain_udpws_err("ws: out of memory"); return make_string(""); }
    snprintf(joined, n + 1, "%s%s", k, KARKAIN_WS_GUID);
    KSha1 s;
    unsigned char digest[20];
    ksha_init(&s);
    ksha_update(&s, (const unsigned char*)joined, n);
    ksha_final(&s, digest);
    free(joined);
    char* out = karkain_b64(digest, 20);
    if (!out) { karkain_udpws_err("ws: out of memory"); return make_string(""); }
    Value r = make_string(out);
    free(out);
    return r;
}

Value karkain_ws_send_text(Value fd, Value text) {
    _karkain_udpws_error[0] = '\0';
    if (fd.type != TYPE_INT || text.type != TYPE_STRING) {
        karkain_udpws_err("ws: invalid argument");
        return make_int(-1);
    }
    const unsigned char* data = (const unsigned char*)(text.strVal ? text.strVal : "");
    size_t len = strlen((const char*)data);
    size_t header;
    if (len < 126) header = 2;
    else if (len <= 65535) header = 4;
    else { karkain_udpws_err("ws: frame too large"); return make_int(-1); }
    size_t total = header + 4 + len;
    unsigned char* frame = (unsigned char*)malloc(total);
    if (!frame) { karkain_udpws_err("ws: out of memory"); return make_int(-1); }
    frame[0] = 0x81;
    size_t p = 2;
    if (len < 126) frame[1] = (unsigned char)(0x80 | len);
    else {
        frame[1] = (unsigned char)(0x80 | 126);
        frame[2] = (unsigned char)(len >> 8);
        frame[3] = (unsigned char)len;
        p = 4;
    }
    unsigned char mask[4] = {0x12, 0x34, 0x56, 0x78};
    memcpy(frame + p, mask, 4);
    p += 4;
    size_t i;
    for (i = 0; i < len; i++) frame[p + i] = data[i] ^ mask[i % 4];
    if (karkain_send_all((KarkainSock)fd.intVal, frame, total) != 0) {
        free(frame);
        karkain_udpws_err("ws: send failed");
        return make_int(-1);
    }
    free(frame);
    return make_int((long long)total);
}

Value karkain_ws_recv_text(Value fd, Value maxv) {
    _karkain_udpws_error[0] = '\0';
    if (fd.type != TYPE_INT || maxv.type != TYPE_INT) {
        karkain_udpws_err("ws: invalid argument");
        return make_string("");
    }
    long long cap = maxv.intVal;
    if (cap <= 0 || cap > 1048576) cap = 1048576;
    unsigned char head[2];
    if (karkain_recv_exact((KarkainSock)fd.intVal, head, 2) != 0) {
        karkain_udpws_err("ws: receive failed");
        return make_string("");
    }
    int opcode = head[0] & 0x0f;
    int masked = (head[1] & 0x80) != 0;
    unsigned long long n = head[1] & 0x7f;
    if (n == 126) {
        unsigned char x[2];
        if (karkain_recv_exact((KarkainSock)fd.intVal, x, 2) != 0) { karkain_udpws_err("ws: receive failed"); return make_string(""); }
        n = ((unsigned long long)x[0] << 8) | x[1];
    } else if (n == 127) {
        unsigned char x[8];
        if (karkain_recv_exact((KarkainSock)fd.intVal, x, 8) != 0) { karkain_udpws_err("ws: receive failed"); return make_string(""); }
        n = 0;
        int i;
        for (i = 0; i < 8; i++) n = (n << 8) | x[i];
    }
    unsigned char mask[4] = {0,0,0,0};
    if (masked && karkain_recv_exact((KarkainSock)fd.intVal, mask, 4) != 0) {
        karkain_udpws_err("ws: receive failed");
        return make_string("");
    }
    if (opcode == 0x8) return make_string("");
    if (opcode != 0x1) { karkain_udpws_err("ws: expected text frame"); return make_string(""); }
    if (n > (unsigned long long)cap) { karkain_udpws_err("ws: frame too large"); return make_string(""); }
    unsigned char* buf = (unsigned char*)malloc((size_t)n + 1);
    if (!buf) { karkain_udpws_err("ws: out of memory"); return make_string(""); }
    if (n && karkain_recv_exact((KarkainSock)fd.intVal, buf, (size_t)n) != 0) {
        free(buf); karkain_udpws_err("ws: receive failed"); return make_string("");
    }
    if (masked) {
        size_t i;
        for (i = 0; i < (size_t)n; i++) buf[i] ^= mask[i % 4];
    }
    buf[n] = 0;
    Value r = make_string((const char*)buf);
    free(buf);
    return r;
}
