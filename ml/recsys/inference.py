"""
inference.py — загрузка артефакта и скоринг кандидатов.

Этот модуль вызывается из Go через ml/recsys/inference_bridge.py
или напрямую из CLI для sanity-check. Использует ТУ ЖЕ featurize(),
что и train.py.
"""

from __future__ import annotations

import argparse
import json
import sys
from dataclasses import dataclass
from pathlib import Path

import joblib
import numpy as np

if __package__:
    from .features import InternView, TaskView, featurize
else:
    sys.path.insert(0, str(Path(__file__).resolve().parent.parent.parent))
    from ml.recsys.features import InternView, TaskView, featurize  # type: ignore


ARTIFACTS_DIR = Path(__file__).parent / "artifacts"


@dataclass
class ScoredItem:
    task_id: str
    score: float
    source: str  # hybrid | cbf | cold_start


class RecsysInference:
    def __init__(self, artifact_path: Path = ARTIFACTS_DIR / "recsys_lr.joblib") -> None:
        art = joblib.load(artifact_path)
        self.model = art["model"]
        self.feature_names = art["feature_names"]
        self._loaded_path = artifact_path

    def score(self, intern: InternView, task: TaskView,
              prior_pos: int = 0, prior_neg: int = 0) -> float:
        feats = featurize(intern, task, prior_pos=prior_pos, prior_neg=prior_neg)
        proba = self.model.predict_proba(np.array([feats], dtype=np.float64))[0, 1]
        return float(proba)

    def rank(self,
             intern: InternView,
             tasks: list[tuple[str, TaskView]],
             prior_pos: int = 0,
             prior_neg: int = 0,
             top_k: int = 5) -> list[ScoredItem]:
        scored: list[ScoredItem] = []
        for tid, task in tasks:
            s = self.score(intern, task, prior_pos=prior_pos, prior_neg=prior_neg)
            scored.append(ScoredItem(task_id=tid, score=s, source="hybrid"))
        scored.sort(key=lambda x: x.score, reverse=True)
        if not intern.competencies:
            for s in scored:
                s.source = "cold_start"
        return scored[:top_k]


def main(argv: list[str] | None = None) -> int:
    p = argparse.ArgumentParser(description="AI Buddy RecSys inference (smoke test)")
    p.add_argument("--artifact", type=Path, default=ARTIFACTS_DIR / "recsys_lr.joblib")
    p.add_argument("--json", action="store_true", help="JSON output")
    args = p.parse_args(argv)

    inf = RecsysInference(args.artifact)
    print(json.dumps({"loaded": str(args.artifact), "n_features": len(inf.feature_names)},
                     ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
