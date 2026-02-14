# ADR-002: BM25 Ranking with Global IDF

## Status

Accepted

## Context

Search results need relevance ranking. Without ranking, results are returned in arbitrary order (e.g., by document ID), which is useless for the end user.

Options considered:
1. **TF-IDF** — Classic, simple, but known to over-weight term frequency
2. **BM25 (Okapi)** — Industry standard, used by Elasticsearch and Lucene
3. **Language models (BM25F, learning-to-rank)** — More sophisticated but requires training data

## Decision

Implement Okapi BM25 with:
- **k1 = 1.2** — Controls term frequency saturation. Higher values give more weight to repeated terms. 1.2 is the standard default.
- **b = 0.75** — Controls document length normalization. 0 = no normalization, 1 = full normalization. 0.75 is standard.
- **Global IDF** — In a sharded index, IDF must be computed across all shards, not per-shard. Otherwise, rare terms in one shard get artificially inflated scores.

Formula:
```
IDF(t) = log((N - df(t)) / (df(t) + 0.5) + 1)

TF_norm(t,d) = (tf * (k1 + 1)) / (tf + k1 * (1 - b + b * dl/avgdl))

Score(q,d) = Σ IDF(t) * TF_norm(t,d)  for each term t in query q
```

## Consequences

**Positive:**
- Industry-standard relevance — results feel natural
- Handles document length bias (long documents don't unfairly dominate)
- Term frequency saturation prevents keyword stuffing
- Global IDF ensures consistent scoring across shards

**Negative:**
- Requires aggregating totalDocs and docFreq across all shards before scoring
- No field weighting (title matches scored same as body matches)
- No proximity scoring (adjacent terms don't get bonus)

## Future Improvements

- BM25F (field-weighted variant: title × 2.0, body × 1.0)
- Proximity boosting for phrase-like queries
- Click-through learning-to-rank layer
