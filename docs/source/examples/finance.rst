Finance
=======

:implemented:`Implemented` — financial numerics on **synthetic data only**
live under ``examples/13-finance/`` and run byte-identically on both engines.
Nothing here stores real financial data or connects to any external system.

.. list-table:: examples/13-finance/
   :widths: 30 70
   :header-rows: 1

   * - File
     - Demonstrates
   * - ``01_compound_interest.kark``
     - Integer-penny compounding, year by year
   * - ``02_loan_amortization.kark``
     - Annuity payment via iterated discount factor (no ``pow`` needed)
   * - ``03_npv.kark``
     - Net present value of a discounted cash-flow stream
   * - ``04_portfolio.kark``
     - Weighted-average portfolio returns with exact integer weights

Compound interest (excerpt)
---------------------------

.. code-block:: kark

   func compound(start, years, percent) {
       let amount = start
       let year = 0
       while (year < years) {
           amount = amount + amount * percent / 100
           year = year + 1
       }
       return amount
   }

Run:

.. code-block:: console

   $ karkain run examples/13-finance/01_compound_interest.kark

.. note::

   Float output prints at 6 significant figures on both engines; where
   exactness matters the examples use integer cents.

.. seealso::

   :doc:`/examples/security` — hashing and encoding used alongside finance
   demos in real applications.