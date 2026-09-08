# 05 Data — Text Statistics

STATUS: WORKING TODAY (validated).

## What it demonstrates

- a small analysis pipeline: tokenize (`split`), search (`longest word`),
  float statistics (`(chars * 1.0) / len(words)`)
- aggregation over words with parallel arrays (there is no map iteration API)

## Commands

```
karkain check examples/showcase/05_data/text_stats/main.kark
karkain run   examples/showcase/05_data/text_stats/main.kark
```

## Expected output (verified)

```
characters=43
words=9
longest=quick
avg_word_length=3.88889
```