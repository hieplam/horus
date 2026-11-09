"""horus: make images with local workflows, for people and for AI agents.

Modules:
    errors    HorusError and the error codes that every caller sees.
    settings  Repo root, .env settings, folders.
    catalog   The workflows in workflows/<name>/ and their manifest.json.
    models    Install, check and remove model files (wraps scripts/workflow.ps1).
    comfy     The HTTP client for the local ComfyUI, and the UI-to-API graph conversion.
    generate  generate_image(): one image from one workflow.
    server    The MCP server (stdio). Needs the `mcp` package; the other modules need only the standard library.
"""

__version__ = "0.1.0"
