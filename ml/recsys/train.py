"""
train.py — воспроизводимый training pipeline Hybrid RecSys.

Что делает:
  1. Загружает синтетический датасет (или генерирует на лету).
  2. Делает детерминированный train/val/test split по intern_id,
     чтобы избежать утечки между стажёрами (group split).
  3. Считает baseline = существующее правило scoreTaskAgainst
     (теги/60% + difficulty/40%, см. internal/api/sprint12.go).
  4. Обучает LogisticRegression на тех же фичах.
  5. Сравнивает baseline и модель по Precision@5, Recall@5, NDCG@5, ROC-AUC.
  6. Сохраняет артефакт joblib + meta.json + metrics.json.

Запуск:
    python -m ml.recsys.train --seed 42
"""

from __future__ import annotations

import argparse
import json
import math
import random
import sys
import time
from dataclasses import dataclass
from pathlib import Path

import joblib
import numpy as np
from sklearn.linear_model import LogisticRegression
from sklearn.metrics import (
    average_precision_score,
    log_loss,
    roc_auc_score,
)

# Поддерживаем запуск и как `python ml/recsys/train.py`, и как `python -m`.
if __package__:
    from .features import FEATURE_NAMES, InternView, TaskView, featurize
    from .generate_dataset import generate
else:
    sys.path.insert(0, str(Path(__file__).resolve().parent.parent.parent))
    from ml.recsys.features import FEATURE_NAMES, InternView, TaskView, featurize  # type: ignore
    from ml.recsys.generate_dataset import generate  # type: ignore


ARTIFACTS_DIR = Path(__file__).parent / "artifacts"


# ---------- Baseline: копия реального правила из Go ----------

def baseline_score(intern: InternView, task: TaskView) -> float:
    """Копия реальной логики из internal/api/sprint12.go:scoreTaskAgainst.

    60% — нормализованное tag overlap, 40% — близость difficulty к max level.
    При отсутствии компетенций — cold start на самых простых задачах.
    """
    if not intern.competencies:
        return 1.0 / (task.difficulty + 1)
    intern_tags = set(intern.competencies.keys())
    levels = list(intern.competencies.values())
    max_lvl = max(levels)
    overlap = len(intern_tags & set(task.tags))
    overlap_score = overlap / max(1, len(task.tags))
    diff_score = 1.0 - abs(task.difficulty - max_lvl) / 4.0
    diff_score = max(0.0, diff_score)
    return 0.6 * overlap_score + 0.4 * diff_score


# ---------- Split ----------

@dataclass
class Split:
    X_train: np.ndarray
    y_train: np.ndarray
    X_val: np.ndarray
    y_val: np.ndarray
    X_test: np.ndarray
    y_test: np.ndarray
    groups_train: list[str]
    groups_val: list[str]
    groups_test: list[str]


def group_split(
    X: np.ndarray,
    y: np.ndarray,
    groups: list[str],
    seed: int,
    val_ratio: float = 0.15,
    test_ratio: float = 0.15,
) -> Split:
    """Group-based split: один стажёр попадает только в одну выборку."""
    unique = sorted(set(groups))
    rng = random.Random(seed)
    rng.shuffle(unique)
    n = len(unique)
    n_test = int(round(n * test_ratio))
    n_val = int(round(n * val_ratio))
    test_set = set(unique[:n_test])
    val_set = set(unique[n_test : n_test + n_val])

    def _mask(gset: set[str]) -> np.ndarray:
        return np.array([g in gset for g in groups], dtype=bool)

    train_m = ~_mask(test_set | val_set)
    val_m = _mask(val_set)
    test_m = _mask(test_set)
    return Split(
        X_train=X[train_m], y_train=y[train_m],
        X_val=X[val_m], y_val=y[val_m],
        X_test=X[test_m], y_test=y[test_m],
        groups_train=[g for g, m in zip(groups, train_m) if m],
        groups_val=[g for g, m in zip(groups, val_m) if m],
        groups_test=[g for g, m in zip(groups, test_m) if m],
    )


# ---------- Ranking metrics ----------

def precision_at_k(y_true: np.ndarray, y_score: np.ndarray, k: int = 5) -> float:
    if len(y_true) == 0:
        return 0.0
    order = np.argsort(-y_score)[:k]
    return float(np.mean(y_true[order]))


def recall_at_k(y_true: np.ndarray, y_score: np.ndarray, k: int = 5) -> float:
    pos = int(np.sum(y_true))
    if pos == 0:
        return 0.0
    order = np.argsort(-y_score)[:k]
    return float(np.sum(y_true[order]) / pos)


def dcg(rel: np.ndarray) -> float:
    return float(np.sum((2 ** rel - 1) / np.log2(np.arange(len(rel)) + 2)))


def ndcg_at_k(y_true: np.ndarray, y_score: np.ndarray, k: int = 5) -> float:
    if len(y_true) == 0:
        return 0.0
    order = np.argsort(-y_score)[:k]
    rel = y_true[order]
    ideal = np.sort(y_true)[::-1][:k]
    idcg = dcg(ideal)
    return dcg(rel) / idcg if idcg > 0 else 0.0


def evaluate(
    X: np.ndarray,
    y: np.ndarray,
    groups: list[str],
    score_fn,
    k: int = 5,
) -> dict[str, float]:
    """Считает метрики per-intern и усредняет."""
    # Сгруппируем по стажёрам.
    by_group: dict[str, tuple[np.ndarray, np.ndarray]] = {}
    for i, g in enumerate(groups):
        a, b = by_group.setdefault(g, (np.empty((0, X.shape[1])), np.empty(0)))
        by_group[g] = (
            np.vstack([a, X[i : i + 1]]),
            np.append(b, y[i]),
        )
    p, r, n = [], [], []
    for g, (Xg, yg) in by_group.items():
        if len(yg) == 0:
            continue
        scores = score_fn(Xg)
        p.append(precision_at_k(yg, scores, k=k))
        r.append(recall_at_k(yg, scores, k=k))
        n.append(ndcg_at_k(yg, scores, k=k))
    return {
        f"precision@{k}": float(np.mean(p)) if p else 0.0,
        f"recall@{k}": float(np.mean(r)) if r else 0.0,
        f"ndcg@{k}": float(np.mean(n)) if n else 0.0,
        "n_interns": len(p),
    }


# ---------- Pipeline ----------

def build_dataset_arrays(ds: dict, seed: int) -> tuple[np.ndarray, np.ndarray, list[str]]:
    interns = {i["id"]: InternView(track=i["track"], competencies=i["competencies"]) for i in ds["interns"]}
    tasks = {t["id"]: TaskView(track=t["track"], difficulty=t["difficulty"], tags=t["tags"], popularity=t["popularity"]) for t in ds["tasks"]}
    # Подсчёт prior interactions по стажёру.
    pos_count: dict[str, int] = {iid: 0 for iid in interns}
    neg_count: dict[str, int] = {iid: 0 for iid in interns}
    for inter in ds["interactions"]:
        if inter["label"] == 1:
            pos_count[inter["intern_id"]] += 1
        else:
            neg_count[inter["intern_id"]] += 1

    X, y, groups = [], [], []
    rng = random.Random(seed)
    # Перемешаем, чтобы порядок в выборке был стабилен при seed.
    items = list(ds["interactions"])
    rng.shuffle(items)
    for inter in items:
        intern_id = inter["intern_id"]
        intern = interns.get(intern_id)
        task = tasks.get(inter["task_id"])
        if intern is None or task is None:
            continue
        feats = featurize(intern, task, prior_pos=pos_count[intern_id], prior_neg=neg_count[intern_id])
        X.append(feats)
        y.append(float(inter["label"]))
        groups.append(intern_id)
    return np.array(X, dtype=np.float64), np.array(y, dtype=np.float64), groups


def main(argv: list[str] | None = None) -> int:
    p = argparse.ArgumentParser(description="AI Buddy Hybrid RecSys training")
    p.add_argument("--seed", type=int, default=42)
    p.add_argument("--dataset", type=Path, default=ARTIFACTS_DIR / "dataset.json")
    p.add_argument("--regenerate", action="store_true",
                   help="Перегенерировать датасет вместо чтения с диска")
    p.add_argument("--interns", type=int, default=60)
    p.add_argument("--tasks", type=int, default=50)
    p.add_argument("--interactions", type=int, default=6000)
    p.add_argument("--C", type=float, default=1.0, help="Inverse regularization")
    args = p.parse_args(argv)

    ARTIFACTS_DIR.mkdir(parents=True, exist_ok=True)

    # 1. Датасет.
    if args.regenerate or not args.dataset.exists():
        ds_obj = generate(args.interns, args.tasks, args.interactions, args.seed)
        ds = {
            "interns": [ds_obj.interns[i].__dict__ for i in range(len(ds_obj.interns))],
            "tasks":   [ds_obj.tasks[i].__dict__   for i in range(len(ds_obj.tasks))],
            "interactions": [ds_obj.interactions[i].__dict__ for i in range(len(ds_obj.interactions))],
            "meta": ds_obj.meta,
        }
        args.dataset.parent.mkdir(parents=True, exist_ok=True)
        args.dataset.write_text(json.dumps(ds, ensure_ascii=False, indent=2), encoding="utf-8")
    else:
        ds = json.loads(args.dataset.read_text(encoding="utf-8"))

    # 2. Матрицы.
    X, y, groups = build_dataset_arrays(ds, seed=args.seed)
    if len(X) == 0:
        print("[fatal] пустой датасет", file=sys.stderr)
        return 2
    split = group_split(X, y, groups, seed=args.seed)

    # 3. Baseline.
    def baseline_on_X(Xb: np.ndarray) -> np.ndarray:
        # Используем те же фичи, что и при обучении (tag_overlap_count,
        # tag_overlap_ratio, avg/max competency, task_difficulty).
        # Не используем prior_*, т.к. baseline их не имеет.
        out = np.zeros(len(Xb))
        for i, row in enumerate(Xb):
            tag_overlap_count = row[0]
            tag_overlap_ratio = row[1]
            avg_lvl = row[2]
            max_lvl = row[3]
            difficulty = row[5]
            if max_lvl == 0:
                out[i] = 1.0 / (difficulty + 1)
            else:
                overlap_score = tag_overlap_ratio
                diff_score = max(0.0, 1.0 - abs(difficulty - max_lvl) / 4.0)
                out[i] = 0.6 * overlap_score + 0.4 * diff_score
        return out

    baseline_test = evaluate(split.X_test, split.y_test, split.groups_test, baseline_on_X, k=5)
    print(f"[baseline] test={baseline_test}", file=sys.stderr)

    # 4. Модель.
    model = LogisticRegression(C=args.C, max_iter=200, random_state=args.seed)
    model.fit(split.X_train, split.y_train)

    def model_on_X(Xb: np.ndarray) -> np.ndarray:
        # Возвращаем вероятности положительного класса.
        return model.predict_proba(Xb)[:, 1]

    train_metrics = evaluate(split.X_train, split.y_train, split.groups_train, model_on_X, k=5)
    val_metrics = evaluate(split.X_val, split.y_val, split.groups_val, model_on_X, k=5)
    test_metrics = evaluate(split.X_test, split.y_test, split.groups_test, model_on_X, k=5)

    # 5. Глобальные метрики (AUC / log-loss).
    auc_test = float(roc_auc_score(split.y_test, model_on_X(split.X_test))) if len(set(split.y_test.tolist())) > 1 else 0.0
    ap_test = float(average_precision_score(split.y_test, model_on_X(split.X_test))) if len(set(split.y_test.tolist())) > 1 else 0.0
    ll_test = float(log_loss(split.y_test, np.clip(model_on_X(split.X_test), 1e-6, 1 - 1e-6), labels=[0.0, 1.0])) if len(split.y_test) else 0.0

    # 6. Сохранение.
    artifact = {
        "model": model,
        "feature_names": list(FEATURE_NAMES),
        "coef": model.coef_.tolist(),
        "intercept": model.intercept_.tolist(),
    }
    model_path = ARTIFACTS_DIR / "recsys_lr.joblib"
    joblib.dump(artifact, model_path)

    meta = {
        "trained_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "seed": args.seed,
        "C": args.C,
        "feature_names": list(FEATURE_NAMES),
        "n_train": int(len(split.X_train)),
        "n_val": int(len(split.X_val)),
        "n_test": int(len(split.X_test)),
        "sklearn_version": __import__("sklearn").__version__,
        "source_baseline": "internal/api/sprint12.go:scoreTaskAgainst",
        "dataset_meta": ds.get("meta", {}),
        "notes": "Synthetic dataset. Метрики отражают поведение модели на синтетике, не на реальных данных Avito.",
    }
    (ARTIFACTS_DIR / "meta.json").write_text(json.dumps(meta, ensure_ascii=False, indent=2), encoding="utf-8")

    metrics = {
        "baseline": {"test": baseline_test},
        "model": {
            "train": train_metrics,
            "val": val_metrics,
            "test": test_metrics,
            "roc_auc_test": auc_test,
            "average_precision_test": ap_test,
            "log_loss_test": ll_test,
        },
        "improvement_p_at_5_vs_baseline": (
            test_metrics["precision@5"] - baseline_test["precision@5"]
        ),
    }
    (ARTIFACTS_DIR / "metrics.json").write_text(json.dumps(metrics, ensure_ascii=False, indent=2), encoding="utf-8")

    print(f"[model] train={train_metrics}", file=sys.stderr)
    print(f"[model] val={val_metrics}", file=sys.stderr)
    print(f"[model] test={test_metrics}", file=sys.stderr)
    print(f"[model] test roc_auc={auc_test:.4f} ap={ap_test:.4f} log_loss={ll_test:.4f}", file=sys.stderr)
    print(f"[ok] artifact -> {model_path}", file=sys.stderr)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
