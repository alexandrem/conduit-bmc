# Development with VirtualBMC

## Understanding the Containers Layout

- x-server-0X – Pretend host OS (runs getty, serial console, VNC desktop). We
  keep one container per simulated physical machine so SOL/VNC sessions have
  something to attach to.
- x-virtualbmc-0X – VirtualBMC process for that server. It needs its own
  container to bind UDP/623 and drive power events against the dev-server-*
  container via Docker.
- x-redfish-0X – Sushy emulator serving the Redfish API for the same machine.
  Separate container lets us expose HTTP/800X, inject TLS/auth settings, etc.
- x-novnc – One container multiplexing web VNC viewers (ports 6080–6082) to
  each underlying server.
- x-test-cli – Optional toolbox with ipmitool/curl for manual debugging.

A single "server" = 3 containers (server, virtualbmc, redfish) plus shared
novnc.

The split makes networking and port mapping explicit, keeps images minimal, and
mirrors how IPMI + Redfish can exist independently in the real world.
