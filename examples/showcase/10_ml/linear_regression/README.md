# 10 ML — Least-Squares Linear Regression

STATUS: WORKING TODAY (validated).

## What it demonstrates

- closed-form least-squares fit from integer training data, computed in floats
- `slope = (n*Sxy - Sx*Sy) / (n*Sxx - Sx^2)`, `intercept = (Sy - m*Sx) / n`
- prediction with the fitted model
- the training data is the perfect line `y = 2x + 1`, so the fit is exact

## Commands

```
karkain check examples/showcase/10_ml/linear_regression/main.kark
karkain run   examples/showcase/10_ml/linear_regression/main.kark
```

## Expected output (verified)

```
slope=2
intercept=1
predict(10)=21
predict(-3)=-5
```