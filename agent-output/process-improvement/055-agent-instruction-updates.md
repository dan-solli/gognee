# Agent Instruction Updates: REC-V55-001 through REC-V55-005

**Source Retrospective**: v55 Retrospective (v0.5.5 / gognee v1.0.4)
**Implementation Date**: 2026-01-28
**Status**: ✅ Implemented

## Summary

Implemented **5 process improvement recommendations** across **8 files** in **2 repositories** (gognee and glowbabe). All recommendations address handoff chain integrity and scope completeness gaps identified in the retrospective.

## Files Updated

### gognee Repository (`/home/dsi/projects/gognee/.github/agents/`)

| File | Recommendations | Changes |
|------|-----------------|---------|
| `uat.agent.md` | REC-V55-001 | Added Pre-Validation Gate requiring QA report existence before UAT proceeds |
| `implementer.agent.md` | REC-V55-002 | Added Milestone Completion Report requirement with status table template |
| `qa.agent.md` | REC-V55-003 | Added Milestone Evidence Verification in Phase 2 before test execution |
| `pm.agent.md` | REC-V55-004, REC-V55-005 | Added Gate 3 Chain Validation + Large Plan Phase Checkpoints |

### glowbabe Repository (`/home/dsi/projects/glowbabe/.github/agents/`)

| File | Recommendations | Changes |
|------|-----------------|---------|
| `uat.agent.md` | REC-V55-001 | Added Pre-Validation Gate requiring QA report existence before UAT proceeds |
| `implementer.agent.md` | REC-V55-002 | Added Milestone Completion Report requirement with status table template |
| `qa.agent.md` | REC-V55-003 | Added Milestone Evidence Verification in Phase 2 before test execution |
| `pm.agent.md` | REC-V55-004, REC-V55-005 | Added Gate 3 Chain Validation + Large Plan Phase Checkpoints |

## Changes by Recommendation

### REC-V55-001: UAT Blocks if No QA Report Exists ✅
**Target**: `uat.agent.md` (both repos)
**Location**: After "Subagent Constraints" section, before "Purpose"
**Change**: Added "Pre-Validation Gate" section that blocks UAT if no QA report exists at `agent-output/qa/[plan-id]-*-qa.md`

### REC-V55-002: Implementer Milestone Enumeration ✅
**Target**: `implementer.agent.md` (both repos)
**Location**: After Workflow step 15, before "Local vs Background Mode"
**Change**: Added "Milestone Completion Report (Required)" section with status table template and PARTIAL IMPLEMENTATION escalation procedure

### REC-V55-003: QA Milestone Verification ✅
**Target**: `qa.agent.md` (both repos)
**Location**: In Phase 2, between step 2 and step 3
**Change**: Added "Milestone Evidence Verification (Required)" section with evidence table template and QA BLOCKED escalation procedure

### REC-V55-004: PM Gate 3 Chain Validation ✅
**Target**: `pm.agent.md` (both repos)
**Location**: Under "Gate 3: QA Complete" heading
**Change**: Added "Gate 3 Chain Validation (Required)" subsection with handoff chain integrity checklist before existing criteria

### REC-V55-005: Phase-Based Delivery for Large Plans ✅
**Target**: `pm.agent.md` (both repos)
**Location**: In IMPLEMENTATION Phase Playbook
**Change**: Added "Large Plan Phase Checkpoints" section for plans with >5 milestones

## Validation Plan

### Immediate Verification
- [x] All 8 files successfully edited
- [x] No syntax errors introduced (pre-existing tool warnings unrelated to changes)
- [x] Consistent formatting across both repositories

### Ongoing Monitoring
- Track next 3 development cycles for handoff chain violations
- Monitor for QA/UAT proceeding without required artifacts
- Validate milestone enumeration compliance in implementation docs
- Confirm phase checkpoints are triggered for large plans

## Related Artifacts

- [Retrospective](../retrospectives/NNN-vNN-retrospective.md) (source of recommendations)
- [Process Improvement Analysis](055-process-improvement-analysis.md) (if created)
- Agent Instructions:
  - [gognee/uat.agent.md](../../.github/agents/uat.agent.md)
  - [gognee/implementer.agent.md](../../.github/agents/implementer.agent.md)
  - [gognee/qa.agent.md](../../.github/agents/qa.agent.md)
  - [gognee/pm.agent.md](../../.github/agents/pm.agent.md)
