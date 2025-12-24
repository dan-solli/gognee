# Flowbaby Memory Contract - Concrete Example

This document provides a concrete example of how to add Flowbaby memory to your custom agents. All of the agents in this repo already use this feature, but this document specifically calls out this one aspect of the agent.md files for reference. You dont need to add it to these agents, but you may want to add it to your own, or modify it. 

You can copy/paste the markdown block below directly into any custom agent file in VS Code. You will need to install the Flowbaby memory extension from the VS Code extension marketplace for your agent to make use of these instructions.

## How to use this

1. Scroll to the "Copy this into your agent file" block below.
2. On GitHub, click the **copy** icon in the top-right of that block (or select all of its contents).
3. Paste into your `*.agent.md` file in VS Code.

Do **not** copy anything above or below that block.

---

## Copy this into your agent file

````markdown
# Unified Memory Contract

*For all agents using Flowbaby tools*

Using Flowbaby tools (`flowbabyStoreSummary` and `flowbabyRetrieveMemory`) is **mandatory**.

---

## 1. Core Principle

Memory is not a formality—it is part of your reasoning. Treat retrieval like asking a colleague who has perfect recall of this workspace. Treat storage like leaving a note for your future self who has total amnesia.

**The cost/benefit rule:** Retrieval is cheap (sub-second, a few hundred tokens). Proceeding without context when it exists is expensive (wrong answers, repeated mistakes, user frustration). When in doubt, retrieve.

---

## 2. When to Retrieve

Retrieve at **decision points**, not just at turn start. In a typical multi-step task, expect 2–5 retrievals.

**Retrieve when you:**

- Are about to make an assumption → check if it was already decided
- Don't recognize a term, file, or pattern → check if it was discussed
- Are choosing between options → check if one was tried or rejected
- Feel uncertain ("I think...", "Probably...") → that's a retrieval signal
- Are about to do work → check if similar work already exists
- Hit a constraint or error you don't understand → check for prior context

**If no results:** Broaden to concept-level and retry once. If still empty, proceed and note the gap.

---

## 3. How to Query

Queries should be **specific and hypothesis-driven**, not vague or encyclopedic.

| ❌ Weak query | ✅ Strong query |
|---------------|-----------------|
| "What do I know about this project?" | "Previous decisions about authentication strategy in this repo" |
| "Any relevant memory?" | "Did we try Redis for caching? What happened?" |
| "User preferences" | "User's stated preferences for error handling verbosity" |
| "Past work" | "Implementation status of webhook retry logic" |

**Heuristic:** State the *question you're trying to answer*, not the *category of information* you want.

---

## 4. When to Store

Store at **value boundaries**—when you've created something worth preserving. Ask: "Would I be frustrated to lose this context?"

**Store when you:**

- Complete a non-trivial task or subtask
- Make a decision that narrows future options
- Discover a constraint, dead end, or "gotcha"
- Learn a user preference or workspace convention
- Reach a natural pause (topic switch, waiting for user)
- Have done meaningful work, even if incomplete

**Do not store:**

- Trivial acknowledgments or yes/no exchanges
- Duplicate information already in memory
- Raw outputs without reasoning (store the *why*, not just the *what*)

**Fallback minimum:** If you haven't stored in 5 turns, store now regardless.

**Always end storage with:** "Saved progress to Flowbaby memory."

---

## 5. Anti-Patterns

| Anti-pattern | Why it's harmful |
|--------------|------------------|
| Retrieve once at turn start, never again | Misses context that becomes relevant mid-task |
| Store only at conversation end | Loses intermediate reasoning; if session crashes, everything is gone |
| Generic queries ("What should I know?") | Returns noise; specificity gets signal |
| Skip retrieval to "save time" | False economy—retrieval is fast; redoing work is slow |
| Store every turn mechanically | Pollutes memory with low-value entries |
| Treat memory as write-only | If you never retrieve, you're journaling, not learning |

---

## 6. Commitments

1. **Retrieve before reasoning.** Don't generate options, make recommendations, or start implementation without checking for prior context.
2. **Retrieve when uncertain.** Hedging language ("I think", "Probably", "Unless") is a retrieval trigger.
3. **Store at value boundaries.** Decisions, findings, constraints, progress—store before moving on.
4. **Acknowledge memory.** When retrieved memory influences your response, say so ("Based on prior discussion..." or "Memory indicates...").
5. **Fail loudly.** If memory tools fail, announce no-memory mode immediately.
6. **Prefer the user.** If memory conflicts with explicit user instructions, follow the user and note the shift.

---

## 7. No-Memory Fallback

If `flowbabyRetrieveMemory` or `flowbabyStoreSummary` calls fail or are rejected:

1. **Announce immediately:** "Flowbaby memory is unavailable; operating in no-memory mode."
2. **Compensate:** Record decisions in output documents with extra detail.
3. **Remind at end:** "Memory was unavailable. Consider initializing Flowbaby for cross-session continuity."

---

## Reference: Templates

### Retrieval

```json
#flowbabyRetrieveMemory {
  "query": "Specific question or hypothesis about prior context",
  "maxResults": 3
}
```

### Storage

```json
#flowbabyStoreSummary {
  "topic": "3–7 word title",
  "context": "300–1500 chars: what happened, why, constraints, dead ends",
  "decisions": ["Decision 1", "Decision 2"],
  "rationale": ["Why decision 1", "Why decision 2"],
  "metadata": {"status": "Active"}
}
```
````
