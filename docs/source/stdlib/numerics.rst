std.numerics
============

Numerical and machine learning foundation module.

Phase 126 added a deterministic numerical/ML runtime foundation written in
canonical Karkain. Every function uses only the byte-identical core surface
(untyped parameters, arithmetic operators, array operations) and computes
square roots and exponentials via fixed-iteration Newton/Taylor series for
byte-identical output on both engines.

.. note::

   Matrices are flat row-major ``float64[]`` plus explicit ``(rows, cols)``
   dimension parameters. Tensors are flat ``float64[]`` plus an explicit
   ``int[]`` shape. Shape mismatches raise deterministic ``assert`` errors on
   both engines.

Vector operations
-----------------

``numerics_vec_alloc(n)`` 
    Zero-initialized float64 vector of length n
``numerics_vec_sum(v)`` 
    Sum of all elements
``numerics_vec_mean(v)`` 
    Arithmetic mean
``numerics_vec_max(v)`` 
    Maximum element
``numerics_vec_min(v)`` 
    Minimum element
``numerics_vec_dot(v1, v2)`` 
    Dot product
``numerics_vec_scale(v, scalar)`` 
    Scale vector by scalar
``numerics_vec_add(v1, v2)`` 
    Element-wise addition
``numerics_vec_sub(v1, v2)`` 
    Element-wise subtraction
``numerics_vec_mul(v1, v2)`` 
    Element-wise multiplication
``numerics_vec_div(v1, v2)`` 
    Element-wise division

Matrix operations
-----------------

``numerics_mat_alloc(rows, cols)`` 
    Zero-initialized matrix
``numerics_mat_identity(n)`` 
    Identity matrix
``numerics_mat_add(m1, m2)`` 
    Matrix addition
``numerics_mat_sub(m1, m2)`` 
    Matrix subtraction
``numerics_mat_mul(m1, m2)`` 
    Matrix multiplication
``numerics_mat_transpose(m)`` 
    Matrix transpose
``numerics_mat_trace(m)`` 
    Matrix trace
``numerics_mat_det(m)`` 
    Matrix determinant (2x2 only)

Tensor operations
-----------------

``numerics_tensor_alloc(shape)`` 
    Zero-initialized tensor
``numerics_tensor_reshape(t, new_shape)`` 
    Reshape tensor
``numerics_tensor_slice(t, indices)`` 
    Slice tensor
``numerics_tensor_concat(t1, t2, axis)`` 
    Concatenate tensors
``numerics_tensor_sum(t)`` 
    Sum all elements
``numerics_tensor_mean(t)`` 
    Mean of all elements

Utility functions
-----------------

``numerics_sqrt(x)`` 
    Square root (Newton iteration)
``numerics_exp(x)`` 
    Exponential (Taylor series)
``numerics_log(x)`` 
    Natural logarithm
``numerics_pow(x, n)`` 
    Power function
``numerics_abs(x)`` 
    Absolute value
``numerics_max(a, b)`` 
    Maximum of two values
``numerics_min(a, b)`` 
    Minimum of two values
``numerics_clamp(x, lo, hi)`` 
    Clamp to range

Example
-------

.. code-block:: kark

   import std.numerics

   func main() {
       let v = numerics_vec_alloc(3)
       print numerics_vec_sum(v)  // 0.0
   }

See ``examples/09-ai/03_numerics_forward.kark`` for a complete example with
golden output validation.
