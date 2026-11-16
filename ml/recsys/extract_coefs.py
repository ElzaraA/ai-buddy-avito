"""
extract_coefs.py — извлечь коэффициенты Logistic Regression из joblib-артефакта
в чистый JSON-файл `coefs.json`. Это позволяет Go-native inference читать
параметры модели без зависимости от Python / sklearn / joblib в runtime.

Запуск:
    python -m ml.recsys.extract_coefs

Вход:  ml/recsys/artifacts/recsys_lr.joblib
Выход: ml/recsys/artifacts/coefs.json
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

ARTIFACTS_DIR = Path(__file__).resolve().parent / "artifacts"


def main() -> int:
    joblib_path = ARTIFACTS_DIR / "recsys_lr.joblib"
    out_path = ARTIFACTS_DIR / "coefs.json"

    if not joblib_path.exists():
        print(f"FAIL: missing {joblib_path}", file=sys.stderr)
        return 1

    try:
        import joblib  # noqa: WPS433 — намеренно локальный импорт
    except ImportError as exc:
        print(f"FAIL: joblib not available: {exc}", file=sys.stderr)
        return 1

    artifact = joblib.load(joblib_path)
    model = artifact["model"]
    coef = artifact["coef"][0] if hasattr(artifact["coef"][0], "__iter__") else artifact["coef"]
    intercept = float(artifact["intercept"][0]) if hasattr(artifact["intercept"], "__iter__") else float(artifact["intercept"])

    payload = {
        "feature_names": list(artifact["feature_names"]),
        "coef": [float(x) for x in coef],
        "intercept": intercept,
        "n_features": len(coef),
    }
    out_path.write_text(json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8")
    print(f"[ok] coefs -> {out_path} (n_features={payload['n_features']})", file=sys.stderr)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
