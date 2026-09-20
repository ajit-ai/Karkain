std.net
=======

TCP networking module with connection and listener support.

Phase 125A added real TCP socket client/server building blocks based on the
``net_*`` runtime builtins (available on both engines). The module wraps
raw fd-based builtins behind friendly types.

.. note::

   Every networking function returns -1 (or "") on failure and leaves a
   deterministic last-error string readable via ``net_error()``. Blocking
   reads time out after 5 seconds.

Types
-----

``Address`` 
    ``{ host, port }`` — a single socket endpoint
``Endpoint`` 
    ``{ host, port, kind }`` — an address with a TCP scheme
``Connection`` 
    Integer fd from ``net_dial`` / ``net_accept_next``
``Listener`` 
    Integer fd from ``net_serve``

Functions
---------

Connection management
^^^^^^^^^^^^^^^^^^^^^

``net_address(host, port)`` 
    Build an Address value
``net_endpoint(host, port)`` 
    Build an Endpoint value with TCP scheme
``net_dial(point)`` 
    Open TCP connection to Address/Endpoint (returns fd or -1)
``net_serve(point)`` 
    Bind and listen on Address/Endpoint (returns Listener fd or -1)
``net_accept_next(listener)`` 
    Block for next pending connection (returns Connection fd or -1)
``net_close(fd)`` 
    Close a connection or listener

Data transfer
^^^^^^^^^^^^^

``net_send(fd, data)`` 
    Send data over connection (returns bytes sent or -1)
``net_recv(fd, max_bytes)`` 
    Receive data (returns string or "" on failure)
``net_send_all(fd, data)`` 
    Send all data (blocks until complete or error)
``net_recv_line(fd)`` 
    Receive until newline (returns string or "" on error)

Error handling
^^^^^^^^^^^^^^

``net_error()`` 
    Last error message (empty on success)
``net_fd_open(fd)`` 
    Check if fd is valid (>= 0)

Example
-------

.. code-block:: kark

   import std.net

   func main() {
       let addr = net_address("127.0.0.1", "8080")
       let conn = net_dial(addr)
       if (conn >= 0) {
           net_send(conn, "hello")
           net_close(conn)
       }
   }

See ``examples/04-networking/`` for complete TCP echo and roundtrip examples.
