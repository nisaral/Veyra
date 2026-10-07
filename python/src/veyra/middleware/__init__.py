"""Veyra Middleware package for wrapping functions, MCP servers, HTTP clients, and AI agent frameworks."""

from veyra.middleware.function import FunctionWrapper
from veyra.middleware.mcp import MCPWrapper, VeyraMCPProxy
from veyra.middleware.http import HTTPWrapper
from veyra.middleware.agent import AgentWrapper

__all__ = [
    "FunctionWrapper",
    "MCPWrapper",
    "VeyraMCPProxy",
    "HTTPWrapper",
    "AgentWrapper",
]
