# 02 Algorithms — FIFO Queue

STATUS: WORKING TODAY (validated). A queue is a data structure here, not a
concurrency facility (category 08 is NOT CURRENTLY SUPPORTED).

## What it demonstrates

- a FIFO queue built on a plain array
- arrays are passed by value into functions: `push` inside a helper mutates a
  copy, so the working idiom is `arr = enqueue(arr, v)` (the builtin returns
  the array); `dequeue` returns `[value, newQueue]`

## Commands

```
karkain check examples/showcase/02_algorithms/queue/main.kark
karkain run   examples/showcase/02_algorithms/queue/main.kark
```

## Expected output (verified)

```
10
0
10
20
30
1
```