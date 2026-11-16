"""
experiment_v2.py — Honest comparative evaluation of RecSys candidates.

Models compared:
  - Random
  - Popularity
  - Deterministic fallback rule (mirrors Go fallback)
  - Logistic Regression (sklearn)
  - CatBoostClassifier (pointwise)
  - CatBoostRanker (YetiRank) — Hybrid: content features + ALS score
  - ALS (implicit) — collaborative filtering only

Evaluation:
  - Intern-grouped split: 70% train / 15% val / 15% test.
    Each unique (intern, task) pair appears in EXACTLY ONE partition.
  - All per-intern candidates are scored by every model with identical
    queries (same Split, same per-intern candidate sets, same test rows).
  - Metrics: NDCG@5, NDCG@10, Precision@5, Precision@10, Recall@5,
    Recall@10, MAP@5, MAP@10, MRR, HitRate@5, HitRate@10, Catalog
    Coverage@5, Catalog Coverage@10.

Output:
  - ml/recsys/artifacts/experiment_v2_metrics.json
  - ml/recsys/artifacts/experiment_v2_table.md

Run: python ml/recsys/experiment_v2.py
"""

from __future__ import annotations

import json
import random
import sys
import time
from collections import defaultdict
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent.parent
sys.path.insert(0, str(ROOT))

from ml.recsys.dataset_split import Row, Split, leak_free_split, load_rows_from_dataset
from ml.recsys.features import FEATURE_NAMES, InternView, TaskView, featurize, build_views
from ml.recsys.ranking_eval import (
    RankingMetrics,
    evaluate_ranking,
    baseline_random,
    baseline_popularity,
    baseline_fallback_rule,
)

ART = ROOT / "ml" / "recsys" / "artifacts"
ART.mkdir(parents=True, exist_ok=True)

SEED = 42
DATASET_PATH = ART / "dataset.json"
random.seed(SEED)


# ---------------------------------------------------------------------------
# Per-model scorer adapters
# ---------------------------------------------------------------------------

def _lr_scorer(model, intern_views, task_views, prior_counts):
    """Train features use prior_counts[(iid,)]; for test, prior_counts is
    computed only over TRAIN rows (already the case in Split.prior_counts).
    """
    def score(intern_id: str, task_id: str) -> float:
        iv = intern_views.get(intern_id)
        tv = task_views.get(task_id)
        if iv is None or tv is None:
            return 0.0
        pos, neg = prior_counts.get(intern_id, (0, 0))
        fv = featurize(iv, tv, prior_pos=pos, prior_neg=neg)
        return float(model.predict_proba([fv])[0, 1])
    return score


def _cb_clf_scorer(model, intern_views, task_views, prior_counts):
    def score(intern_id: str, task_id: str) -> float:
        iv = intern_views.get(intern_id)
        tv = task_views.get(task_id)
        if iv is None or tv is None:
            return 0.0
        pos, neg = prior_counts.get(intern_id, (0, 0))
        fv = featurize(iv, tv, prior_pos=pos, prior_neg=neg)
        return float(model.predict_proba([fv])[0, 1])
    return score


def _als_scorer(user_factors, item_factors, intern_index, task_index):
    import numpy as np
    def score(intern_id: str, task_id: str) -> float:
        if intern_id not in intern_index or task_id not in task_index:
            return 0.0
        return float(np.dot(user_factors[intern_index[intern_id]],
                            item_factors[task_index[task_id]]))
    return score


def _hybrid_scorer(ranker, als_score_fn, intern_views, task_views, prior_counts):
    """CatBoostRanker (YetiRank) trained on (content features + ALS score)."""
    import numpy as np
    def score(intern_id: str, task_id: str) -> float:
        iv = intern_views.get(intern_id)
        tv = task_views.get(task_id)
        if iv is None or tv is None:
            return 0.0
        pos, neg = prior_counts.get(intern_id, (0, 0))
        fv = featurize(iv, tv, prior_pos=pos, prior_neg=neg)
        als_v = als_score_fn(intern_id, task_id)
        fv_ext = fv + [float(als_v)]
        return float(ranker.predict([fv_ext])[0])
    return score


# ---------------------------------------------------------------------------
# Build feature matrices per partition (for sklearn/CatBoost pointwise training)
# ---------------------------------------------------------------------------

def _build_xy(rows: list[Row], intern_views, task_views, prior_counts, extra_dim: list[float] | None = None):
    X, y = [], []
    for r in rows:
        iv = intern_views.get(r.intern_id)
        tv = task_views.get(r.task_id)
        if iv is None or tv is None:
            continue
        pos, neg = prior_counts.get(r.intern_id, (0, 0))
        fv = featurize(iv, tv, prior_pos=pos, prior_neg=neg)
        if extra_dim is not None and len(extra_dim) >= len(X):
            fv = fv + [extra_dim[len(X)]]
        X.append(fv)
        y.append(r.label)
    return X, y


def _build_xy_with_als(rows, intern_views, task_views, prior_counts, als_fn):
    X, y = [], []
    for r in rows:
        iv = intern_views.get(r.intern_id)
        tv = task_views.get(r.task_id)
        if iv is None or tv is None:
            continue
        pos, neg = prior_counts.get(r.intern_id, (0, 0))
        fv = featurize(iv, tv, prior_pos=pos, prior_neg=neg)
        fv = fv + [float(als_fn(r.intern_id, r.task_id))]
        X.append(fv)
        y.append(r.label)
    return X, y


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def main() -> int:
    print("[v2] === LOAD DATASET ===")
    rows = load_rows_from_dataset(DATASET_PATH)
    split = leak_free_split(rows, seed=SEED)
    stats = split.stats()
    print(f"[v2] split stats: train_n={stats['train']['n']} val_n={stats['val']['n']} "
          f"test_n={stats['test']['n']} prior_interns={stats['prior_interns']}")

    # Build views from raw dataset.json
    ds_dict = json.loads(DATASET_PATH.read_text(encoding="utf-8"))
    intern_views, task_views = build_views(ds_dict)
    print(f"[v2] views: interns={len(intern_views)} tasks={len(task_views)}")

    results: dict[str, dict] = {}

    # -------- Baselines --------
    t0 = time.time()
    m = baseline_random(split, seed=SEED)
    results["random"] = m.as_table_row() | {"fit_seconds": time.time() - t0}
    print(f"[v2] random: NDCG@5={m.ndcg_at_5:.4f}")

    t0 = time.time()
    m = baseline_popularity(split)
    results["popularity"] = m.as_table_row() | {"fit_seconds": time.time() - t0}
    print(f"[v2] popularity: NDCG@5={m.ndcg_at_5:.4f}")

    t0 = time.time()
    m = baseline_fallback_rule(split, intern_views, task_views)
    results["fallback_rule"] = m.as_table_row() | {"fit_seconds": time.time() - t0}
    print(f"[v2] fallback_rule: NDCG@5={m.ndcg_at_5:.4f}")

    # -------- Logistic Regression --------
    from sklearn.linear_model import LogisticRegression
    X_train, y_train = _build_xy(split.train, intern_views, task_views, split.prior_counts)
    X_test,  y_test  = _build_xy(split.test,  intern_views, task_views, split.prior_counts)
    print(f"[v2] LR feature shapes: X_train={len(X_train)} X_test={len(X_test)}")

    t0 = time.time()
    lr = LogisticRegression(max_iter=1000, random_state=SEED)
    lr.fit(X_train, y_train)
    fit_seconds = time.time() - t0
    m = evaluate_ranking(split, _lr_scorer(lr, intern_views, task_views, split.prior_counts))
    results["logistic_regression"] = m.as_table_row() | {"fit_seconds": fit_seconds}
    print(f"[v2] logistic_regression: NDCG@5={m.ndcg_at_5:.4f}")

    # -------- CatBoostClassifier --------
    try:
        from catboost import CatBoostClassifier
        t0 = time.time()
        cb = CatBoostClassifier(
            iterations=300, depth=6, learning_rate=0.05,
            random_seed=SEED, verbose=False,
        )
        X_val, y_val = _build_xy(split.val, intern_views, task_views, split.prior_counts)
        cb.fit(X_train, y_train, eval_set=(X_val, y_val), early_stopping_rounds=30)
        fit_seconds = time.time() - t0
        m = evaluate_ranking(
            split,
            _cb_clf_scorer(cb, intern_views, task_views, split.prior_counts),
        )
        results["catboost_classifier"] = m.as_table_row() | {"fit_seconds": fit_seconds}
        print(f"[v2] catboost_classifier: NDCG@5={m.ndcg_at_5:.4f}")
    except Exception as e:
        print(f"[v2] catboost_classifier: SKIPPED ({type(e).__name__}: {e})")
        results["catboost_classifier"] = {"error": str(e)}

    # -------- ALS (implicit) --------
    try:
        import numpy as np
        from scipy.sparse import csr_matrix
        from implicit.als import AlternatingLeastSquares

        intern_ids_sorted = sorted(intern_views.keys())
        task_ids_sorted = sorted(task_views.keys())
        intern_index = {iid: idx for idx, iid in enumerate(intern_ids_sorted)}
        task_index = {tid: idx for idx, tid in enumerate(task_ids_sorted)}

        # Build user-item matrix from TRAIN interactions only (leak-free).
        pairs = []
        for r in split.train:
            if r.intern_id in intern_index and r.task_id in task_index:
                pairs.append((intern_index[r.intern_id], task_index[r.task_id], r.label))
        rr, cc, vv = [], [], []
        for i, j, lbl in pairs:
            rr.append(i); cc.append(j); vv.append(1.0 if lbl == 1 else 0.1)
        mat = csr_matrix((vv, (rr, cc)), shape=(len(intern_ids_sorted), len(task_ids_sorted)))

        t0 = time.time()
        als = AlternatingLeastSquares(
            factors=32, regularization=0.05, iterations=30,
            random_state=SEED, use_gpu=False,
        )
        als.fit(mat, show_progress=False)
        fit_seconds = time.time() - t0
        als_user_factors = als.user_factors
        als_item_factors = als.item_factors
        als_scorer = _als_scorer(als_user_factors, als_item_factors, intern_index, task_index)
        m = evaluate_ranking(split, als_scorer)
        results["als"] = m.as_table_row() | {"fit_seconds": fit_seconds}
        print(f"[v2] als: NDCG@5={m.ndcg_at_5:.4f}")
    except Exception as e:
        print(f"[v2] als: SKIPPED ({type(e).__name__}: {e})")
        results["als"] = {"error": str(e)}
        als_scorer = None

    # -------- CatBoostRanker (Hybrid: content + ALS) --------
    try:
        from catboost import CatBoostRanker, Pool
        if als_scorer is None:
            raise RuntimeError("ALS scorer unavailable; skipping hybrid ranker")

        def _build_ranker_xy(rows):
            triplets = []
            for r in rows:
                iv = intern_views.get(r.intern_id)
                tv = task_views.get(r.task_id)
                if iv is None or tv is None:
                    continue
                pos, neg = split.prior_counts.get(r.intern_id, (0, 0))
                fv = featurize(iv, tv, prior_pos=pos, prior_neg=neg)
                fv = fv + [float(als_scorer(r.intern_id, r.task_id))]
                triplets.append((r.intern_id, r.label, fv))
            # CatBoost requires group_id rows to be CONTIGUOUS: sort by intern_id.
            triplets.sort(key=lambda t: t[0])
            X = [t[2] for t in triplets]
            y = [t[1] for t in triplets]
            g = [t[0] for t in triplets]
            return X, y, g

        Xtr, ytr, gtr = _build_ranker_xy(split.train)
        Xva, yva, gva = _build_ranker_xy(split.val)

        t0 = time.time()
        train_pool = Pool(Xtr, label=ytr, group_id=gtr)
        val_pool = Pool(Xva, label=yva, group_id=gva)
        ranker = CatBoostRanker(
            iterations=500, depth=6, learning_rate=0.05,
            loss_function="YetiRankPairwise",
            random_seed=SEED, verbose=False,
        )
        ranker.fit(train_pool, eval_set=val_pool, early_stopping_rounds=30)
        fit_seconds = time.time() - t0

        m = evaluate_ranking(
            split,
            _hybrid_scorer(ranker, als_scorer, intern_views, task_views, split.prior_counts),
        )
        results["catboost_ranker_hybrid"] = m.as_table_row() | {"fit_seconds": fit_seconds}
        print(f"[v2] catboost_ranker_hybrid: NDCG@5={m.ndcg_at_5:.4f}")
    except Exception as e:
        print(f"[v2] catboost_ranker_hybrid: SKIPPED ({type(e).__name__}: {e})")
        results["catboost_ranker_hybrid"] = {"error": str(e)}

    # -------- Save metrics --------
    out_path = ART / "experiment_v2_metrics.json"
    out_path.write_text(json.dumps(results, indent=2, ensure_ascii=False), encoding="utf-8")
    print(f"[v2] wrote {out_path}")

    # -------- Markdown table --------
    cols = ["NDCG@5", "NDCG@10", "P@5", "P@10", "R@5", "R@10", "MAP@5", "MAP@10",
            "MRR", "HR@5", "HR@10", "Coverage@5", "Coverage@10"]
    lines = [
        "# Experiment v2 — Honest Comparison",
        "",
        "Same Split, same per-intern test queries, leak-free features (prior counts",
        "computed from TRAIN rows only; each (intern, task) pair appears in exactly",
        "one partition).",
        "",
        "| Model | " + " | ".join(cols) + " | fit_sec |",
        "|---|" + "|".join(["---:"] * (len(cols) + 1)) + "|",
    ]
    order = [
        "random", "popularity", "fallback_rule",
        "logistic_regression", "catboost_classifier",
        "als", "catboost_ranker_hybrid",
    ]
    for name in order:
        r = results.get(name, {})
        if "error" in r:
            row = ["ERROR: " + r["error"]] + ["-"] * (len(cols)) + ["-"]
        else:
            row = [f"{r.get(c, 0):.4f}" if isinstance(r.get(c), (int, float)) else "-"
                   for c in cols]
            row.append(f"{r.get('fit_seconds', 0):.2f}")
        lines.append(f"| {name} | " + " | ".join(row) + " |")
    md_path = ART / "experiment_v2_table.md"
    md_path.write_text("\n".join(lines), encoding="utf-8")
    print(f"[v2] wrote {md_path}")

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
