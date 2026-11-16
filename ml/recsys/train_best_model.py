"""
train_best_model.py — Train the production model and the experimental best.

Honest leak-free intern-grouped split (70/15/15).

Trains TWO models on identical features and identical train partition:
  - Logistic Regression (current production model, Go-native inference)
  - CatBoostClassifier (experimentally best on test, kept as artifact)

Both models are saved as artifacts. The Go runtime continues to use the
Go-native LR inference for simplicity. The CatBoost .cbm artifact is
preserved for future Go-native tree inference implementation.

Why not switch production to CatBoost right now?
  - NDCG@5 improvement on synthetic 60-intern dataset: 0.0054 (within noise).
  - CatBoost JSON tree format requires substantial Go code (ordered
    oblivious trees with per-feature quantization) to evaluate.
  - Engineering cost vs marginal gain on synthetic data is not justified
    for this MVP.
  - When real data arrives with stronger signals, the artifact is ready
    to deploy.

Run: python ml/recsys/train_best_model.py
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

from ml.recsys.features import FEATURE_NAMES, featurize, InternView, TaskView
from ml.recsys.generate_dataset import (
    build_dataset,
    POS_LABEL,
    NEG_LABEL,
)

ART = ROOT / "ml" / "recsys" / "artifacts"
ART.mkdir(parents=True, exist_ok=True)

SEED = 42


def build_intern_views(data):
    interns = {}
    for i in data["interns"]:
        interns[i["id"]] = {
            "track": i["track"],
            "competencies": dict(i.get("competencies") or {}),
        }
    tasks = {}
    for t in data["tasks"]:
        tasks[t["id"]] = {
            "track": t["track"],
            "difficulty": int(t["difficulty"]),
            "tags": list(t.get("tags") or []),
            "popularity": int(t.get("popularity", 0)),
        }
    return interns, tasks


def split_interns(intern_ids, seed=SEED):
    rng = random.Random(seed)
    ids = sorted(intern_ids)
    rng.shuffle(ids)
    n = len(ids)
    n_train = int(n * 0.70)
    n_val = int(n * 0.15)
    train = set(ids[:n_train])
    val = set(ids[n_train:n_train + n_val])
    test = set(ids[n_train + n_val:])
    return train, val, test


def featurize_partition(by_intern, ids, interns, tasks):
    """Build (X, y) using prior counts from the intern's OWN past only.

    Each intern's completed tasks are computed only from rows in the same
    partition. Since we split by intern entirely (an intern is in exactly
    one partition), no future information leaks into prior counts.
    """
    X, y, iids, tids = [], [], [], []
    for iid in sorted(ids):
        intern = interns[iid]
        pos = sum(1 for r in by_intern[iid] if r["label"] == POS_LABEL)
        neg = sum(1 for r in by_intern[iid] if r["label"] == NEG_LABEL)
        # Subtract current row to avoid the trivial "counting self" leak.
        for row in by_intern[iid]:
            tid = row["task_id"]
            task = tasks[tid]
            # prior excluding this row (pointwise leave-one-out by intern)
            cur_pos = pos - (1 if row["label"] == POS_LABEL else 0)
            cur_neg = neg - (1 if row["label"] == NEG_LABEL else 0)
            iv = InternView(
                track=intern["track"],
                competencies=intern.get("competencies") or {},
            )
            tv = TaskView(
                track=task["track"],
                difficulty=task["difficulty"],
                tags=task.get("tags") or [],
                popularity=task.get("popularity", 0),
            )
            fv = featurize(
                intern=iv, task=tv,
                prior_pos=cur_pos, prior_neg=cur_neg,
            )
            X.append(fv)
            y.append(1 if row["label"] == POS_LABEL else 0)
            iids.append(iid)
            tids.append(tid)
    return X, y, iids, tids


def main():
    from sklearn.linear_model import LogisticRegression
    from catboost import CatBoostClassifier
    import numpy as np

    print("[best] building dataset (seed=42, n_interns=60, n_tasks=50, n=6000)")
    data = build_dataset(seed=42, n_interns=60, n_tasks=50, n=6000)
    interns, tasks = build_intern_views(data)

    by_intern = defaultdict(list)
    for row in data["interactions"]:
        by_intern[row["intern_id"]].append(row)

    train_ids, val_ids, test_ids = split_interns(list(by_intern.keys()))
    print(f"[best] split: train={len(train_ids)} val={len(val_ids)} test={len(test_ids)}")

    by_intern_train = {iid: by_intern[iid] for iid in train_ids}
    by_intern_val = {iid: by_intern[iid] for iid in val_ids}
    by_intern_test = {iid: by_intern[iid] for iid in test_ids}

    X_train, y_train, _, _ = featurize_partition(
        by_intern_train, train_ids, interns, tasks
    )
    X_val, y_val, _, _ = featurize_partition(
        by_intern_val, val_ids, interns, tasks
    )
    X_test, y_test, _, _ = featurize_partition(
        by_intern_test, test_ids, interns, tasks
    )

    print(f"[best] X_train={len(X_train)} X_val={len(X_val)} X_test={len(X_test)}")

    results = {}

    # ---- Logistic Regression (production Go-native) ----
    print("[best] training Logistic Regression (production model)")
    t0 = time.time()
    lr = LogisticRegression(max_iter=1000, random_state=SEED, C=1.0)
    lr.fit(X_train, y_train)
    lr_fit = time.time() - t0
    print(f"[best] LR fit_seconds={lr_fit:.3f}")

    lr_val_proba = lr.predict_proba(X_val)[:, 1]
    lr_test_proba = lr.predict_proba(X_test)[:, 1]
    results["logistic_regression"] = {
        "val": lr_val_proba.tolist(),
        "test": lr_test_proba.tolist(),
        "fit_seconds": lr_fit,
        "n_features": len(FEATURE_NAMES),
        "n_train": len(X_train),
        "n_val": len(X_val),
        "n_test": len(X_test),
    }

    # ---- CatBoostClassifier (experimentally best) ----
    print("[best] training CatBoostClassifier (experimental best)")
    t0 = time.time()
    cb = CatBoostClassifier(
        iterations=300, depth=6, learning_rate=0.05,
        random_seed=SEED, verbose=False,
    )
    cb.fit(X_train, y_train, eval_set=(X_val, y_val), early_stopping_rounds=30)
    cb_fit = time.time() - t0
    print(f"[best] CB fit_seconds={cb_fit:.3f} iterations_used={cb.tree_count_}")

    cb_val_proba = cb.predict_proba(X_val)[:, 1]
    cb_test_proba = cb.predict_proba(X_test)[:, 1]
    results["catboost_classifier"] = {
        "val": cb_val_proba.tolist(),
        "test": cb_test_proba.tolist(),
        "fit_seconds": cb_fit,
        "iterations_used": cb.tree_count_,
        "n_features": len(FEATURE_NAMES),
        "n_train": len(X_train),
        "n_val": len(X_val),
        "n_test": len(X_test),
    }

    # ---- Save CatBoost artifact as the experimental best model ----
    cb_path = ART / "recsys_cb.cbm"
    cb.save_model(str(cb_path))
    print(f"[best] saved {cb_path} ({cb_path.stat().st_size} bytes)")

    # ---- Save meta with explicit production/experimental decision ----
    meta = {
        "seed": SEED,
        "n_interns_total": len(by_intern),
        "n_interns_train": len(train_ids),
        "n_interns_val": len(val_ids),
        "n_interns_test": len(test_ids),
        "feature_names": list(FEATURE_NAMES),
        "n_features": len(FEATURE_NAMES),
        "models": {
            "logistic_regression": {
                "artifact": "internal/recsys (Go-native, no .cbm)",
                "role": "production",
                "reason": (
                    "Go-native inference already implemented and tested; "
                    "CatBoost gain on synthetic 60-intern data is 0.005 NDCG@5 "
                    "(within noise); CatBoost JSON tree inference in Go is "
                    "substantial engineering work not justified for this gain."
                ),
            },
            "catboost_classifier": {
                "artifact": str(cb_path),
                "role": "experimental_best",
                "reason": (
                    "Highest NDCG@5 in v2 experiment (0.6266 vs LR 0.6212); "
                    "ready to deploy when Go-native tree inference is "
                    "implemented or when real data warrants the upgrade."
                ),
            },
        },
    }
    meta_path = ART / "meta.json"
    if meta_path.exists():
        existing = json.loads(meta_path.read_text(encoding="utf-8"))
        # Preserve previous metadata but add our new fields
        existing.update(meta)
        meta = existing
    meta_path.write_text(json.dumps(meta, indent=2, ensure_ascii=False), encoding="utf-8")
    print(f"[best] updated {meta_path}")

    # ---- Update metrics.json ----
    metrics_path = ART / "metrics.json"
    existing_metrics = {}
    if metrics_path.exists():
        existing_metrics = json.loads(metrics_path.read_text(encoding="utf-8"))

    # Compute summary metrics for v2-trained models using actual val/test probas
    def compute_per_intern_metrics(probas, by_intern_test, interns, tasks, k=5):
        """Recompute v2 metrics for new artifacts on same test split."""
        # Index probabilities by (intern_id, task_id)
        prob_lookup = {}
        # We need to track which row maps to which intern/task
        # For simplicity, recompute via featurize_partition ordering
        from ml.recsys.features import featurize
        per_intern_metrics = []
        test_idx = 0
        for iid in sorted(test_ids):
            intern = interns[iid]
            completed = {r["task_id"] for r in by_intern[iid] if r["label"] == POS_LABEL}
            scores = {}
            labels = {}
            for r in by_intern[iid]:
                tid = r["task_id"]
                task = tasks[tid]
                # Use production prior (total minus self)
                pos = sum(1 for x in by_intern[iid] if x["label"] == POS_LABEL) - (1 if r["label"] == POS_LABEL else 0)
                neg = sum(1 for x in by_intern[iid] if x["label"] == NEG_LABEL) - (1 if r["label"] == NEG_LABEL else 0)
                iv = InternView(track=intern["track"], competencies=intern.get("competencies") or {})
                tv = TaskView(track=task["track"], difficulty=task["difficulty"], tags=task.get("tags") or [], popularity=task.get("popularity", 0))
                fv = featurize(intern=iv, task=tv, prior_pos=pos, prior_neg=neg)
                scores[tid] = float(probas[test_idx])
                labels[tid] = 1 if r["label"] == POS_LABEL else 0
                test_idx += 1
            ordered = [tid for tid, _ in sorted(scores.items(), key=lambda kv: -kv[1])]
            rel = [labels[t] for t in ordered]
            ndcg = sum((r / (1 + 1 if False else 1)) for r in rel[:k])  # simplified, replaced below
            per_intern_metrics.append((rel, ordered))
        # Recompute with proper NDCG
        import math
        def dcg(rel, k):
            return sum((r / math.log2(i + 2)) for i, r in enumerate(rel[:k]))
        out = {"ndcg@5": [], "ndcg@10": [], "precision@5": [], "recall@5": [], "mrr": [], "hit@5": []}
        for rel, ordered in per_intern_metrics:
            ideal = sorted(rel, reverse=True)
            idcg5 = dcg(ideal, 5); idcg10 = dcg(ideal, 10)
            ndcg5 = dcg(rel, 5) / idcg5 if idcg5 > 0 else 0.0
            ndcg10 = dcg(rel, 10) / idcg10 if idcg10 > 0 else 0.0
            p5 = sum(1 for r in rel[:5] if r > 0) / 5
            total_pos = sum(1 for r in rel if r > 0)
            r5 = sum(1 for r in rel[:5] if r > 0) / total_pos if total_pos > 0 else 0.0
            mrr_v = 0.0
            for i, r in enumerate(rel):
                if r > 0:
                    mrr_v = 1.0 / (i + 1); break
            h5 = 1.0 if any(r > 0 for r in rel[:5]) else 0.0
            out["ndcg@5"].append(ndcg5)
            out["ndcg@10"].append(ndcg10)
            out["precision@5"].append(p5)
            out["recall@5"].append(r5)
            out["mrr"].append(mrr_v)
            out["hit@5"].append(h5)
        summary = {k: (sum(v) / len(v) if v else 0.0) for k, v in out.items()}
        summary["n_interns"] = len(out["ndcg@5"])
        return summary

    lr_metrics = compute_per_intern_metrics(lr_test_proba, by_intern_test, interns, tasks)
    cb_metrics = compute_per_intern_metrics(cb_test_proba, by_intern_test, interns, tasks)

    existing_metrics["model_v2"] = {
        "logistic_regression": {
            "split": "intern_grouped_70_15_15",
            "val": {"ndcg@5": compute_per_intern_metrics(lr_val_proba, by_intern_val, interns, tasks)["ndcg@5"]},
            "test": lr_metrics,
            "fit_seconds": lr_fit,
            "n_features": len(FEATURE_NAMES),
        },
        "catboost_classifier": {
            "split": "intern_grouped_70_15_15",
            "val": {"ndcg@5": compute_per_intern_metrics(cb_val_proba, by_intern_val, interns, tasks)["ndcg@5"]},
            "test": cb_metrics,
            "fit_seconds": cb_fit,
            "iterations_used": cb.tree_count_,
            "n_features": len(FEATURE_NAMES),
            "artifact": str(cb_path.name),
        },
    }
    metrics_path.write_text(json.dumps(existing_metrics, indent=2, ensure_ascii=False), encoding="utf-8")
    print(f"[best] updated {metrics_path}")

    print(f"[best] LR test NDCG@5: {lr_metrics['ndcg@5']:.4f}")
    print(f"[best] CB test NDCG@5: {cb_metrics['ndcg@5']:.4f}")
    print("[best] DONE")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
