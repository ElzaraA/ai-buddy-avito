# Рекомендации задач адаптации (RecSys)

Модель подбирает стажёру следующую задачу из каталога команды (`data/tasks/catalog.json`).
Обучение офлайн на Python, в продукте скоринг считает Go (`internal/recsys`) по экспортированным весам.
Python в рантайме не нужен.

## Что внутри

| Файл | Назначение |
|---|---|
| `generate_dataset.py` | синтетический датасет: 60 стажёров, 50 задач, 6000 взаимодействий (seed 42) |
| `features.py` | 11 признаков пары «стажёр — задача»: пересечение технологий, уровни компетенций, совпадение трека, сложность и разрыв с уровнем, популярность, история стажёра |
| `train.py` | логистическая регрессия (sklearn), разбиение 70/15/15, метрики precision@5, recall@5, NDCG@5, ROC-AUC |
| `extract_coefs.py` | экспорт весов в JSON → в продукте `data/recsys/model.json` |
| `experiment_v2.py`, `ranking_eval.py`, `dataset_split.py`, `features_v2.py`, `train_best_model.py` | сравнение моделей без утечки: случайная, популярность, правило, LR, CatBoost, ALS, гибрид CatBoost-ranker |
| `artifacts/` | веса (`coefs.json`, `recsys_lr.joblib`), метрики (`metrics.json`, `experiment_v2_table.md`), датасет |

## Результаты (честно)

Сравнение моделей на одном разбиении без утечки (`artifacts/experiment_v2_table.md`, тестовая выборка):

| Модель | NDCG@5 | P@5 | MRR |
|---|---:|---:|---:|
| случайная | 0.517 | 0.485 | 0.663 |
| популярность | 0.576 | 0.532 | 0.727 |
| правило (трек + технологии) | 0.592 | 0.555 | 0.743 |
| **логистическая регрессия** | **0.621** | **0.562** | **0.809** |
| CatBoost | 0.627 | 0.568 | 0.768 |
| ALS (коллаборативная) | 0.551 | 0.505 | 0.705 |

Логистическая регрессия заметно лучше случайной и популярности и чуть лучше правила; CatBoost даёт сопоставимое
качество, но LR проще объяснить и считать в Go без зависимостей. На синтетике отрыв небольшой — это ожидаемо:
закономерности в данных сгенерированы. На реальной истории задач стажёров Авито модель переобучается тем же пайплайном.

## Как это работает в продукте

1. `GET /api/v1/recommendations` — для стажёра: кандидаты = задачи своей команды, которые он ещё не брал.
2. Профиль стажёра = стартовые компетенции из каталога + **+1 к каждой технологии сделанной задачи** (до 5):
   сделал задачу — рекомендации меняются.
3. История стажёра (сделано / отложено) идёт в признаки `intern_prior_pos/neg`.
4. Ответ содержит `source`: `model` (веса загружены) или `rule_based` (весов нет — прозрачное правило), и для каждой
   задачи — объяснение человеческими словами, построенное на тех же признаках.

## Совпадение с Go

`internal/recsys/recsys_test.go::TestScoreMatchesSklearn` сверяет оценки Go с `predict_proba` настоящей sklearn-модели
(`artifacts/recsys_lr.joblib`) на трёх входах: расхождение меньше 1e-6. Эталонные значения получены так:

```python
import joblib; from features import featurize, InternView, TaskView
m = joblib.load('artifacts/recsys_lr.joblib')['model']
m.predict_proba([featurize(InternView('backend', {'go': 2, 'sql': 1, 'docker': 1, 'testing': 1}),
                           TaskView('backend', 2, ['go', 'testing', 'review'], 22))])[0, 1]  # 0.747144
```

## Переобучение

```bash
pip install scikit-learn joblib numpy
python ml/recsys/generate_dataset.py   # или реальные данные в том же формате
python ml/recsys/train.py
python ml/recsys/extract_coefs.py && cp ml/recsys/artifacts/coefs.json data/recsys/model.json
go test ./internal/recsys/            # обновите эталон в TestScoreMatchesSklearn
```

Данные синтетические и не являются данными Авито.
