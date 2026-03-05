# Funxy VMM Dashboard

An interactive terminal user interface (TUI) for monitoring and managing your Funxy Virtual Machine Manager (VMM) clusters.

## Features

- **Global View**: See all running Virtual Machines across all active VMM clusters running in the current directory.
- **Inspect**: View the internal state and stack trace of any running VM to debug issues.
- **Stats**: Monitor real-time metrics (executed CPU instructions, memory allocations, rates) for a specific VM.
- **Trace**: Live-refresh recent RPC trace events for a VM (direction, method, status, duration, trace id), with pause/resume and status filtering.
- **Uptime**: Show how long a VM has been running.
- **Stop**: Send a graceful shutdown request to a VM (triggers `onTerminate` hooks if configured).
- **Kill**: Forcefully terminate a stuck or unresponsive VM immediately.
- **Reload**: Trigger hot reload for a VM (reloads script without restart).

## Usage

First, ensure you have at least one VMM cluster running (either in the background or in another terminal window).

Then, launch the VMM Dashboard by providing the target directory where the cluster was launched (where the `.vmm*.pid` files are stored):

```bash
# To scan the current directory:
./funxy kit/vmmui .

# To scan a specific directory:
./funxy kit/vmmui /path/to/project
```

### Custom Socket Path

If the VMM was started with a custom `--socket` path, you can connect directly:

```bash
# Pass the socket file path directly:
./funxy kit/vmmui /var/run/funxy_vmm.sock

# Or use the --socket flag:
./funxy kit/vmmui . --socket /var/run/funxy_vmm.sock
```

### Navigation

- The interface is fully interactive.
- Type the **number** corresponding to the VM or menu action you want to select, and press **Enter**.
- Follow the on-screen prompts (e.g., typing `y`/`yes` for confirmations or pressing Enter to return to the main menu).

### Exiting

- **Exit Dashboard** — select from the main supervisor list to quit cleanly.
- **Enter** — in Inspect/Trace views, press Enter to go back to the VM list.
- **P** — in Trace view, pause/resume auto-refresh.
- **F** — in Trace view, cycle filter `all -> error -> fast_fail`.
- **R** — in Trace view, force manual refresh (useful while paused).
- **Ctrl+C** — force quit at any time.
