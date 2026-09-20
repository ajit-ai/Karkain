std.http
========

HTTP/1.1 client and server building blocks.

Phase 125A added minimal HTTP/1.1 support building on the ``net_*`` runtime
builtins. The server half provides building blocks for deterministic single-
process loopback choreography; ``http_get`` / ``http_post`` are one-shot clients.

.. note::

   Body transfer is Content-Length based. Every message terminates with
   ``Connection: close``. Failure is signaled via the ``error`` field.

Types
-----

``Request`` 
    ``{ method, path, headers, body, error }``
``Response`` 
    ``{ status, reason, headers, body, error }``
``headers`` 
    Map of header name (string) → header value (string)

Client functions
----------------

``http_make_request(method, path, headers, body)`` 
    Build a Request value
``http_new_headers()`` 
    Return empty header map
``http_get(url)`` 
    One-shot GET request (returns Response)
``http_post(url, headers, body)`` 
    One-shot POST request (returns Response)
``http_request_send(req, conn)`` 
    Send Request over connection
``http_response_recv(conn)`` 
    Receive Response from connection

Server functions
----------------

``http_make_response(status, reason, headers, body)`` 
    Build a Response value
``http_listen(host, port)`` 
    Bind and listen for HTTP connections
``http_accept(listener)`` 
    Accept next HTTP connection
``http_read_request(conn)`` 
    Read HTTP request from connection
``http_write_response(conn, resp)`` 
    Write HTTP response to connection
``http_close(conn)`` 
    Close HTTP connection

Parsing utilities
-----------------

``http_parse_request(data)`` 
    Parse HTTP request data into Request
``http_parse_response(data)`` 
    Parse HTTP response data into Response
``http_parse_headers(data)`` 
    Parse headers into map
``http_split_message(data)`` 
    Split message into headers and body
``http_build_request(req)`` 
    Serialize Request to HTTP string
``http_build_response(resp)`` 
    Serialize Response to HTTP string
``http_build_headers(headers)`` 
    Serialize headers to HTTP string

Example
-------

.. code-block:: kark

   import std.http

   func main() {
       let req = http_make_request("GET", "/", http_new_headers(), "")
       let resp = http_get("http://example.com")
       if (resp["error"] == "") {
           print resp["body"]
       }
   }

See ``examples/07-web/`` for HTTP loopback and codec examples.
