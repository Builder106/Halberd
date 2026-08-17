# Halberd Roadmap

High-level threat intelligence and honeypot development roadmap.

## v1.1 — High-Fidelity Decoy Protocols

- **Expanded Service Emulation**: Low-interaction emulators for SSH, Telnet, HTTP, and Redis protocol handshakes.
- **eBPF Kernel Telemetry**: Low-overhead system call interception for real-time payload capture.

## v1.2 — Automated Threat Intelligence Feeds

- **STIX/TAXII Export Pipeline**: Structured threat intelligence sharing format for captured attacker IP reputations.
- **Dynamic Web Command Center**: Live attack map and intrusion visualization dashboard.

## Out of Scope

- Active counter-offensive / retaliatory operations
- Kernel-space modification outside validated eBPF bytecode

---
For technical specifications, see [`docs/specs/`](docs/specs/).
