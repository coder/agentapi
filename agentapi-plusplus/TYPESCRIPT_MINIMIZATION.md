# TypeScript Minimization Plan

## Current State
- Many .ts files in thegent-audit/docs/reference/api/ts-stubs/
- These appear to be generated API stubs

## Recommendations

### Can Remove/Replace
1. **Generated stubs** - Can generate on-demand via Rust
2. **Template files** - Already duplicated across projects  
3. **Vitepress configs** - Consolidate to single config

### Keep (Essential)
- Actual extension code (.ts in extensions/vscode/src/)
- Playwright test configs
- Build configs

### Action Items
1. Identify duplicate configs → consolidate
2. Replace generated stubs with runtime generation
3. Remove unused templates

## Other Languages
- Python: Already minimized via Rust migration
- Zig: Keep for low-level only  
- Mojo: Keep for numerical compute
- Go: Already optimal
