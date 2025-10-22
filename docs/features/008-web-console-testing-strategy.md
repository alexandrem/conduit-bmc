---
rfd: "008"
title: "Web Console End-to-End Testing with Playwright"
state: "draft"
breaking_changes: false
testing_required: true
database_changes: false
api_changes: false
dependencies: []
database_migrations: []
areas: [ "gateway", "tests" ]
---

# RFD 008 - Web Console End-to-End Testing with Playwright

**Status:** 🚧 Draft

## Summary

This RFD outlines a comprehensive end-to-end testing strategy for the BMC web
console interfaces using Playwright. The system currently implements two web
console types (VNC graphical console via noVNC and SOL serial console via
XTerm.js). This document proposes browser-based E2E tests to complement the
existing Go-based E2E test framework in `tests/e2e/`, covering UI interactions,
WebSocket connections, real-time data streaming, and integrated power operations.

## Problem

The BMC management system implements web-based console interfaces that are
complex, stateful applications. While backend logic is covered by Go tests in
`tests/e2e/suites/console/`, the browser-side UI behavior lacks automated
testing:

### Current Testing Gaps

- **No Browser-Based E2E Tests**: Web UIs are not tested in actual browsers
- **WebSocket Client Behavior**: Browser-side WebSocket handling untested
- **UI Interactions**: Mouse/keyboard input to consoles not validated
- **Cross-Browser Compatibility**: Chrome, Firefox, Safari behavior unknown
- **Client-Side Error Handling**: Connection failures, reconnection logic untested
- **Visual Regressions**: UI layout and rendering changes go undetected

### Existing Infrastructure

- **Go E2E Tests**: `tests/e2e/` provides framework, suites (auth, power, console), and backend simulation
- **Console Implementations**:
  - `gateway/internal/webui/templates/vnc.html` (noVNC client)
  - `gateway/internal/webui/templates/console.html` (XTerm.js SOL terminal)
- **Test Backends**: VirtualBMC (IPMI) and synthetic Redfish simulators available

### What's Missing

- Browser automation testing the actual rendered HTML/JS
- Validation of client-side WebSocket reconnection logic
- Screenshot/visual regression testing for UI changes
- Performance metrics from browser perspective (load times, frame rates)
- Real user interaction patterns (click, type, fullscreen, power buttons)

## Solution

Add Playwright browser automation tests to complement existing Go E2E tests,
integrating with the current test infrastructure in `tests/e2e/`.

### Key Design Decisions

- **Integration with Existing Tests**: Reuse VirtualBMC/synthetic backends from `tests/e2e/backends/`
- **Browser Focus**: Test actual browser rendering and client-side JavaScript behavior
- **Page Object Pattern**: Encapsulate UI interactions for maintainability
- **Parallel Architecture**: Browser tests run alongside Go tests, not replacing them
- **CI Integration**: Use same orchestration as `make test-e2e`

### Benefits

- Catch client-side JavaScript errors and browser-specific bugs
- Validate real user workflows (clicking buttons, typing in terminals)
- Detect visual regressions in console UIs
- Measure browser performance metrics (load time, frame rate, memory)
- Test WebSocket reconnection logic from browser perspective

### Test Architecture

```
tests/
├── e2e/                          # Existing Go E2E framework
│   ├── framework/                # Reusable for setup
│   ├── suites/                   # Backend-focused tests
│   └── backends/                 # VirtualBMC, synthetic
└── e2e-browser/                  # New Playwright tests
    ├── playwright.config.ts      # Browser test configuration
    ├── fixtures/
    │   └── console-session.ts    # Session setup helpers
    ├── pages/
    │   ├── vnc-console.page.ts   # VNC page object
    │   └── sol-console.page.ts   # SOL page object
    └── specs/
        ├── vnc/                  # VNC browser tests
        ├── sol/                  # SOL terminal tests
        ├── power/                # UI power control tests
        └── visual/               # Screenshot regression
```

### Test Scope and Coverage

#### VNC Console Tests
- Connection establishment and status indicators
- noVNC canvas rendering and mouse/keyboard events
- Fullscreen mode, screenshot capture, view-only mode
- Connection loss and reconnection behavior
- Power control integration from VNC UI

#### SOL Console Tests
- XTerm.js terminal initialization and rendering
- Keyboard input and special key sequences (F1-F12, ESC)
- Terminal output streaming and scrollback
- Copy/paste and clear terminal operations
- Power control integration from SOL UI

#### WebSocket Tests
- Browser WebSocket connection establishment
- Message flow (client to server, server to client)
- Reconnection manager behavior on disconnect
- Connection status UI updates

#### Power Operations Tests
- UI button state during operations (disabled/enabled)
- Real-time power status updates via Connect RPC
- Error handling and user feedback

#### Visual Regression Tests
- Console UI layout consistency
- Status indicator colors and animations
- Connection log panel visibility
- Responsive design at different viewport sizes

### Integration with Existing Test Infrastructure

Playwright tests will reuse the existing E2E backend infrastructure:

1. **Backend Reuse**: VirtualBMC and synthetic Redfish from `tests/e2e/backends/`
2. **Orchestration**: Extend `make test-e2e` to include browser tests
3. **Configuration**: Share server endpoints from `tests/e2e/configs/`
4. **Parallel Execution**: Run browser tests after or alongside Go E2E tests

## Implementation Plan

### Phase 1: Foundation

- [ ] Install Playwright and configure `tests/e2e-browser/playwright.config.ts`
- [ ] Create page object models for VNC and SOL consoles
- [ ] Implement session fixture to create console sessions
- [ ] Integrate with existing `make test-e2e` orchestration

### Phase 2: Core Console Tests

- [ ] VNC console connection, status, and reconnection tests
- [ ] SOL terminal rendering and keyboard input tests
- [ ] WebSocket connection lifecycle tests
- [ ] Browser console error detection

### Phase 3: Integration Tests

- [ ] Power operation UI tests (button states, status updates)
- [ ] Error handling scenarios (connection failures, timeouts)
- [ ] Cross-browser testing (Chromium, Firefox, WebKit)
- [ ] Session cookie authentication validation

### Phase 4: Visual and Performance

- [ ] Screenshot baseline creation for visual regression
- [ ] Performance metrics collection (load time, frame rate)
- [ ] Memory leak detection in long-running sessions
- [ ] Mobile viewport responsive testing

## Testing Strategy

### Test Categories

- **Functional**: Console connection, UI interactions, session management
- **Integration**: WebSocket flow, Connect RPC calls, authentication
- **Performance**: Page load time, WebSocket latency, memory usage
- **Visual**: Screenshot comparison for UI changes
- **Cross-Browser**: Chromium, Firefox, WebKit compatibility

### Test Execution

```bash
# Run all browser tests
make test-e2e-browser

# Run specific suite
cd tests/e2e-browser
npx playwright test specs/vnc/

# Run in headed mode for debugging
npx playwright test --headed

# Generate visual baseline
npx playwright test specs/visual/ --update-snapshots
```

### CI/CD Integration

Extend existing `make test-e2e` target:

1. Start VirtualBMC backends (already done by `make test-e2e-machines-up`)
2. Start Gateway and Manager (already running in dev environment)
3. Run Go E2E tests (existing `tests/e2e/suites/`)
4. Run Playwright browser tests (new `tests/e2e-browser/`)
5. Collect and upload test results (JUnit XML, HTML reports)
6. Tear down test environment

## Success Metrics

### Coverage Targets

- **90%+** of UI interactions covered by browser tests
- **All** console types (VNC, SOL) tested in multiple browsers
- **All** power operations validated from UI perspective
- **Cross-browser** compatibility verified (Chromium, Firefox, WebKit)

### Performance Benchmarks

- VNC console initial load: **< 2 seconds**
- SOL terminal initialization: **< 1 second**
- WebSocket connection establishment: **< 500ms**
- Power status refresh: **< 1 second**

### Quality Gates

- Zero JavaScript console errors in happy path tests
- All WebSocket connections properly closed after tests
- No memory leaks detected in 30-minute session tests
- Visual regression tests pass (no unintended UI changes)

## Appendix

### Example Page Object

```typescript
// pages/vnc-console.page.ts
export class VNCConsolePage {
  constructor(private page: Page) {}

  async waitForConnection() {
    await expect(this.page.locator('#vnc-status-text')).toHaveText('Connected');
    await expect(this.page.locator('#noVNC_canvas')).toBeVisible();
  }

  async clickPowerButton(operation: 'on' | 'off' | 'reset' | 'cycle') {
    const buttonId = `#power-${operation}-btn`;
    await this.page.locator(buttonId).click();
  }

  async sendCtrlAltDel() {
    await this.page.locator('button:has-text("Ctrl+Alt+Del")').click();
  }

  async takeScreenshot() {
    const downloadPromise = this.page.waitForEvent('download');
    await this.page.locator('button:has-text("Screenshot")').click();
    return await downloadPromise;
  }
}
```

### Example Test

```typescript
// specs/vnc/connection.spec.ts
import { test, expect } from '@playwright/test';
import { VNCConsolePage } from '../../pages/vnc-console.page';

test.describe('VNC Console Connection', () => {
  test('should establish VNC connection successfully', async ({ page }) => {
    await page.goto('/vnc/test-session-id');

    const console = new VNCConsolePage(page);
    await console.waitForConnection();

    // Verify status indicator shows connected state
    await expect(page.locator('#vnc-status-dot')).toHaveClass(/bg-green-400/);
  });

  test('should handle connection failure gracefully', async ({ page }) => {
    await page.route('**/websocket/vnc/**', route => route.abort());
    await page.goto('/vnc/test-session-id');

    await expect(page.locator('#vnc-status-text')).toContainText('Connection Lost');
    await expect(page.locator('#vnc-status-dot')).toHaveClass(/bg-red-400/);
  });
});
```

### Playwright Configuration

```typescript
// playwright.config.ts
import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './specs',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  use: {
    baseURL: 'http://localhost:8081',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
    { name: 'firefox', use: { ...devices['Desktop Firefox'] } },
    { name: 'webkit', use: { ...devices['Desktop Safari'] } },
  ],
});
```
