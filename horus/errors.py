"""The error model of horus.

Every failure is a HorusError with a stable `code`. A caller (a person or an AI
agent) decides what to do from three fields, without reading the message:

    retryable   true: the same call can pass when it is sent again.
    action_by   who must act before the next call can pass:
                  "caller"  change the arguments (for example a smaller size),
                  "retry"   send the same call again,
                  "human"   a person must do something on this machine
                            (accept a license, set a token, free disk space).
    next_step   one sentence: the exact thing to do next.

`details` holds the facts (file names, allowed values, the backend message).
The table ERRORS is the single list of codes. docs/mcp.md shows the same table.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any

# code -> (retryable, action_by, default next_step)
ERRORS: dict[str, tuple[bool, str, str]] = {
    "INVALID_ARGUMENT": (False, "caller", "Fix the argument named in details.field, then call again."),
    "WORKFLOW_NOT_FOUND": (False, "caller", "Call list_workflows and use one of the names in details.known."),
    "WORKFLOW_NOT_INSTALLED": (False, "caller", "Call install_workflow for this workflow, then call generate_image again."),
    "LICENSE_CONFIRMATION_REQUIRED": (
        False, "human",
        "Show the license and the download size to the user. When the user agrees, call install_workflow again with accept_license=true.",
    ),
    "CONFIRMATION_REQUIRED": (
        False, "human",
        "Show the plan in details.files to the user. When the user agrees, call again with confirm=true.",
    ),
    "OUTPUT_EXISTS": (False, "caller", "Pass another out_path, or pass overwrite=true."),
    "BACKEND_UNAVAILABLE": (
        True, "human",
        "The local image backend (ComfyUI) does not answer and horus could not start it. Ask the user to run scripts/start-comfyui.ps1 in the horus repo and read docs/setup.md.",
    ),
    "BACKEND_NOT_INSTALLED": (False, "human", "Ask the user to set up horus on this machine: docs/setup.md in the horus repo."),
    "BACKEND_RESTART_REQUIRED": (
        True, "human",
        "The image backend must restart to see new model files. Ask the user to restart ComfyUI, then call again.",
    ),
    "NODE_PACK_MISSING": (False, "human", "The backend lacks a node pack that this workflow needs. Ask the user to install it (see details.missing)."),
    "HF_TOKEN_INVALID": (
        False, "human",
        "The Hugging Face token is missing or wrong. Ask the user to put a valid Read token in HF_TOKEN in the .env file of the horus repo.",
    ),
    "LICENSE_NOT_ACCEPTED": (
        False, "human",
        "The Hugging Face account has not accepted the model license. Ask the user to open details.license_url, accept the license, then call again.",
    ),
    "DISK_FULL": (False, "human", "Not enough free disk space. Ask the user to free the space in details, then call again."),
    "FILE_CONFLICT": (
        False, "human",
        "A different file with the same name is on disk. horus never overwrites it. Ask the user to move or delete details.path.",
    ),
    "FILE_IN_USE": (
        True, "human",
        "A model file is open in the image backend and Windows does not let horus delete it. Ask the user to stop ComfyUI, then call again.",
    ),
    "DOWNLOAD_FAILED": (True, "retry", "Call install_workflow again. It resumes the partial download."),
    "OUT_OF_MEMORY": (True, "caller", "Call again. If it fails again, use a smaller width and height."),
    "GENERATION_FAILED": (True, "retry", "Call generate_image again with the same arguments. If it fails twice, report details.backend_message to the user."),
    "GENERATION_TIMEOUT": (True, "retry", "The job took longer than the time limit and horus cancelled it. Call again when the machine is less busy."),
    "CANCELLED": (True, "retry", "The job was cancelled on the backend. Call again if you still need the image."),
    "INTERNAL": (False, "human", "This is a bug in horus. Report details to the user."),
}


@dataclass
class HorusError(Exception):
    code: str
    message: str
    details: dict[str, Any] = field(default_factory=dict)
    next_step: str | None = None
    retryable: bool | None = None

    def __post_init__(self) -> None:
        if self.code not in ERRORS:
            raise ValueError(f"unknown error code {self.code}")
        super().__init__(self.message)

    def to_dict(self) -> dict[str, Any]:
        retryable, action_by, step = ERRORS[self.code]
        return {
            "code": self.code,
            "message": self.message,
            "retryable": retryable if self.retryable is None else self.retryable,
            "action_by": action_by,
            "next_step": self.next_step or step,
            "details": self.details,
        }

    def __str__(self) -> str:
        return f"{self.code}: {self.message}"
