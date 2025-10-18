---
rfd: "020"
title: "Enhanced Server Status API"
state: "implemented"
api_changes: true
areas: ["cli", "gateway", "local-agent", "webui"]
---

# RFD 020 - Enhanced Server Status API

**Status:** 🎉 Implemented

## Summary

Enhance the existing BMC info API to include detailed system status information
from Redfish's `/redfish/v1/Systems/{SystemId}` endpoint, particularly the
`BootProgress` attribute and other boot/system state indicators. This provides
visibility into why a server might not show console output (e.g., stuck in BIOS
VGA screen, POST failures) and helps with troubleshooting boot issues.

## Problem

Users currently face challenges when troubleshooting servers that show blank SOL
console screens:

- **Current behavior/limitations**:

  - The existing `bmc-cli server info` command only shows BMC hardware
    information (manager details, firmware version)
  - No visibility into system boot state, POST progress, or why the console
    might be blank
  - Cannot determine if server is stuck in BIOS/UEFI setup, VGA mode, or POST
    failure
  - Web console (SOL/VNC) pages lack contextual information about server boot
    status

- **Why this matters**:

  - Debugging blank console screens is difficult without boot progress
    information
  - Users waste time trying to access console when server is in non-SOL boot
    phase (VGA BIOS screens)
  - Redfish BMCs provide rich boot state information (`BootProgress`,
    `BootSourceOverride`, `PostState`) that is currently unused
  - Better diagnostics improve operational efficiency and reduce troubleshooting
    time

- **Use cases affected**:
  - Troubleshooting servers with blank SOL console output
  - Determining if server is stuck in BIOS/UEFI setup during boot
  - Identifying POST failures or boot sequence issues
  - Monitoring server boot progress in web console UI
  - Understanding why VNC might be needed instead of SOL (VGA-only boot phase)

## Solution

Extend the BMC info API to fetch and expose detailed system status information
from Redfish Systems endpoints:

**Key Design Decisions:**

- Extend existing `GetBMCInfo` RPC rather than creating separate API to keep
  related information together
- Fetch `/redfish/v1/Systems/{SystemId}` for Redfish BMCs in addition to
  existing Manager info
- Add optional extended status fields to `RedfishInfo` message for boot
  progress, POST state, and boot source
- Support graceful degradation: if Systems endpoint fails, still return Manager
  info (backward compatible)
- Display boot progress in CLI `server info` command with visual indicators
- Add boot status section to web console sidebar for SOL/VNC pages
- Keep IPMI implementation unchanged (IPMI lacks equivalent boot progress APIs)

**Benefits:**

- Immediate visibility into server boot state when troubleshooting console
  issues
- Helps users understand when to use VNC (VGA BIOS screens) vs SOL (OS console)
- Reduces mean time to resolution for boot-related issues
- Provides actionable information in web console UI without requiring CLI access
- Lays foundation for future boot management features (boot device override,
  etc.)

**Architecture Overview:**

```
CLI/Browser → Manager → Gateway → Agent → Redfish BMC
                                            ├─ /redfish/v1/Managers/{Id} (existing)
                                            └─ /redfish/v1/Systems/{SystemId} (new)
```

### Component Changes

1. **Local Agent** (`local-agent/pkg/redfish/`):

   - Add `GetSystemInfo()` method to fetch `/redfish/v1/Systems/{SystemId}` data
   - Extend `ComputerSystem` struct with boot progress fields:
     - `BootProgress.LastState` (standard Redfish)
     - `BootProgress.OemLastState` (Dell-specific)
     - `Boot.BootSourceOverrideTarget/Enabled/Mode`
     - `BiosVersion`, `SerialNumber`, `SKU`, `HostName`, `LastResetTime`
   - Modify `GetBMCInfo()` to optionally fetch system info and merge with
     manager info
   - Handle Dell iDRAC-specific extensions:
     - `Oem.Dell.DellSystem` with health rollup statuses
     - Parse rollup status fields (CPU, Storage, Temp, Voltage, Fans, PSU, etc.)
     - Extract `BootProgress.OemLastState` when `LastState` is `"OEM"`

2. **Gateway** (`gateway/proto/gateway/v1/`):

   - Extend `RedfishInfo` protobuf message with new optional fields for system
     status
   - Add `SystemStatus` sub-message for boot-related attributes
   - Maintain backward compatibility with existing clients

3. **CLI** (`cli/cmd/server_bmc_info.go`):

   - Display boot progress information in `server info` output for Redfish
     servers
   - Use visual indicators (icons, color-coded status) for boot state
   - Add optional `--extended` flag to show full system details

4. **Web Console** (`gateway/internal/webui/`):
   - Add "Boot Status" section to SOL/VNC console sidebar
   - Display boot progress, POST state, and boot source information
   - Auto-refresh boot status periodically (similar to power status)
   - Show helpful context (e.g., "Server in VGA mode - use VNC for BIOS access")

**Configuration Example:**

No configuration changes required - feature is enabled by default for Redfish
BMCs.

## API Changes

### Extended Protobuf Messages

```protobuf
// Extend existing RedfishInfo message (backward compatible)
message RedfishInfo {
    string manager_id = 1;
    string name = 2;
    string model = 3;
    string manufacturer = 4;
    string firmware_version = 5;
    string status = 6;
    string power_state = 7;
    repeated NetworkProtocol network_protocols = 8;

    // NEW: Optional system status information
    SystemStatus system_status = 9;
}

// NEW: SystemStatus contains boot and system state information
message SystemStatus {
    string system_id = 1;                    // e.g., "System.Embedded.1"
    string boot_progress = 2;                // e.g., "SystemHardwareInitializationComplete", "OSBootStarted", "OSRunning", "OEM"
    string boot_progress_oem = 3;            // OEM-specific boot state (Dell: OemLastState, e.g., "No bootable devices.")
    string post_state = 4;                   // e.g., "Completed", "InProgress", "Failed" (may be empty on Dell)
    BootSourceOverride boot_source = 5;      // Boot source override settings
    string bios_version = 6;                 // BIOS/UEFI firmware version
    string serial_number = 7;                // System serial number
    string sku = 8;                          // System SKU
    string hostname = 9;                     // System hostname
    string last_reset_time = 10;             // Last reset/reboot timestamp (RFC3339)
    map<string, string> oem_health = 11;     // Vendor-specific health status (Dell: CPURollupStatus, StorageRollupStatus, etc.)
}

// NEW: Boot source override information
message BootSourceOverride {
    string target = 1;                       // e.g., "None", "Pxe", "Hdd", "Usb", "BiosSetup"
    string enabled = 2;                      // e.g., "Once", "Continuous", "Disabled"
    string mode = 3;                         // e.g., "UEFI", "Legacy"
}
```

### CLI Commands

```bash
# Existing command now shows extended info for Redfish servers
bmc-cli server info <server-id>

# Example output for Redfish (Dell PowerEdge R640 with system status):
Server ID:    bmc-dc-local-dev-http-//redfish-01-8000
BMC Type:     redfish

Redfish Information:
  Manager ID:         iDRAC.Embedded.1
  Name:               Manager
  Model:              PowerEdge R640
  Manufacturer:       Dell Inc.
  Firmware Version:   6.10.30.00
  Status:             Enabled (OK)
  Power State:        On

System Status:
  System ID:          System.Embedded.1
  Hostname:           localhost
  Serial Number:      MXWSJ0018500XM
  SKU:                CBF6TH3
  BIOS Version:       2.21.2
  Last Reset:         2025-10-14 21:39:06 UTC
  Boot Progress:      OEM - No bootable devices. ⚠
  Boot Source:        None (Legacy, Disabled)
  Boot Order:         NIC.Slot.3-1-1, NIC.Slot.3-2-1, HardDisk.List.1-1
  Health Status:      OK (CPU: OK, Storage: OK, Temp: OK, Fans: OK)

  Note: No bootable devices detected. Check boot configuration or use VNC to access BIOS setup.

# Example output when server is running normally:
System Status:
  System ID:          System.Embedded.1
  Hostname:           server01.example.com
  Serial Number:      MXWSJ0018500XM
  SKU:                CBF6TH3
  BIOS Version:       2.21.2
  Last Reset:         2025-10-14 08:15:23 UTC
  Boot Progress:      OSRunning ✓
  Boot Source:        Hdd (UEFI, Disabled)
  Health Status:      OK (CPU: OK, Storage: OK, Temp: OK, Fans: OK)

# Example output when server is stuck in BIOS setup:
System Status:
  System ID:          System.Embedded.1
  Boot Progress:      SetupEntered ⚠
  BIOS Version:       2.21.2
  Serial Number:      MXWSJ0018500XM
  Boot Source:        BiosSetup (UEFI, Once)

  Note: Server is in BIOS Setup mode. Use VNC console for graphical access.

# Optional extended flag for full details
bmc-cli server info <server-id> --extended

# Example JSON output
bmc-cli server info <server-id> --output json
{
  "server_id": "bmc-dc-local-dev-http-//redfish-01-8000",
  "bmc_type": "redfish",
  "redfish_info": {
    "manager_id": "iDRAC.Embedded.1",
    "name": "Manager",
    "model": "PowerEdge R640",
    "manufacturer": "Dell Inc.",
    "firmware_version": "6.10.30.00",
    "status": "Enabled (OK)",
    "power_state": "On",
    "system_status": {
      "system_id": "System.Embedded.1",
      "hostname": "localhost",
      "serial_number": "MXWSJ0018500XM",
      "sku": "CBF6TH3",
      "bios_version": "2.21.2",
      "last_reset_time": "2025-10-14T21:39:06Z",
      "boot_progress": "OEM",
      "boot_progress_oem": "No bootable devices.",
      "post_state": "",
      "boot_source": {
        "target": "None",
        "enabled": "Disabled",
        "mode": "Legacy"
      },
      "oem_health": {
        "cpu_rollup_status": "OK",
        "storage_rollup_status": "OK",
        "temp_rollup_status": "OK",
        "volt_rollup_status": "OK",
        "fan_rollup_status": "OK",
        "ps_rollup_status": "OK",
        "battery_rollup_status": "OK",
        "system_health_rollup_status": "OK"
      }
    }
  }
}
```

### Web Console Integration

The web console SOL/VNC pages will display boot status in the sidebar:

- Boot Status
  - Boot Progress
  - POST State
  - Boot Source

## Implementation Plan

### Phase 1: Redfish Client Extensions

- [ ] Extend `ComputerSystem` struct in `local-agent/pkg/redfish/types.go` with
      boot status fields
- [ ] Add `GetSystemInfo()` method to `local-agent/pkg/redfish/client.go`
- [ ] Update `GetBMCInfo()` to fetch and merge system status information
- [ ] Add unit tests for Redfish system info parsing

### Phase 2: Protocol Definitions

- [ ] Extend `RedfishInfo` message in `proto/gateway/v1/gateway.proto`
- [ ] Add `SystemStatus` and `BootSourceOverride` messages
- [ ] Generate protobuf code (`make gen`)

### Phase 3: Agent & Gateway Integration

- [ ] Update agent's `GetBMCInfo()` RPC handler to include system status
- [ ] Update `local-agent/pkg/bmc/client.go` to call Redfish system info methods
- [ ] Add error handling for Systems endpoint (graceful degradation)
- [ ] Update gateway handler (no changes needed - passes through agent response)

### Phase 4: CLI Display

- [ ] Update `cli/cmd/server_bmc_info.go` to display system status section
- [ ] Add visual indicators for boot progress states (✓, ⚠, ✗)
- [ ] Add contextual hints for common boot states (BIOS setup, POST failure)
- [ ] Update JSON output format

### Phase 5: Web Console Integration

- [ ] Add boot status section to
      `gateway/internal/webui/templates/bmc_info_sidebar.html`
- [ ] Create JavaScript function to fetch and display boot status
- [ ] Add periodic refresh (30s interval, similar to power status)
- [ ] Add contextual hints/warnings based on boot state
- [ ] Update VNC and console templates to include boot status sidebar

### Phase 6: Testing & Documentation

- [ ] Add unit tests for system info parsing
- [ ] Add integration tests with Redfish simulators
- [ ] Add E2E test for CLI `server info` with system status
- [ ] Test web console boot status display and auto-refresh
- [ ] Update user documentation with examples

## Testing Strategy

### Unit Tests

1. **Redfish Client Tests** (`local-agent/pkg/redfish/client_test.go`):

   - Test parsing of Systems endpoint response with various boot states
   - Test handling of missing/optional fields (graceful degradation)
   - Test vendor-specific OEM extensions (Dell iDRAC, HPE iLO)
   - Test error handling when Systems endpoint unavailable

2. **BMC Client Tests** (`local-agent/pkg/bmc/client_test.go`):

   - Test integration of system status into BMCInfo response
   - Test fallback behavior when system info fetch fails
   - Verify Manager info still returned on System endpoint failure

3. **CLI Tests** (`cli/cmd/server_bmc_info_test.go`):
   - Test display formatting for different boot states
   - Test JSON output includes system status
   - Test handling of missing system status (IPMI, old Redfish)

### Integration Tests

1. **Redfish Simulator Tests** (`tests/integration/system_status_test.go`):

   - Test against DMTF Redfish mock server with different boot states
   - Simulate boot progress states: POST, BIOS setup, OS running
   - Test boot source override scenarios
   - Verify OEM extension handling

2. **Dell iDRAC Integration** (manual testing):
   - Test against real Dell iDRAC with `HostBootStatus` OEM field
   - Verify boot progress during actual server boot cycle
   - Test BIOS setup detection

### E2E Tests

1. **CLI E2E Test** (`tests/e2e/system_status_test.go`):

   - Start full stack with Redfish simulator
   - Execute `bmc-cli server info` and verify system status output
   - Verify boot progress indicators displayed correctly
   - Test JSON output format

2. **Web Console E2E Test**:
   - Load SOL console page and verify boot status sidebar appears
   - Verify boot status auto-refresh updates display
   - Test contextual hints displayed for BIOS setup state

## Redfish Boot Progress Values

Common `BootProgress` values and their meanings:

| Value                                     | Meaning                      | SOL Available? | Recommended Action         |
| ----------------------------------------- | ---------------------------- | -------------- | -------------------------- |
| `None`                                    | No boot progress information | Unknown        | Check POST state           |
| `PrimaryProcessorInitializationStarted`   | CPU initialization           | No             | VNC for BIOS messages      |
| `MemoryInitializationStarted`             | Memory training              | No             | VNC for BIOS messages      |
| `SecondaryProcessorInitializationStarted` | Additional CPUs              | No             | VNC for BIOS messages      |
| `PCIResourceConfigStarted`                | PCIe enumeration             | No             | VNC for BIOS messages      |
| `SystemHardwareInitializationComplete`    | POST complete                | Maybe          | Check boot source          |
| `SetupEntered`                            | BIOS/UEFI setup active       | No             | Use VNC for setup access   |
| `OSBootStarted`                           | Boot loader running          | Yes            | SOL should show output     |
| `OSRunning`                               | Operating system loaded      | Yes            | SOL active                 |
| `OEM` (Dell)                              | Vendor-specific state        | Varies         | Check `OemLastState` field |

**Dell iDRAC-Specific States** (when `BootProgress.LastState` = `"OEM"`):

The `BootProgress.OemLastState` field provides Dell-specific boot status:

- `"No bootable devices."` - No valid boot devices found, stuck at boot device
  selection
- Other OEM states may include vendor-specific diagnostics or boot phase
  indicators

**Important**: Always check both `BootProgress.LastState` and
`BootProgress.OemLastState` for complete boot status on Dell systems.

## Security Considerations

- No new authentication/authorization required (uses existing BMC credentials)
- System status information is read-only and non-sensitive
- Boot source override settings revealed but not modifiable (future enhancement)
- No credential exposure in status data
- Existing authorization enforced (user must have access to server)

## Future Enhancements

- **Boot Source Management**: Add API to modify boot source override (PXE, USB,
  BIOS setup)
- **Boot Progress Monitoring**: WebSocket stream for real-time boot progress
  updates during server boot
- **POST Code Display**: Show POST codes from Redfish for detailed boot
  diagnostics
- **Health Status**: Integrate system health status (sensor thresholds,
  component failures)
- **IPMI System Status**: Investigate IPMI equivalents (limited compared to
  Redfish)
- **Boot Configuration**: BIOS settings management via Redfish BIOS registry
- **Firmware Inventory**: Display all firmware versions (BIOS, NICs, storage
  controllers)

## Appendix

### Redfish Systems Endpoint Structure

Real-world example from Dell PowerEdge R640 with iDRAC9:

```json
GET /redfish/v1/Systems/System.Embedded.1

{
    "@odata.context": "/redfish/v1/$metadata#ComputerSystem.ComputerSystem",
    "@odata.id": "/redfish/v1/Systems/System.Embedded.1",
    "@odata.type": "#ComputerSystem.v1_20_0.ComputerSystem",
    "Id": "System.Embedded.1",
    "Name": "System",
    "SystemType": "Physical",
    "Manufacturer": "Dell Inc.",
    "Model": "PowerEdge R640",
    "SerialNumber": "MXWSJ0018500XM",
    "PartNumber": "0G5DR5A00",
    "SKU": "CBF6TH3",
    "UUID": "4c4c4544-0042-4610-8036-c3c04f544833",
    "HostName": "localhost",
    "PowerState": "On",
    "BiosVersion": "2.21.2",
    "IndicatorLED": "Lit",
    "LocationIndicatorActive": false,
    "LastResetTime": "2025-10-14T21:39:06+00:00",

    "Status": {
        "State": "Enabled",
        "Health": "OK",
        "HealthRollup": "OK"
    },

    "Boot": {
        "BootSourceOverrideTarget": "None",
        "BootSourceOverrideEnabled": "Disabled",
        "BootSourceOverrideMode": "Legacy",
        "UefiTargetBootSourceOverride": null,
        "BootOrder": [
            "NIC.Slot.3-1-1",
            "NIC.Slot.3-2-1",
            "HardDisk.List.1-1"
        ],
        "BootSourceOverrideTarget@Redfish.AllowableValues": [
            "None", "Pxe", "Floppy", "Cd", "Hdd", "BiosSetup",
            "Utilities", "UefiTarget", "SDCard", "UefiHttp"
        ],
        "StopBootOnFault": "Never"
    },

    "BootProgress": {
        "LastState": "OEM",
        "OemLastState": "No bootable devices."
    },

    "ProcessorSummary": {
        "Count": 2,
        "CoreCount": 48,
        "LogicalProcessorCount": 96,
        "Model": "Intel(R) Xeon(R) Gold 6252N CPU @ 2.30GHz",
        "Status": {
            "Health": "OK",
            "HealthRollup": "OK",
            "State": "Enabled"
        }
    },

    "MemorySummary": {
        "TotalSystemMemoryGiB": 192,
        "MemoryMirroring": "System",
        "Status": {
            "Health": "OK",
            "HealthRollup": "OK",
            "State": "Enabled"
        }
    },

    "Oem": {
        "Dell": {
            "@odata.type": "#DellOem.v1_3_0.DellOemResources",
            "DellSystem": {
                "@odata.type": "#DellSystem.v1_4_0.DellSystem",
                "Id": "System.Embedded.1",
                "BIOSReleaseDate": "02/19/2024",
                "ChassisServiceTag": "CBF6TH3",
                "ExpressServiceCode": "26812028343",
                "SystemGeneration": "14G Monolithic",
                "SystemID": 1814,
                "SystemRevision": "I",
                "NodeID": "CBF6TH3",
                "EstimatedExhaustTemperatureCelsius": 38,
                "EstimatedSystemAirflowCFM": 32,
                "LastSystemInventoryTime": "2025-10-14T21:39:00+00:00",
                "MemoryOperationMode": "OptimizerMode",
                "PopulatedDIMMSlots": 6,
                "PopulatedPCIeSlots": 2,
                "PowerCapEnabledState": "Disabled",

                "CPURollupStatus": "OK",
                "StorageRollupStatus": "OK",
                "TempRollupStatus": "OK",
                "VoltRollupStatus": "OK",
                "FanRollupStatus": "OK",
                "PSRollupStatus": "OK",
                "BatteryRollupStatus": "OK",
                "IntrusionRollupStatus": "OK",
                "SELRollupStatus": "OK",
                "SystemHealthRollupStatus": "OK",
                "CurrentRollupStatus": "OK",
                "CoolingRollupStatus": "OK"
            }
        }
    }
}
```

**Key Observations:**

1. **Boot Progress on Dell iDRAC**: The `BootProgress.LastState` can be:

   - Standard Redfish values: `OSRunning`, `SetupEntered`,
     `SystemHardwareInitializationComplete`, etc.
   - `"OEM"` with additional `OemLastState` field for vendor-specific states
     (e.g., "No bootable devices.")

2. **Dell OEM Extensions**: Rich system health information in
   `Oem.Dell.DellSystem`:

   - Rollup status for all subsystems (CPU, Storage, Temp, Voltage, Fans, PSU,
     Battery, etc.)
   - System inventory timestamps
   - Thermal metrics (exhaust temperature, airflow CFM)
   - Hardware population details (DIMM slots, PCIe slots)

3. **No `PostState` field**: Dell iDRAC doesn't expose a top-level `PostState`
   field; boot state is primarily in `BootProgress`

4. **Boot Order Details**: Full boot device order available in `Boot.BootOrder`
   array

### Example Use Cases

**Use Case 1: Blank SOL Console During Boot**

Problem: User connects to SOL console but sees no output during server boot.

With this feature:

1. User runs `bmc-cli server info server-001`
2. System status shows: `Boot Progress: SetupEntered ⚠`
3. Hint displayed: "Server in BIOS Setup - use VNC for graphical access"
4. User switches to VNC console and sees BIOS setup screen

**Use Case 2: Web Console Troubleshooting**

Problem: Operator sees blank screen in web SOL console.

With this feature:

1. Boot status sidebar shows: `Boot Progress: MemoryInitializationStarted`
2. Hint displayed: "Server in early POST - VGA output only"
3. Operator switches to VNC console button
4. Operator sees BIOS memory training screen

**Use Case 3: Monitoring Server Boot Process**

Problem: Admin wants to monitor server boot progress after power on.

With this feature:

1. Admin opens SOL web console
2. Boot status section auto-refreshes every 30s
3. Status progression visible:
   - `POST State: InProgress` → `Completed`
   - `Boot Progress: PrimaryProcessorInitializationStarted` →
     `SystemHardwareInitializationComplete` → `OSBootStarted` → `OSRunning`
4. SOL output appears when boot loader starts

### Reference Implementations

- **Redfish Specification**: DSP0266 (Redfish Resource and Schema Guide)
- **DMTF ComputerSystem Schema **:
  https://redfish.dmtf.org/schemas/v1/ComputerSystem.json
- **Dell iDRAC Redfish **:
  https://downloads.dell.com/manuals/common/dellemc-redfish-api-idrac9.pdf
- **HPE iLO Redfish**: Similar boot progress available in Systems endpoint

---

## Notes

- Focus on Redfish BMCs initially; IPMI has limited equivalent functionality
- Graceful degradation ensures backward compatibility with existing deployments
- Web console integration provides immediate value without requiring CLI access
