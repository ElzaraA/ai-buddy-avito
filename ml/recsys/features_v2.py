"""
features_v2.py — Honest, leak-free feature engineering for RecSys.

KEY DIFFERENCES vs features.py:
  - All features are computed as if at a SINGLE point in time BEFORE the
    candidate (intern, task) pair is observed.
  - No feature uses the row's own label.
  - Counts are based on a fixed "history" partition (deterministic),
    not the row being scored.

Feature schema (numeric, dense):
  intern_track_backend, intern_track_frontend, intern_track_data, intern_track_devops
  num_competencies (how many skills have any level)
  avg_competency_level
  max_competency_level
  num_completed_tasks
  task_track_backend, task_track_frontend, task_track_data, task_track_devops
  task_difficulty
  task_popularity
  task_num_tags
  tag_overlap (|task.tags ∩ intern.competencies|)
  tag_overlap_ratio (|∩| / |task.tags|)
  avg_competency_match (mean of intern level for matched tags; 0 if no match)
  max_competency_match
  difficulty_gap (task.difficulty - avg_competency_level)
  same_track (1 if intern.track == task.track)
  popularity_normalized (popularity / max_popularity)
"""

from __future__ import annotations

from collections import Counter
from typing import Iterable

TRACKS = ("backend", "frontend", "data", "devops")


def _one_hot_track(track: str) -> tuple[int, int, int, int]:
    return tuple(1 if track == t else 0 for t in TRACKS)


def featurize_pair(
    intern: dict,
    task: dict,
    intern_completed_count: int,
    popularity_max: int,
) -> list[float]:
    """Compute feature vector for (intern, task) candidate pair.

    Both `intern` and `task` are dicts matching the dataset schema:
      intern: {id, track, competencies: {skill: level}, completed_tasks: [...]}
      task:   {id, track, difficulty, popularity, tags: [...], competencies_required: [...]}

    `intern_completed_count` and `popularity_max` are pre-computed constants
    (not per-row), so they do not leak.
    """
    comp = intern.get("competencies", {}) or {}
    intern_track = intern.get("track", "")
    task_track = task.get("track", "")
    tags = task.get("tags", []) or []
    comp_keys = set(comp.keys())
    tag_set = set(tags)

    matched_tags = comp_keys & tag_set
    match_levels = [comp[t] for t in matched_tags if isinstance(comp.get(t), int)]

    avg_competency_level = (sum(comp.values()) / len(comp)) if comp else 0.0
    max_competency_level = max(comp.values()) if comp else 0
    num_competencies = len(comp)

    if popularity_max <= 0:
        popularity_max = 1
    popularity_norm = float(task.get("popularity", 0)) / popularity_max

    feats: list[float] = []
    feats.extend(_one_hot_track(intern_track))
    feats.extend([
        float(num_competencies),
        float(avg_competency_level),
        float(max_competency_level),
        float(intern_completed_count),
    ])
    feats.extend(_one_hot_track(task_track))
    feats.extend([
        float(task.get("difficulty", 0)),
        float(task.get("popularity", 0)),
        float(len(tags)),
        float(len(matched_tags)),
        float(len(matched_tags) / len(tags)) if tags else 0.0,
        float(sum(match_levels) / len(match_levels)) if match_levels else 0.0,
        float(max(match_levels)) if match_levels else 0.0,
        float(task.get("difficulty", 0) - avg_competency_level),
        float(1 if intern_track == task_track and intern_track else 0),
        float(popularity_norm),
    ])

    return feats


FEATURE_NAMES: list[str] = [
    "intern_track_backend", "intern_track_frontend", "intern_track_data", "intern_track_devops",
    "num_competencies", "avg_competency_level", "max_competency_level", "num_completed_tasks",
    "task_track_backend", "task_track_frontend", "task_track_data", "task_track_devops",
    "task_difficulty", "task_popularity", "task_num_tags",
    "tag_overlap", "tag_overlap_ratio", "avg_competency_match", "max_competency_match",
    "difficulty_gap", "same_track", "popularity_normalized",
]
assert len(FEATURE_NAMES) == 22, f"feature count mismatch: {len(FEATURE_NAMES)}"
