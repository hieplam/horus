// Package workflows holds the built-in workflows: the workflow folders of
// this repo, put into the program by the build (go:embed). internal/settings
// copies them to <user folder>/builtin-workflows when the program runs
// without a clone of the repo.
//
// The program embeds each workflow folder in full: manifest.json,
// workflow.json, models.json, README.md, recipe.json and examples/. The tool
// describe_workflow returns the path of an example image, so the images are
// in the program too. The folder _template is not embedded: only
// horus new-workflow reads it, and that command works in a clone.
//
// A new workflow folder must be added to the go:embed line below. The test
// TestEveryWorkflowFolderIsEmbedded fails when one is missing.
package workflows

import "embed"

// FS holds the built-in workflows. Its top folders are the workflow names.
//
//go:embed it-illustration vi-speech
var FS embed.FS
