# Hook Execution Models: Architecture Clarification

**Context:** User clarification on `goneat assess --hook` vs hook file contents
**Question:** What goes in the actual hook files? Is `assess --hook` for manual testing?

---

## Three Architectural Approaches

### Model 1: Orchestrated Assessment (Current Implementation)
**Hook files call `goneat assess --hook`**

```
.git/hooks/pre-commit:
#!/bin/bash
goneat assess --hook pre-commit --manifest .goneat/hooks.yaml
```

**Pros:**
- ✅ Single entry point for all validation logic
- ✅ Centralized configuration management
- ✅ Easy to test hooks manually: `goneat assess --hook pre-commit`
- ✅ Unified reporting and audit trails
- ✅ Simple hook files, complex logic in goneat

**Cons:**
- ❌ Requires goneat to be installed and in PATH
- ❌ Hook execution depends on goneat binary availability
- ❌ Less transparent what happens in each hook

**Use Case:** Teams that want unified, intelligent validation with simple hook files

---

### Model 2: Embedded Logic (Alternative)
**Hook files are generated Go programs with embedded logic**

```
.git/hooks/pre-commit:
#!/usr/bin/env go run
package main

import (
    "goneat/internal/assess"
    "goneat/internal/hooks"
)

func main() {
    engine := assess.NewAssessmentEngine()
    result, err := engine.RunHookAssessment("pre-commit")
    if err != nil {
        os.Exit(1)
    }
}
```

**Pros:**
- ✅ Self-contained hook files
- ✅ No external dependencies
- ✅ Direct access to goneat's logic
- ✅ Transparent execution
- ✅ Can run without goneat binary installed

**Cons:**
- ❌ More complex hook file generation
- ❌ Duplication of logic across hook files
- ❌ Harder to test hooks manually
- ❌ Updates require regenerating all hooks

**Use Case:** Teams that want self-contained hooks with no external dependencies

---

### Model 3: Command Composition (Alternative)
**Hook files call specific goneat commands**

```
.git/hooks/pre-commit:
#!/bin/bash
goneat format --check --quiet
goneat lint --check --quiet
goneat assess --categories security --fail-on high
```

**Pros:**
- ✅ Transparent what each hook does
- ✅ Easy to customize individual hooks
- ✅ Can mix goneat commands with external tools
- ✅ Simple to understand and debug

**Cons:**
- ❌ Logic scattered across multiple hook files
- ❌ Harder to maintain consistency
- ❌ No unified reporting or audit trails
- ❌ Duplication of configuration

**Use Case:** Teams that want fine-grained control and transparency

---

## The Clarification Question

**What is `goneat assess --hook` for?**

### If Model 1 (Orchestrated):
- **Purpose:** Manual testing/simulation of hook execution
- **Usage:** `goneat assess --hook pre-commit` (runs same logic as hook)
- **Hook Content:** Simple script calling `goneat assess --hook`

### If Model 2 (Embedded):
- **Purpose:** Testing hook logic without git context
- **Usage:** `goneat assess --hook pre-commit` (tests the logic)
- **Hook Content:** Generated Go program with embedded logic

### If Model 3 (Composition):
- **Purpose:** Running specific assessment categories
- **Usage:** `goneat assess --categories format,lint`
- **Hook Content:** Script calling multiple goneat commands

---

## Recommendation: Model 1 (Orchestrated Assessment)

**Why Model 1 is the strongest fit for goneat's vision:**

1. **Unified Intelligence**: Single assessment engine orchestrates all validation
2. **Dogfooding**: Hooks use goneat's own commands (perfect dogfooding)
3. **Maintainability**: Logic centralized, not scattered across hook files
4. **Testability**: `goneat assess --hook` allows manual testing
5. **Extensibility**: Easy to add new validation categories
6. **User Experience**: Simple, predictable hook behavior

**The hook files become simple entry points:**
```bash
#!/bin/bash
# .git/hooks/pre-commit
goneat assess --hook pre-commit
```

**While the intelligence lives in goneat:**
```bash
# Manual testing
goneat assess --hook pre-commit

# Direct assessment
goneat assess --categories format,lint

# CI/CD integration
goneat assess --format json --output reports/
```

---

## Decision Framework

**Choose Model 1 if you want:**
- Unified, intelligent validation orchestration
- Simple hook files with complex logic in goneat
- Easy manual testing and debugging
- Perfect alignment with goneat's assess-first architecture

**Choose Model 2 if you want:**
- Self-contained hooks with no external dependencies
- Direct embedding of goneat logic
- Maximum transparency in hook execution

**Choose Model 3 if you want:**
- Fine-grained control over each hook
- Mix of goneat and external commands
- Maximum customization flexibility

---

## Next Steps

**Before proceeding with v0.1.2:**

1. **Confirm Model**: Which execution model aligns with your vision?
2. **Clarify Purpose**: What should `goneat assess --hook` do?
3. **Define Hook Content**: What goes in the actual `.git/hooks/` files?
4. **User Experience**: How should developers interact with hooks?

**This architectural decision will shape goneat's hook system for the long term.**

---

**Current Implementation:** Model 1 (Orchestrated Assessment)
**Status:** Ready for clarification and potential model switch</content>
</xai:function_call/>
**Recommendation:** Model 1 (Orchestrated Assessment)
**Status:** Ready for clarification and potential model switch

---

## The Key Question

**What should `goneat assess --hook pre-commit` do?**

**Option A:** Manual testing of hook logic (current implementation)
```bash
# User runs this to test what the hook would do
goneat assess --hook pre-commit

# Hook file contains:
#!/bin/bash
goneat assess --hook pre-commit
```

**Option B:** Direct assessment without hook context
```bash
# User runs this for general assessment
goneat assess --categories format,lint

# Hook file contains the actual logic:
#!/bin/bash
goneat format --check
goneat lint --check
```

**Which approach aligns better with your vision of how developers should interact with goneat hooks?**

The answer will determine whether we:
- Keep the current orchestrated approach
- Switch to embedded logic in hook files  
- Go with command composition

**This is the architectural pivot point for goneat hooks.** 🎯

---

**P.S.** Regardless of the model, the core value proposition remains: **unified, intelligent validation that delivers immediate business value**. The execution model just determines *how* that intelligence is delivered to users. 

**Ready for your guidance on the preferred model!** 🚀