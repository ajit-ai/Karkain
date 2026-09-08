# 13 Finance — Compound Interest, Moving Averages, Volatility, EMI

STATUS: WORKING TODAY (validated).

## What it demonstrates

- annual compounding in floats (`amount * (1 + rate)` per year)
- simple moving average over a price window
- volatility = population stddev of prices (Newton float sqrt)
- EMI-style loan payment by compounding a monthly rate

## Commands

```
karkain check examples/showcase/13_finance/finance/main.kark
karkain run   examples/showcase/13_finance/finance/main.kark
```

## Expected output (verified)

```
compound_1000@5pct_10y=1628.89
ma3[0]=101
ma3[1]=102.667
volatility=3.68179
emi_100k@8pct_12mo=8698.84
```