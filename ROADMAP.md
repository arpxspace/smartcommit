### 1. Flip the Model
**Current:** AI generates → Human reviews → Human edits
**New:** Human writes → AI critiques → Human refines

**Implementation:**
- [ ] New workflow: Have side by side textarea and feedback panel
- [ ] After user writes initial message, AI analyzes it
- [ ] AI provides specific, actionable feedback
- [ ] User refines based on feedback
- [ ] Option to skip AI critique for trivial commits

### 2. Prompt Engineering Overhaul
**Problem:** Current messages are too verbose, focus on "what" not "why"

**New prompting strategy:**
```
You are a commit message coach. Analyze this commit message and diff.

CRITIQUE FOR:
1. Does it explain WHY the change was made? (not just what changed)
2. Is it concise? Flag vague words: "enhances", "improves", "optimizes", "better"
3. Are there unrelated changes that should be separate commits?
4. Does it link to relevant issues/tickets?
5. Would someone 6 months from now understand the reasoning?

PROVIDE:
- 2-3 specific improvements (be brief)
- Highlight one thing done well
- Optional: suggest rewrite for one weak section

DO NOT:
- Rewrite the entire message
- Describe what's in the diff
- Use marketing language
- Be verbose
```

---

## Links & References

### Style Guides to Reference
- [Google CL Descriptions](https://google.github.io/eng-practices/review/developer/cl-descriptions.html)
- [Zulip Commit Discipline](https://zulip.readthedocs.io/en/latest/contributing/commit-discipline.html)
- [Conventional Commits](https://www.conventionalcommits.org/)

### HN Feedback Highlights
- "Write thoughtful messages for humans, not just AI summaries"
- "Commit messages are documentation for humans AND machines"
- "The tool should help me articulate what's in my head, not write for me"

---

*Last updated: 2025-11-23*
*Based on: HN feedback, community discussion, maintainer insights*
