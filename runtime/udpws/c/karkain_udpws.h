#ifndef KARKAIN_UDPWS_H
#define KARKAIN_UDPWS_H

// v1.2.21: UDP datagram and WebSocket text-frame runtime. The generated
// preamble defines Value, KarkainSock, socket headers, make_* helpers and
// send/recv before this block is appended. No new link library: Winsock is
// already linked with -lws2_32 on Windows; POSIX uses libc sockets.

Value karkain_udp_send(Value host, Value port, Value data);
Value karkain_udp_send_on(Value fd, Value host, Value port, Value data);
Value karkain_udp_bind(Value host, Value port);
Value karkain_udp_recv(Value fd, Value max);
Value karkain_udp_last_error(void);

Value karkain_ws_accept(Value key);
Value karkain_ws_send_text(Value fd, Value text);
Value karkain_ws_recv_text(Value fd, Value max);
Value karkain_ws_last_error(void);

#endif
