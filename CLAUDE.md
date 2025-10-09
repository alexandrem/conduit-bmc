# CLAUDE.md

Multi-tenant BMC (IPMI/Redfish) management system. Flow: `CLI/Browser ↔ Manager (auth) → Gateway (proxy/UI) → Agent → BMC`

## Quick Reference

- **Architecture**: `docs/.ai/ARCHITECTURE-AI.md` (flows, topology, protocols)
- **Design**: `docs/.ai/DESIGN-AI.md` (components, sessions, API types)
- **Development**: `docs/.ai/DEVELOPMENT-AI.md` (setup, commands, workflows)
- **Testing**: `docs/.ai/TESTING-AI.md` (test tiers, commands)
- **Features**: RFD format in `docs/features/000-RFD-TEMPLATE.md`

## Critical Rules

**Testing**: All tests must pass (`make test-all`) before commits.

**Code**: Follow Effective Go conventions, Go Doc Comments style.

**Files**: NEVER create files unless absolutely necessary. ALWAYS prefer editing existing files.
