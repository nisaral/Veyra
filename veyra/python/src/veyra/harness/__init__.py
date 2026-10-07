"""Harness adapters. Each one maps the portable execution state to its own loop."""

from veyra.harness.base import HarnessAdapter, HarnessError

__all__ = ["HarnessAdapter", "HarnessError"]