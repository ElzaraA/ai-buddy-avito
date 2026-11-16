"""
features.py — единый feature extractor для training и inference.

Контракт: train.py и inference.py используют ОДНУ И ТУ ЖЕ функцию
featurize() с ОДНИМ И ТЕМ ЖЕ порядком признаков (FEATURE_NAMES).
Любое расхождение сломает воспроизводимость и будет видно в тестах.
"""

from __future__ import annotations

from dataclasses import dataclass


FEATURE_NAMES: list[str] = [
    # --- Content-based (5 фичей, пересечение со scoreTaskAgainst) ---
    "tag_overlap_count",      # |intern_tags ∩ task_tags|
    "tag_overlap_ratio",      # |∩| / |task_tags|
    "avg_competency_level",   # средний уровень компетенций стажёра
    "max_competency_level",   # максимальный уровень компетенций
    "track_match",            # 1.0 если track совпадает, иначе 0.0
    # --- Difficulty (3 фичи) ---
    "task_difficulty",        # 1..5
    "skill_gap",              # difficulty - avg_competency_level
    "difficulty_penalty",     # max(0, difficulty - avg - 1)
    # --- Behavioral / popularity (3 фичи) ---
    "task_popularity",        # сколько уже выполнили
    "intern_prior_pos",       # сколько позитивных взаимодействий у стажёра
    "intern_prior_neg",       # сколько негативных
]


@dataclass(frozen=True)
class InternView:
    track: str
    competencies: dict[str, int]


@dataclass(frozen=True)
class TaskView:
    track: str
    difficulty: int
    tags: list[str]
    popularity: int


def featurize(
    intern: InternView,
    task: TaskView,
    prior_pos: int = 0,
    prior_neg: int = 0,
) -> list[float]:
    """Возвращает список фичей ровно в порядке FEATURE_NAMES.

    Этот список должен детерминированно соответствовать FEATURE_NAMES,
    иначе model и inference перестанут согласовываться.
    """
    intern_tags = set(intern.competencies.keys())
    levels = list(intern.competencies.values())
    avg_lvl = sum(levels) / len(levels) if levels else 0.0
    max_lvl = max(levels) if levels else 0
    overlap = len(intern_tags & set(task.tags))
    overlap_ratio = overlap / max(1, len(task.tags))
    skill_gap = task.difficulty - avg_lvl
    difficulty_penalty = max(0.0, task.difficulty - avg_lvl - 1.0)
    track_match = 1.0 if intern.track == task.track else 0.0

    return [
        float(overlap),
        overlap_ratio,
        avg_lvl,
        float(max_lvl),
        track_match,
        float(task.difficulty),
        skill_gap,
        difficulty_penalty,
        float(task.popularity),
        float(prior_pos),
        float(prior_neg),
    ]


def feature_names() -> list[str]:
    return list(FEATURE_NAMES)


def build_views(ds: dict) -> tuple[dict[str, InternView], dict[str, TaskView]]:
    """Convert raw dataset dicts (from generate()) into InternView/TaskView maps.

    Expected schema for ds["interns"]: [{id, track, competencies: {tag: lvl}}, ...]
    Expected schema for ds["tasks"]:   [{id, track, difficulty, tags, popularity}, ...]
    """
    interns: dict[str, InternView] = {}
    for i in ds.get("interns", []):
        interns[i["id"]] = InternView(
            track=i["track"],
            competencies=dict(i.get("competencies") or {}),
        )
    tasks: dict[str, TaskView] = {}
    for t in ds.get("tasks", []):
        tasks[t["id"]] = TaskView(
            track=t["track"],
            difficulty=int(t["difficulty"]),
            tags=list(t.get("tags") or []),
            popularity=int(t.get("popularity", 0)),
        )
    return interns, tasks
