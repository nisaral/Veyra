"""Adapters for MCP, HTTP, and Python execution targets."""

from veyra.adapters.mcp import MCPAdapter
from veyra.adapters.http import HTTPAdapter
from veyra.adapters.python import PythonAdapter

__all__ = ["MCPAdapter", "HTTPAdapter", "PythonAdapter"]
