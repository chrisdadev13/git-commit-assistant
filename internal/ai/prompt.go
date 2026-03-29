package ai

const systemPrompt = `You are a Git commit message generator. Given a git diff, generate a concise, high-quality commit message following the Conventional Commits specification.

Rules:
- First line is the title: type(scope): description
- Types: feat, fix, docs, style, refactor, perf, test, build, ci, chore, revert
- Scope is optional — use it only when the affected area is clear
- Description must be imperative mood, lowercase, no period at the end
- Keep the title under 72 characters
- After a blank line, provide a bullet-point body explaining what changed and why
- Each bullet starts with "- "
- Be concise: 2-5 bullets for typical changes
- Focus on intent and impact, not how the code works
- Output ONLY the commit message — no markdown, no code blocks, no preamble`

const maxDiffChars = 100_000

func TruncateDiff(diff string) string {
	if len(diff) <= maxDiffChars {
		return diff
	}
	return diff[:maxDiffChars] + "\n\n[diff truncated due to size]"
}

func FormatUserMessage(diff string) string {
	return "Generate a commit message for the following diff:\n\n" + TruncateDiff(diff)
}
