"""
generate_dataset.py — Synthetic dataset generator for AI Buddy RecSys.

Цель: создать реалистичный воспроизводимый синтетический датасет
(interns, tasks, competencies, interactions) для обучения модели
персональных рекомендаций задач стажёрам Backend-команды Авито.

Источник данных: нет реальных данных Avito.
Используется синтетика с фиксированным seed, отражающая реальную
структуру канонической БД (см. docs/architecture/DATABASE_SCHEMA.md,
internal/store/entities.go).

ВАЖНО: каждый синтетический лейбл отмечен как synthetic. Метрики
на синтетике не доказывают реальный продуктовый эффект.

Запуск:
    python -m ml.recsys.generate_dataset \\
        --out ml/recsys/artifacts/dataset.json \\
        --interns 60 --tasks 50 --interactions 6000 --seed 42
"""

from __future__ import annotations

import argparse
import json
import math
import random
import sys
import time
from dataclasses import asdict, dataclass, field
from pathlib import Path


# ---------- Domain entities (1:1 с internal/store/entities.go) ----------

TRACKS = ("backend", "data", "devops", "frontend", "ml")
TAGS_POOL = (
    "go", "chi", "postgres", "kafka", "grpc", "docker", "k8s",
    "redis", "metrics", "observability", "sql", "linux", "ci",
    "review", "testing", "python", "vector", "embeddings", "rag",
)
LEVELS = (1, 2, 3, 4, 5)


@dataclass
class Intern:
    id: str
    track: str
    competencies: dict[str, int] = field(default_factory=dict)  # tag -> level


@dataclass
class Task:
    id: str
    title: str
    track: str
    difficulty: int
    tags: list[str]
    popularity: int  # сколько стажёров уже выполнили


@dataclass
class Interaction:
    intern_id: str
    task_id: str
    # 1 = позитивное взаимодействие (выполнил/полезно),
    # 0 = негативное (отклонил/пропустил).
    label: int


@dataclass
class Dataset:
    interns: list[Intern]
    tasks: list[Task]
    interactions: list[Interaction]
    meta: dict


# ---------- Public label constants (used by experiment_v2.py) ----------

POS_LABEL = 1
NEG_LABEL = 0


def compute_completion_probability(intern: Intern, task: Task, rng: random.Random | None = None) -> float:
    """Pure deterministic completion probability (no sampling).

    Used by experiment_v2.py as the *true* relevance signal in the synthetic
    world — same functional form as make_interaction's logit, but returns the
    raw probability so the experiment can drive both label generation and a
    latent-quality baseline.

    logit = +1.2 * skill_gap
          + 0.9 * overlap_ratio
          + 0.5 * track_match
          - 0.6 * difficulty_penalty

    Returns the sigmoid of that logit.
    """
    intern_tags = set(intern.competencies.keys())
    intern_levels = list(intern.competencies.values())
    avg_level = sum(intern_levels) / max(1, len(intern_levels))

    overlap = len(intern_tags & set(task.tags))
    overlap_ratio = overlap / max(1, len(task.tags))

    skill_gap = (task.difficulty - avg_level) / 4.0
    difficulty_penalty = max(0.0, task.difficulty - avg_level - 1.0)

    track_match = 1.0 if task.track == intern.track else 0.0

    logit = (
        1.2 * skill_gap
        + 0.9 * overlap_ratio
        + 0.5 * track_match
        - 0.6 * difficulty_penalty
    )
    return 1.0 / (1.0 + math.exp(-logit))


# ---------- Realistic generative model ----------

def make_intern(i: int, rng: random.Random) -> Intern:
    track = rng.choice(TRACKS)
    # Каждый стажёр сильнее в 2-4 тегах своего трека.
    track_tags = [t for t in TAGS_POOL if t in _track_tags(track)]
    strong = rng.sample(track_tags, k=min(rng.randint(2, 4), len(track_tags)))
    # Уровень компетенции от 1 до 5, чаще низкий у стажёров.
    comps = {t: rng.randint(1, 4) for t in strong}
    # Иногда есть побочный навык.
    if rng.random() < 0.3:
        comps[rng.choice(TAGS_POOL)] = rng.randint(1, 3)
    return Intern(id=f"intern-{i:03d}", track=track, competencies=comps)


def _track_tags(track: str) -> set[str]:
    return {
        "backend": {"go", "chi", "grpc", "postgres", "kafka", "redis", "sql", "linux"},
        "data":    {"python", "sql", "postgres", "metrics", "observability"},
        "devops":  {"docker", "k8s", "ci", "linux", "observability", "metrics"},
        "frontend":{"review", "testing"},
        "ml":      {"python", "vector", "embeddings", "rag"},
    }.get(track, set())


def make_task(i: int, rng: random.Random) -> Task:
    track = rng.choice(TRACKS)
    tags = rng.sample(list(TAGS_POOL), k=rng.randint(2, 4))
    difficulty = rng.randint(1, 5)
    return Task(
        id=f"task-{i:03d}",
        title=f"{track.title()} task #{i}",
        track=track,
        difficulty=difficulty,
        tags=tags,
        popularity=rng.randint(0, 30),
    )


def make_interaction(intern: Intern, task: Task, rng: random.Random) -> Interaction:
    """Генерирует метку по реалистичной, воспроизводимой функции полезности.

    P(positive) = sigmoid(
        +1.2 * skill_gap           (задача чуть выше текущего уровня)
        +0.9 * tag_overlap_ratio   (пересечение тегов стажёра и задачи)
        +0.5 * track_match         (совпадение трека)
        -0.6 * difficulty_penalty  (слишком сложно — плохо)
        -0.4 * noise               (случайный шум)
    )
    """
    intern_tags = set(intern.competencies.keys())
    intern_levels = list(intern.competencies.values())
    avg_level = sum(intern_levels) / max(1, len(intern_levels))

    overlap = len(intern_tags & set(task.tags))
    overlap_ratio = overlap / max(1, len(task.tags))

    skill_gap = (task.difficulty - avg_level) / 4.0  # ~[-1, +1]
    difficulty_penalty = max(0.0, task.difficulty - avg_level - 1.0)

    track_match = 1.0 if task.track == intern.track else 0.0
    noise = rng.gauss(0, 0.4)

    logit = (
        1.2 * skill_gap
        + 0.9 * overlap_ratio
        + 0.5 * track_match
        - 0.6 * difficulty_penalty
        - 0.4 * (1.0 if track_match == 0.0 else 0.0) * abs(noise)
    )
    p = 1.0 / (1.0 + math.exp(-logit))
    label = POS_LABEL if rng.random() < p else NEG_LABEL
    return Interaction(intern_id=intern.id, task_id=task.id, label=label)


# ---------- Convenience builder for experiment_v2.py ----------

def build_dataset(seed: int = 42, n_interns: int = 60, n_tasks: int = 50, n: int = 6000) -> dict:
    """Build the synthetic dataset and return a plain dict with the schema
    expected by experiment_v2.py and any downstream consumer.

    Schema:
      {
        "interns":      [{"id", "track", "competencies": {tag: level}}],
        "tasks":        [{"id", "track", "difficulty", "tags", "popularity", "title"}],
        "interactions": [{"intern_id", "task_id", "label"}],
        "meta":         {"seed", "synthetic", "size": {...}, ...},
      }
    """
    ds = generate(num_interns=n_interns, num_tasks=n_tasks, num_interactions=n, seed=seed)
    return {
        "interns": [asdict(i) for i in ds.interns],
        "tasks": [asdict(t) for t in ds.tasks],
        "interactions": [asdict(x) for x in ds.interactions],
        "meta": ds.meta,
    }


# ---------- Main ----------

def generate(num_interns: int, num_tasks: int, num_interactions: int, seed: int) -> Dataset:
    rng = random.Random(seed)
    interns = [make_intern(i, rng) for i in range(num_interns)]
    tasks = [make_task(i, rng) for i in range(num_tasks)]
    interactions: list[Interaction] = []
    for _ in range(num_interactions):
        intern = rng.choice(interns)
        task = rng.choice(tasks)
        interactions.append(make_interaction(intern, task, rng))
    meta = {
        "generated_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "seed": seed,
        "synthetic": True,
        "size": {"interns": len(interns), "tasks": len(tasks), "interactions": len(interactions)},
        "schema_ref": "internal/store/entities.go (InternProfile, Task, Interaction)",
    }
    return Dataset(interns=interns, tasks=tasks, interactions=interactions, meta=meta)


def main(argv: list[str] | None = None) -> int:
    p = argparse.ArgumentParser(description="AI Buddy synthetic RecSys dataset generator")
    p.add_argument("--out", required=True, type=Path)
    p.add_argument("--interns", type=int, default=60)
    p.add_argument("--tasks", type=int, default=50)
    p.add_argument("--interactions", type=int, default=6000)
    p.add_argument("--seed", type=int, default=42)
    args = p.parse_args(argv)

    ds = generate(args.interns, args.tasks, args.interactions, args.seed)
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(
        json.dumps(
            {
                "interns": [asdict(i) for i in ds.interns],
                "tasks": [asdict(t) for t in ds.tasks],
                "interactions": [asdict(x) for x in ds.interactions],
                "meta": ds.meta,
            },
            ensure_ascii=False,
            indent=2,
        ),
        encoding="utf-8",
    )
    print(f"[ok] dataset -> {args.out} "
          f"({ds.meta['size']['interns']} interns, "
          f"{ds.meta['size']['tasks']} tasks, "
          f"{ds.meta['size']['interactions']} interactions, "
          f"seed={args.seed})", file=sys.stderr)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
