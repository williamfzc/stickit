#!/usr/bin/env python3
"""Validate the docs/ tree: frontmatter, index registration, link integrity."""
import re
import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[1]
KNOWLEDGE_DIR = REPO_ROOT / "docs"
REQUIRED_FIELDS = ("type", "title", "description")

# Optional: expected type per directory. Empty because docs/ holds several
# types (Index, Concept, Log) at its root.
DIR_TYPES = {}

MD_LINK = re.compile(r"\[[^\]]*\]\(([^)]+)\)")


def frontmatter(text):
    if not text.startswith("---"):
        return None, "missing opening frontmatter delimiter (---)"
    parts = text.split("---", 2)
    if len(parts) < 3:
        return None, "missing closing frontmatter delimiter (---)"
    return parts[1], None


def check_fields(rel, path, fm_text, yaml_mod):
    if yaml_mod is None:
        return []
    try:
        data = yaml_mod.safe_load(fm_text)
    except yaml_mod.YAMLError as exc:
        return [f"{rel}: invalid YAML: {str(exc)[:120]}"]
    if not isinstance(data, dict):
        return [f"{rel}: frontmatter is not a mapping"]

    errors = [f"{rel}: missing required field '{f}'"
              for f in REQUIRED_FIELDS if not data.get(f)]

    expected = DIR_TYPES.get(path.parent.name)
    if expected and path.name != "index.md" and data.get("type") != expected:
        errors.append(
            f"{rel}: documents in {path.parent.name}/ must be type {expected}"
        )
    return errors


def check_links(rel, path, text):
    errors = []
    for target in MD_LINK.findall(text):
        target = target.strip()
        if not target or target.startswith(("http://", "https://", "#", "mailto:")):
            continue
        if not (path.parent / target.split("#")[0]).resolve().exists():
            errors.append(f"{rel}: broken link -> {target}")
    return errors


def check_indexes():
    errors = []
    directories = [KNOWLEDGE_DIR] + sorted(
        p for p in KNOWLEDGE_DIR.rglob("*") if p.is_dir()
    )
    for directory in directories:
        pages = [p for p in directory.glob("*.md") if p.name != "index.md"]
        if not pages:
            continue
        index = directory / "index.md"
        rel_dir = directory.relative_to(REPO_ROOT)
        if not index.exists():
            errors.append(f"{rel_dir}/: has {len(pages)} page(s) but no index.md")
            continue
        listed = index.read_text(encoding="utf-8")
        errors.extend(
            f"{index.relative_to(REPO_ROOT)}: does not register {p.name}"
            for p in sorted(pages) if p.name not in listed
        )
    return errors


def main():
    if not KNOWLEDGE_DIR.exists():
        print(f"WARN: {KNOWLEDGE_DIR.name}/ not found.")
        return 0
    try:
        import yaml
    except ImportError:
        yaml = None
        print("WARN: PyYAML missing; frontmatter fields not validated.")

    errors = []
    files = sorted(KNOWLEDGE_DIR.rglob("*.md"))
    for path in files:
        rel = path.relative_to(REPO_ROOT)
        text = path.read_text(encoding="utf-8")
        fm_text, err = frontmatter(text)
        if err:
            errors.append(f"{rel}: {err}")
        else:
            errors.extend(check_fields(rel, path, fm_text, yaml))
        errors.extend(check_links(rel, path, text))
    errors.extend(check_indexes())

    if errors:
        for error in errors:
            print(f"ERROR: {error}")
        print(f"Validation failed ({len(errors)} error(s)).")
        return 1
    print(f"Validation passed ({len(files)} files verified).")
    return 0


if __name__ == "__main__":
    sys.exit(main())
