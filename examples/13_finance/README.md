# 13 — Finance

Financial numerics on **synthetic data only** — no real financial data is
stored, and nothing connects to external financial systems. Goals: show
Karkain's numeric/domain-programming ability cleanly.

| Example                     | Status   | Engine | Idea                                  |
|-----------------------------|----------|--------|---------------------------------------|
| `01_compound_interest.kark` | Runnable | both   | year-by-year integer compounding      |
| `02_loan_amortization.kark` | Runnable | both   | annuity payment by iterated factor    |
| `03_npv.kark`               | Runnable | both   | discounted cash flows                  |
| `04_portfolio.kark`         | Runnable | both   | weighted-average returns, integers    |

## Notes

- Integer-pennies where exactness matters; float output prints at 6
  significant figures on both engines.
- No pow() for arbitrary floats, so `(1+r)^-n` is built by iteration.
- Not financial advice; not a data-storage demo.

## Run

```
karkain run examples/13_finance/01_compound_interest.kark
```