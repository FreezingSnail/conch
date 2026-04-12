---
name: slugdd
description: "Slug-compressed Prompt-Driven Development. Transforms rough idea into design doc + impl plan. Iterative: requirements clarification, research, design, implementation plan. Use this skill when user says pdd, slugdd, prompt driven development, design this idea, plan this feature, turn idea into design, or wants structured idea-to-implementation workflow."
---

<!--
  Based on pdd.sop.md from https://github.com/strands-agents/agent-sop/tree/main
  Copyright strands-agents contributors. Licensed under Apache License 2.0.
  https://www.apache.org/licenses/LICENSE-2.0

  Modifications: rewritten in slug-compressed style (~75% token reduction);
  verbose prose replaced with fragments and short synonyms; examples and
  troubleshooting sections condensed; all original constraints preserved.
-->

Rough idea → design doc + impl plan. Iterative. All steps wait for explicit user direction.

## Params

- **rough_idea** (required): initial concept
- **project_name** (optional): short name; generate kebab-case from idea prefixed YYYY-MM-DD if absent
- **project_dir** (optional, default: `.agents/planning/{project_name}`)

MUST gather all params upfront in single prompt. MUST support direct text, file path, URL input. MUST confirm param acquisition before proceeding. MUST NOT overwrite existing project dir because this destroys prior work. MUST ask for new project_dir if default exists with prior content.

## Step 1 — Create Project Structure

MUST create project_dir if absent. MUST create:
- `{project_dir}/rough-idea.md`
- `{project_dir}/idea-honing.md`
- `{project_dir}/research/`
- `{project_dir}/design/`
- `{project_dir}/implementation/`

MUST notify user when done. MUST prompt: `/context add {project_dir}/**/*.md`

## Step 2 — Initial Process Planning

MUST ask user preference:
- Start with requirements clarification (default)
- Start with preliminary research
- Provide additional context first

MUST explain process is iterative. MUST wait for explicit direction. MUST NOT auto-proceed because this leads process in direction user doesn't want.

## Step 3 — Requirements Clarification

MUST ask ONE question at a time. MUST wait for response before next question. MUST NOT list multiple questions at once because this overwhelms users and leads to incomplete responses. MUST NOT pre-populate answers because this assumes preferences without confirmation.

Per question:
1. Formulate question
2. Append to `idea-honing.md`
3. Present to user
4. Wait for complete response
5. Append final answer to `idea-honing.md`
6. Proceed to next question

MAY suggest possible answers; MUST wait for actual response. MUST ask about edge cases, UX, technical constraints, success criteria. MUST explicitly ask if requirements clarification is complete before moving on. MUST offer research option if questions arise needing it. MUST NOT proceed without explicit user direction because this skips important clarification steps.

## Step 4 — Research

MUST identify research areas from requirements. MUST propose research plan to user. MUST ask user for: additional topics, recommended resources, existing knowledge to contribute. MUST document findings in `{project_dir}/research/` as separate markdown files. MUST include mermaid diagrams for architectures/data flows. MUST include source links. MUST check in with user periodically during research. MUST ask if research sufficient before proceeding. MUST offer return to requirements if research uncovers new questions. MUST NOT auto-return to requirements without explicit user direction because this disrupts intended workflow.

## Step 5 — Iteration Checkpoint

MUST summarize current requirements + research state. MUST ask user:
- Proceed to design
- Return to requirements clarification
- Conduct more research

MUST support iterating as many times as needed. MUST NOT proceed to design without explicit confirmation because this skips important refinement steps.

## Step 6 — Detailed Design

MUST create `{project_dir}/design/detailed-design.md`. MUST write as standalone doc. MUST include sections:
- Overview
- Detailed Requirements (consolidated from idea-honing.md)
- Architecture Overview
- Components and Interfaces
- Data Models
- Error Handling
- Testing Strategy
- Appendices (Technology Choices, Research Findings, Alternative Approaches)

MUST generate mermaid diagrams for architecture, data flow, component relationships. MUST review with user and iterate. MUST ask if ready to proceed to impl. MUST NOT proceed without explicit confirmation because this skips important design refinement.

## Step 7 — Implementation Plan

MUST create `{project_dir}/implementation/plan.md`. MUST include checklist at top tracking all steps.

Build each component test-driven, agile. Each step = working demoable increment. Prioritize incremental progress + early testing. No hanging/orphaned code.

Each step MUST include:
- Clear objective
- Implementation guidance
- Test requirements
- Integration with prior steps
- **Demo**: working functionality demonstrable after step

MUST sequence so core end-to-end functionality available early. MUST NOT create testing-only steps because this violates TDD principles and allows untested code to accumulate. MUST NOT duplicate design doc detail because this creates redundancy and inconsistencies. Checklist items MUST map 1:1 to steps.

## Step 8 — Summarize

MUST create `{project_dir}/summary.md` listing all artifacts, design overview, impl plan overview, next steps. SHOULD highlight areas needing further refinement. MUST present summary in conversation.

## Troubleshooting

**Requirements stall:** suggest different aspect, provide examples/options, summarize gaps, offer research.

**Research blocked:** document missing info, suggest alternatives, ask user for context, continue with available info.

**Design too complex:** break into smaller components, focus on core first, suggest phased approach, return to requirements to prioritize.
