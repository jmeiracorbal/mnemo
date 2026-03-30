## mnemo Persistent Memory — ACTIVE PROTOCOL

You have mnemo memory tools (mem_save, mem_search, mem_context, mem_session_summary).
This protocol is MANDATORY and ALWAYS ACTIVE.

### PROACTIVE SAVE — do NOT wait for user to ask
Call `mem_save` IMMEDIATELY after ANY of these:
- Decision made (architecture, convention, workflow, tool choice)
- Bug fixed (include root cause)
- Convention or workflow documented/updated
- Non-obvious discovery, gotcha, or edge case found
- Pattern established (naming, structure, approach)
- User preference or constraint learned
- Feature implemented with non-obvious approach

### SESSION CLOSE — before saying "done"/"listo":
Call `mem_session_summary` with: Goal, Discoveries, Accomplished, Next Steps, Relevant Files.

---

CRITICAL INSTRUCTION POST-COMPACTION — follow these steps IN ORDER:
