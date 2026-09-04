# AI-assisted commit messages (OrcaRouter)

`gito commit` can optionally generate a Conventional Commit suggestion from the **staged diff** using OrcaRouter.

This feature is opt-in. Existing local/manual Git workflows are unchanged when no OrcaRouter API key is configured.

## Setup

```bash
export ORCAROUTER_API_KEY="..."
```

Optional overrides:

```bash
export GITO_AI_MODEL="orcarouter/auto"
export ORCAROUTER_BASE_URL="https://api.orcarouter.ai/v1"
```

## Usage

Stage the exact changes you want the model to inspect, then run:

```bash
git add <files>
gito commit
```

On the commit type screen press `a` to generate a suggestion. gito fills the commit type, scope, subject, and body, then shows the normal confirmation screen.

- `y` / `enter`: commit the suggestion
- `e`: edit it manually
- `r`: regenerate with OrcaRouter
- `n`: cancel

## Privacy and cost

Only the output of `git diff --cached` is sent to OrcaRouter. Unstaged and untracked changes are not included.

Review staged changes before using AI generation. A staged diff can contain source code, credentials, customer data, or other sensitive information. API usage may incur charges according to your OrcaRouter account and selected model.

For latency and cost control, gito truncates very large staged diffs before sending them to the model.
