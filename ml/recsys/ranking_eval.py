"""
ranking_eval.py — Per-intern ranking evaluation for RecSys.

Why per-intern?
---------------
For a recommender system, the unit of evaluation is "given intern i,
rank their candidate tasks and measure quality of the ranking."

Averaging Precision@K over flat test rows (as we did before) is wrong:
it treats every (intern, task) as an independent prediction, which is
fine for *classification* but tells us nothing about *ranking quality*.

The correct setup:
  For each intern present in the test split, gather all their test
  (candidate, label) rows. Score them with the model. Sort. Compute
  ranking metrics (NDCG@K, Precision@K, Recall@K, MAP@K, HitRate@K).
  Average across interns.

In addition to per-intern metrics we compute:
  - Catalog Coverage: fraction of unique task_ids appearing in top-K
    recommendations across all interns.
  - Recommendation Coverage: fraction of interns that received ≥1
    recommendation (i.e. had any candidates).

We also include reference baselines: random and popularity, computed on
the SAME test set / SAME per-intern candidate sets, so all numbers in
the model-selection table are comparable.
"""

from __future__ import annotations

import math
import random
from collections import defaultdict
from dataclasses import dataclass, field
from typing import Callable, Iterable, Sequence

from .dataset_split import Row, Split


@dataclass
class RankingMetrics:
    ndcg_at_5: float
    ndcg_at_10: float
    precision_at_5: float
    precision_at_10: float
    recall_at_5: float
    recall_at_10: float
    map_at_5: float
    map_at_10: float
    mrr: float
    hit_rate_at_5: float
    hit_rate_at_10: float
    catalog_coverage_at_5: float
    catalog_coverage_at_10: float
    n_interns: int
    n_queries: int  # how many per-intern queries were evaluated

    def as_table_row(self) -> dict:
        return {
            "NDCG@5":   round(self.ndcg_at_5, 4),
            "NDCG@10":  round(self.ndcg_at_10, 4),
            "P@5":      round(self.precision_at_5, 4),
            "P@10":     round(self.precision_at_10, 4),
            "R@5":      round(self.recall_at_5, 4),
            "R@10":     round(self.recall_at_10, 4),
            "MAP@5":    round(self.map_at_5, 4),
            "MAP@10":   round(self.map_at_10, 4),
            "MRR":      round(self.mrr, 4),
            "HR@5":     round(self.hit_rate_at_5, 4),
            "HR@10":    round(self.hit_rate_at_10, 4),
            "Coverage@5":  round(self.catalog_coverage_at_5, 4),
            "Coverage@10": round(self.catalog_coverage_at_10, 4),
            "n_interns": self.n_interns,
        }


def _dcg(rels: Sequence[int], k: int) -> float:
    return sum(
        (rel / math.log2(i + 2)) for i, rel in enumerate(rels[:k]) if rel > 0
    )


def _ndcg_at_k(rels_sorted: Sequence[int], k: int) -> float:
    """rels_sorted = labels ordered by predicted score desc."""
    dcg = _dcg(rels_sorted, k)
    ideal = sorted(rels_sorted, reverse=True)
    idcg = _dcg(ideal, k)
    if idcg == 0:
        return 0.0
    return dcg / idcg


def _precision_at_k(rels_sorted: Sequence[int], k: int) -> float:
    top = rels_sorted[:k]
    if not top:
        return 0.0
    return sum(1 for r in top if r > 0) / len(top)


def _recall_at_k(rels_sorted: Sequence[int], k: int, n_relevant: int) -> float:
    if n_relevant == 0:
        return 0.0
    top = rels_sorted[:k]
    return sum(1 for r in top if r > 0) / n_relevant


def _ap_at_k(rels_sorted: Sequence[int], k: int, n_relevant: int) -> float:
    if n_relevant == 0:
        return 0.0
    hits = 0
    score = 0.0
    for i, r in enumerate(rels_sorted[:k]):
        if r > 0:
            hits += 1
            score += hits / (i + 1)
    return score / min(n_relevant, k)


def _rr(rels_sorted: Sequence[int]) -> float:
    for i, r in enumerate(rels_sorted):
        if r > 0:
            return 1.0 / (i + 1)
    return 0.0


def _hit_rate_at_k(rels_sorted: Sequence[int], k: int) -> float:
    return 1.0 if any(r > 0 for r in rels_sorted[:k]) else 0.0


def _build_per_intern_queries(rows: Iterable[Row]) -> dict[str, list[Row]]:
    q: dict[str, list[Row]] = defaultdict(list)
    for r in rows:
        q[r.intern_id].append(r)
    return dict(q)


def _scored_ranker(
    score_fn: Callable[[str, str], float],
    query: dict[str, list[Row]],
    rng_seed: int | None = None,
) -> dict[str, list[tuple[str, float, int]]]:
    """Apply score_fn(intern_id, task_id) to every (intern, task) in query.
    Returns intern_id -> sorted list of (task_id, score, label), descending by score.
    """
    out: dict[str, list[tuple[str, float, int]]] = {}
    for intern_id, rs in query.items():
        scored = []
        for r in rs:
            s = score_fn(intern_id, r.task_id)
            scored.append((r.task_id, s, r.label))
        scored.sort(key=lambda x: x[1], reverse=True)
        out[intern_id] = scored
    return out


def _random_scorer(seed: int) -> Callable[[str, str], float]:
    rng = random.Random(seed)
    return lambda _i, _t: rng.random()


def _popularity_scorer(train_rows: list[Row]) -> Callable[[str, str], float]:
    counts: dict[str, int] = defaultdict(int)
    for r in train_rows:
        if r.label == 1:
            counts[r.task_id] += 1
    max_c = max(counts.values()) if counts else 1
    return lambda _i, t: counts.get(t, 0) / max_c


def evaluate_ranking(
    split: Split,
    score_fn: Callable[[str, str], float],
    name: str = "model",
    topk_values: tuple[int, ...] = (5, 10),
) -> RankingMetrics:
    """Evaluate score_fn as a per-intern ranker on the test split.

    score_fn(intern_id, task_id) -> float (higher = more relevant).
    """
    queries = _build_per_intern_queries(split.test)
    # Filter out interns with no positives (we can't compute AP / NDCG for them).
    eval_queries = {
        i: rs for i, rs in queries.items() if any(r.label > 0 for r in rs)
    }

    ndcg5, ndcg10 = [], []
    p5, p10 = [], []
    r5, r10 = [], []
    map5, map10 = [], []
    rr_list = []
    hr5, hr10 = [], []
    cov5, cov10 = set(), set()
    n_interns_all = 0  # all interns we returned recommendations for

    for intern_id, rs in eval_queries.items():
        scored = [(r.task_id, score_fn(intern_id, r.task_id), r.label) for r in rs]
        scored.sort(key=lambda x: x[1], reverse=True)
        rels = [s[2] for s in scored]
        n_rel = sum(1 for r in rels if r > 0)
        if n_rel == 0:
            continue

        n_interns_all += 1
        ndcg5.append(_ndcg_at_k(rels, 5))
        ndcg10.append(_ndcg_at_k(rels, 10))
        p5.append(_precision_at_k(rels, 5))
        p10.append(_precision_at_k(rels, 10))
        r5.append(_recall_at_k(rels, 5, n_rel))
        r10.append(_recall_at_k(rels, 10, n_rel))
        map5.append(_ap_at_k(rels, 5, n_rel))
        map10.append(_ap_at_k(rels, 10, n_rel))
        rr_list.append(_rr(rels))
        hr5.append(_hit_rate_at_k(rels, 5))
        hr10.append(_hit_rate_at_k(rels, 10))

        cov5.update(t for t, _, _ in scored[:5])
        cov10.update(t for t, _, _ in scored[:10])

    # Catalog coverage: fraction of unique task_ids ever appearing in any test query.
    all_tasks: set[str] = {r.task_id for r in split.test}
    cov5_frac = len(cov5 & all_tasks) / max(1, len(all_tasks))
    cov10_frac = len(cov10 & all_tasks) / max(1, len(all_tasks))

    def m(xs): return sum(xs) / len(xs) if xs else 0.0

    return RankingMetrics(
        ndcg_at_5=m(ndcg5),
        ndcg_at_10=m(ndcg10),
        precision_at_5=m(p5),
        precision_at_10=m(p10),
        recall_at_5=m(r5),
        recall_at_10=m(r10),
        map_at_5=m(map5),
        map_at_10=m(map10),
        mrr=m(rr_list),
        hit_rate_at_5=m(hr5),
        hit_rate_at_10=m(hr10),
        catalog_coverage_at_5=cov5_frac,
        catalog_coverage_at_10=cov10_frac,
        n_interns=n_interns_all,
        n_queries=len(eval_queries),
    )


def baseline_random(split: Split, seed: int = 0) -> RankingMetrics:
    return evaluate_ranking(split, _random_scorer(seed), name="random")


def baseline_popularity(split: Split) -> RankingMetrics:
    return evaluate_ranking(split, _popularity_scorer(split.train), name="popularity")


def fallback_rule_scorer(intern_views: dict, task_views: dict) -> "Callable[[str, str], float]":
    """Deterministic rule: sum of competency levels for overlapping tags - 0.1*(diff-1).

    Used to evaluate the existing deterministic fallback against ML models
    on the same test split.
    """
    def score(intern_id: str, task_id: str) -> float:
        iv = intern_views.get(intern_id)
        tv = task_views.get(task_id)
        if iv is None or tv is None:
            return 0.0
        s = 0.0
        comp = iv.competencies or {}
        for tag in (tv.tags or []):
            s += float(comp.get(tag, 0))
        s -= 0.1 * float(max(0, tv.difficulty - 1))
        return s
    return score


def baseline_fallback_rule(split: Split, intern_views: dict, task_views: dict) -> RankingMetrics:
    return evaluate_ranking(split, fallback_rule_scorer(intern_views, task_views), name="fallback_rule")


__all__ = [
    "RankingMetrics",
    "evaluate_ranking",
    "baseline_random",
    "baseline_popularity",
    "baseline_fallback_rule",
    "fallback_rule_scorer",
]
