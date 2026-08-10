// Phase 16: RPC (Remote Procedure Call) runtime
// Implements TCP wire protocol for distributed actor communication

#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

#ifdef _WIN32
#include <winsock2.h>
#pragma comment(lib, "ws2_32.lib")
#else
#include <sys/socket.h>
#include <netinet/in.h>
#include <arpa/inet.h>
#include <unistd.h>
#define closesocket close
#endif

// RPC message types
typedef enum {
    RPC_SPAWN,
    RPC_SEND,
    RPC_STOP,
    RPC_PING
} RPCMessageType;

// RPC message structure
typedef struct {
    RPCMessageType type;
    int actor_id;
    int sender_id;
    size_t data_size;
    char data[1024];
} RPCMessage;

// RPC node structure
typedef struct {
    int node_id;
    int port;
    int socket_fd;
    int is_running;
} RPCNode;

// Initialize RPC node
RPCNode* rpc_init_node(int node_id, int port) {
    RPCNode* node = (RPCNode*)malloc(sizeof(RPCNode));
    node->node_id = node_id;
    node->port = port;
    node->is_running = 1;
    
#ifdef _WIN32
    WSADATA wsa_data;
    WSAStartup(MAKEWORD(2, 2), &wsa_data);
#endif
    
    // Create socket
    node->socket_fd = socket(AF_INET, SOCK_STREAM, 0);
    if (node->socket_fd < 0) {
        free(node);
        return NULL;
    }
    
    // Bind to port
    struct sockaddr_in addr;
    addr.sin_family = AF_INET;
    addr.sin_addr.s_addr = INADDR_ANY;
    addr.sin_port = htons(port);
    
    if (bind(node->socket_fd, (struct sockaddr*)&addr, sizeof(addr)) < 0) {
        closesocket(node->socket_fd);
        free(node);
        return NULL;
    }
    
    // Listen for connections
    listen(node->socket_fd, 10);
    
    return node;
}

// Send RPC message to remote node
int rpc_send_message(RPCNode* node, const char* host, int port, RPCMessage* msg) {
    int sock = socket(AF_INET, SOCK_STREAM, 0);
    if (sock < 0) return 0;
    
    struct sockaddr_in addr;
    addr.sin_family = AF_INET;
    addr.sin_port = htons(port);
    inet_pton(AF_INET, host, &addr.sin_addr);
    
    if (connect(sock, (struct sockaddr*)&addr, sizeof(addr)) < 0) {
        closesocket(sock);
        return 0;
    }
    
    send(sock, (char*)msg, sizeof(RPCMessage), 0);
    closesocket(sock);
    
    return 1;
}

// Spawn actor on remote node
int rpc_spawn_remote(RPCNode* node, const char* host, int port, const char* actor_name) {
    RPCMessage msg;
    msg.type = RPC_SPAWN;
    msg.actor_id = 0;
    msg.sender_id = node->node_id;
    msg.data_size = strlen(actor_name);
    strncpy(msg.data, actor_name, sizeof(msg.data) - 1);
    
    return rpc_send_message(node, host, port, &msg);
}

// Stop RPC node
void rpc_stop_node(RPCNode* node) {
    node->is_running = 0;
    closesocket(node->socket_fd);
    
#ifdef _WIN32
    WSACleanup();
#endif
    
    free(node);
}