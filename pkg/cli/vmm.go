package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"github.com/funvibe/funxy/internal/evaluator"
	"github.com/funvibe/funxy/internal/ext"
	"github.com/funvibe/funxy/internal/modules"
	funxy "github.com/funvibe/funxy/pkg/embed"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func isProcessAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

func discoverScriptImports(scriptPath string) []string {
	seen := map[string]struct{}{}
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		return []string{}
	}
	re := regexp.MustCompile(`import\s+"([^"]+)"`)
	for _, m := range re.FindAllStringSubmatch(string(content), -1) {
		if len(m) >= 2 {
			seen[strings.TrimSpace(m[1])] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for cap := range seen {
		out = append(out, cap)
	}
	sort.Strings(out)
	return out
}

// handleVmm handles the "funxy vmm <script>" command, which runs a script
// as the Supervisor VM within the Funxy Virtual Machine Manager (VMM).
func handleVmm() bool {
	if len(os.Args) < 2 {
		return false
	}

	if os.Args[1] != "vmm" {
		return false
	}

	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: funxy vmm <supervisor_script.lang> [--pidfile <path>]\n")
		os.Exit(1)
	}

	var scriptPath string
	var adminCmd string
	var adminCmdArgs []string
	pidFile := ".vmm.pid"
	socketPath := filepath.Join(os.TempDir(), "funxy_vmm.sock")
	metricsPort := "9090"
	rpcSerialization := evaluator.SerializeModeAuto

	for i := 2; i < len(os.Args); i++ {
		if os.Args[i] == "--pidfile" {
			if i+1 < len(os.Args) {
				pidFile = os.Args[i+1]
				i++
			} else {
				fmt.Fprintf(os.Stderr, "Error: --pidfile requires a path argument\n")
				os.Exit(1)
			}
		} else if os.Args[i] == "--socket" {
			if i+1 < len(os.Args) {
				socketPath = os.Args[i+1]
				i++
			} else {
				fmt.Fprintf(os.Stderr, "Error: --socket requires a path argument\n")
				os.Exit(1)
			}
		} else if os.Args[i] == "--metrics-port" {
			if i+1 < len(os.Args) {
				metricsPort = os.Args[i+1]
				i++
			} else {
				fmt.Fprintf(os.Stderr, "Error: --metrics-port requires a port argument\n")
				os.Exit(1)
			}
		} else if os.Args[i] == "--rpc-serialization" {
			if i+1 < len(os.Args) {
				rpcSerialization = strings.ToLower(strings.TrimSpace(os.Args[i+1]))
				i++
			} else {
				fmt.Fprintf(os.Stderr, "Error: --rpc-serialization requires a mode (auto|fdf|ephemeral)\n")
				os.Exit(1)
			}
		} else if scriptPath == "" && adminCmd == "" {
			arg := os.Args[i]
			if arg == "ps" || arg == "kill" || arg == "stop" || arg == "stats" || arg == "inspect" || arg == "circuit" || arg == "trace" || arg == "uptime" || arg == "reload" {
				adminCmd = arg
			} else {
				scriptPath = arg
			}
		} else if adminCmd != "" {
			adminCmdArgs = append(adminCmdArgs, os.Args[i])
		}
	}

	if adminCmd != "" {
		if (adminCmd == "kill" || adminCmd == "stop" || adminCmd == "stats" || adminCmd == "inspect" || adminCmd == "circuit" || adminCmd == "uptime") && len(adminCmdArgs) == 0 {
			fmt.Fprintf(os.Stderr, "Usage: funxy vmm %s <id> [--socket <path>]\n", adminCmd)
			os.Exit(1)
		}
		isDefaultSocket := socketPath == filepath.Join(os.TempDir(), "funxy_vmm.sock")
		runAdminCommand(adminCmd, adminCmdArgs, socketPath, isDefaultSocket)
		return true
	}

	if scriptPath == "" {
		fmt.Fprintf(os.Stderr, "Usage: funxy vmm <supervisor_script.lang> [--pidfile <path>] [--socket <path>] [--metrics-port <port>] [--rpc-serialization <auto|fdf|ephemeral>]\n")
		fmt.Fprintf(os.Stderr, "       funxy vmm ps|stop|kill|stats|inspect|circuit|uptime <id> [--socket <path>]\n")
		fmt.Fprintf(os.Stderr, "       funxy vmm trace [<id>|all|--all] [--socket <path>]\n")
		fmt.Fprintf(os.Stderr, "       funxy vmm reload [id] [--socket <path>]\n")
		os.Exit(1)
	}

	if rpcSerialization != evaluator.SerializeModeAuto && rpcSerialization != evaluator.SerializeModeFDF && rpcSerialization != evaluator.SerializeModeEphemeral {
		fmt.Fprintf(os.Stderr, "Error: invalid --rpc-serialization mode '%s' (expected auto|fdf|ephemeral)\n", rpcSerialization)
		os.Exit(1)
	}

	// Initialize virtual packages and extensions
	modules.InitVirtualPackages()
	ext.RegisterVirtualPackagesFromRegistry()

	// Create Hypervisor
	h := funxy.NewHypervisor()
	h.SetRPCSerializationMode(rpcSerialization)

	h.RegisterCapabilityProvider(func(cap string, vm *funxy.VM) error {
		if cap == "supervisor" {
			vm.RegisterSupervisor(h)
			return nil
		}
		// In CLI vmm mode, capabilities are treated as allowed import paths.
		// Actual module validity is checked by normal import pipeline.
		return nil
	})

	// Configuration for the Supervisor VM
	config := map[string]interface{}{
		"name":         "supervisor",
		"capabilities": []interface{}{"supervisor"},
	}

	// Add all known lib/* and ext/* modules as capabilities to the supervisor
	// so it can selectively grant them to workers
	for libMod := range modules.GetAllVirtualPackages() {
		caps := config["capabilities"].([]interface{})
		if libMod == "lib" {
			continue // Skip the meta-package
		}
		// libMod is already in the format "lib/..." or "core/..."
		config["capabilities"] = append(caps, libMod)
	}
	for _, extMod := range evaluator.GetAllExtModules() {
		caps := config["capabilities"].([]interface{})
		config["capabilities"] = append(caps, "ext/"+extMod)
	}
	for _, imp := range discoverScriptImports(scriptPath) {
		caps := config["capabilities"].([]interface{})
		config["capabilities"] = append(caps, imp)
	}

	pid := os.Getpid()
	isDefaultPidFile := pidFile == ".vmm.pid"
	isDefaultSocketPath := socketPath == filepath.Join(os.TempDir(), "funxy_vmm.sock")

	needNew := false

	if isDefaultPidFile {
		if data, err := os.ReadFile(pidFile); err == nil {
			if existingPid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
				if isProcessAlive(existingPid) {
					needNew = true
				}
			}
		}
	}

	if isDefaultSocketPath && !needNew {
		conn, err := net.DialTimeout("unix", socketPath, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			needNew = true
		}
	}

	if needNew {
		if isDefaultPidFile {
			pidFile = fmt.Sprintf(".vmm-%d.pid", pid)
		}
		if isDefaultSocketPath {
			socketPath = filepath.Join(os.TempDir(), fmt.Sprintf("funxy_vmm_%d.sock", pid))
		}
	}

	fmt.Printf("Booting Funxy VMM Supervisor (%s)...\n", scriptPath)
	fmt.Printf("VMM Host PID: %d (use `kill -SIGUSR1 %d` to trigger hot-reload)\n", pid, pid)

	// Write PID file for external tooling
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", pid)), 0644); err == nil {
		defer func() {
			if data, err := os.ReadFile(pidFile); err == nil && string(data) == fmt.Sprintf("%d", pid) {
				os.Remove(pidFile)
			}
		}()
	}

	startAdminServer(h, socketPath)
	defer func() {
		if data, err := os.ReadFile(pidFile); err == nil && string(data) == fmt.Sprintf("%d", pid) {
			os.Remove(socketPath)
		}
	}()

	// Start Prometheus metrics server
	startMetricsServer(h, metricsPort)

	id, err := h.SpawnVM(scriptPath, config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error spawning supervisor VM: %v\n", err)
		// Spawn failure happens before the normal shutdown path. Clean up pid/socket explicitly
		// because os.Exit bypasses deferred cleanup.
		if data, readErr := os.ReadFile(pidFile); readErr == nil && strings.TrimSpace(string(data)) == fmt.Sprintf("%d", pid) {
			_ = os.Remove(pidFile)
		}
		_ = os.Remove(socketPath)
		os.Exit(1)
	}

	// Setup signal handling for graceful shutdown and hot-reload
	sigCh := make(chan os.Signal, 1)
	vmmNotifySignals(sigCh)

	// Block until a signal is received or supervisor exits (use WaitForExit to avoid stealing child events)
	exitCh := make(chan bool)
	go func() {
		<-h.WaitForExit(id)
		fmt.Printf("\nSupervisor VM '%s' exited. Shutting down host...\n", id)
		exitCh <- true
	}()

	// Wait while running or until supervisor exits
	// In the CLI, we run the script via RunChunk which returns when the script ends.
	// But the user can also use the CLI admin commands on another terminal to stop VMs.

	for {
		select {
		case sig := <-sigCh:
			if vmmIsHotReloadSignal(sig) {
				fmt.Println("Received SIGUSR1. Broadcasting hot_reload event...")
				h.BroadcastEvent(map[string]interface{}{
					"type": "hot_reload",
				})
			} else {
				fmt.Printf("\nReceived signal %s. Shutting down Supervisor VM...\n", sig)
				_, killErr := h.KillVM(id, true, 5000)
				if killErr != nil {
					fmt.Fprintf(os.Stderr, "Error shutting down supervisor VM: %v\n", killErr)
					os.Exit(1)
				}
				fmt.Println("VMM shutdown complete.")
				return true
			}
		case <-exitCh:
			return true
		}
	}
}

func getActiveSockets(defaultSocketPath string, isDefault bool) []string {
	if !isDefault {
		return []string{defaultSocketPath}
	}

	var sockets []string

	// Read all .vmm*.pid files in current directory
	files, err := os.ReadDir(".")
	if err != nil {
		return []string{defaultSocketPath} // fallback
	}

	foundAny := false
	for _, f := range files {
		if !f.IsDir() && strings.HasPrefix(f.Name(), ".vmm") && strings.HasSuffix(f.Name(), ".pid") {
			data, err := os.ReadFile(f.Name())
			if err == nil {
				if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
					if isProcessAlive(pid) {
						foundAny = true
						// deduce socket path
						if f.Name() == ".vmm.pid" {
							sockets = append(sockets, filepath.Join(os.TempDir(), "funxy_vmm.sock"))
						} else {
							// .vmm-<pid>.pid
							sockets = append(sockets, filepath.Join(os.TempDir(), fmt.Sprintf("funxy_vmm_%d.sock", pid)))
						}
					} else {
						// Clean up stale pid file
						os.Remove(f.Name())
					}
				}
			}
		}
	}

	if !foundAny {
		// Just return the default so it prints a normal error
		return []string{defaultSocketPath}
	}

	return sockets
}

func sendAdminRequest(cmd string, cmdArgs []string, socketPath string) ([]byte, int, error) {
	client := http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
	}

	url := "http://unix/" + cmd
	if len(cmdArgs) > 0 {
		url += "?id=" + cmdArgs[0]
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	return body, resp.StatusCode, nil
}

func formatUptime(sec int64) string {
	if sec < 60 {
		return fmt.Sprintf("%ds", sec)
	}
	if sec < 3600 {
		return fmt.Sprintf("%dm%ds", sec/60, sec%60)
	}
	if sec < 86400 {
		return fmt.Sprintf("%dh%dm%ds", sec/3600, (sec%3600)/60, sec%60)
	}
	return fmt.Sprintf("%dd%dh%dm%ds", sec/86400, (sec%86400)/3600, (sec%3600)/60, sec%60)
}

func printAdminResponse(cmd string, cmdArgs []string, body []byte) {
	if cmd == "stats" {
		var stats map[string]uint64
		if err := json.Unmarshal(body, &stats); err == nil {
			fmt.Printf("Stats for %s:\n", cmdArgs[0])
			var keys []string
			for k := range stats {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Printf("- %s: %d\n", k, stats[k])
			}
		} else {
			fmt.Println(string(body))
		}
	} else if cmd == "uptime" {
		var info map[string]interface{}
		if err := json.Unmarshal(body, &info); err == nil {
			var sec int64
			switch v := info["uptime_seconds"].(type) {
			case float64:
				sec = int64(v)
			case int64:
				sec = v
			case int:
				sec = int64(v)
			default:
				fmt.Println(string(body))
				return
			}
			fmt.Printf("Uptime: %s\n", formatUptime(sec))
		} else {
			fmt.Println(string(body))
		}
	} else if cmd == "inspect" {
		var info map[string]interface{}
		if err := json.Unmarshal(body, &info); err == nil {
			fmt.Printf("Inspection for VM '%s':\n", cmdArgs[0])
			if mode, ok := info["rpc_serialization_mode"].(string); ok && mode != "" {
				fmt.Printf("RPC serialization mode: %s\n", mode)
			}
			if stats, ok := info["stats"].(map[string]interface{}); ok {
				fmt.Printf("Stats:\n")
				var keys []string
				for k := range stats {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					fmt.Printf("  %s: %v\n", k, stats[k])
				}
			}
			if stackTrace, ok := info["stack_trace"].(string); ok {
				fmt.Printf("\nStack Trace:\n%s\n", stackTrace)
			}
		} else {
			fmt.Println(string(body))
		}
	} else if cmd == "circuit" {
		var info map[string]interface{}
		if err := json.Unmarshal(body, &info); err == nil {
			fmt.Printf("RPC circuit for VM '%s':\n", cmdArgs[0])
			var keys []string
			for k := range info {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Printf("- %s: %v\n", k, info[k])
			}
		} else {
			fmt.Println(string(body))
		}
	} else {
		fmt.Println(strings.TrimSpace(string(body)))
	}
}

func parseTraceTarget(cmdArgs []string) (string, bool) {
	if len(cmdArgs) == 0 {
		return "", true
	}
	arg := strings.TrimSpace(cmdArgs[0])
	if arg == "" || arg == "all" || arg == "--all" {
		return "", true
	}
	return arg, false
}

func runAdminCommand(cmd string, cmdArgs []string, socketPath string, isDefaultSocket bool) {
	sockets := getActiveSockets(socketPath, isDefaultSocket)

	if cmd == "trace" {
		vmID, streamAll := parseTraceTarget(cmdArgs)
		var lastErr error
		for _, sock := range sockets {
			err := streamTrace(sock, vmID, streamAll)
			if err == nil {
				return
			}
			lastErr = err
		}
		if lastErr != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", lastErr)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Error: VM not found in any active cluster\n")
		os.Exit(1)
	}

	if cmd == "ps" {
		var allVms []string
		for _, sock := range sockets {
			body, statusCode, err := sendAdminRequest(cmd, cmdArgs, sock)
			if err != nil || statusCode != http.StatusOK {
				continue
			}
			var vms []string
			if err := json.Unmarshal(body, &vms); err == nil {
				allVms = append(allVms, vms...)
			}
		}
		sort.Strings(allVms)
		fmt.Println("Running VMs:")
		for _, id := range allVms {
			fmt.Println("- " + id)
		}
		return
	}

	var lastErrBody string
	var lastErr error

	for _, sock := range sockets {
		body, statusCode, err := sendAdminRequest(cmd, cmdArgs, sock)
		if err != nil {
			lastErr = err
			continue
		}
		if statusCode == http.StatusOK {
			printAdminResponse(cmd, cmdArgs, body)
			return
		} else {
			lastErrBody = strings.TrimSpace(string(body))
		}
	}

	if lastErrBody != "" {
		fmt.Fprintf(os.Stderr, "Error: %s\n", lastErrBody)
		os.Exit(1)
	} else if lastErr != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to connect to VMM: %v\n", lastErr)
		os.Exit(1)
	} else {
		fmt.Fprintf(os.Stderr, "Error: VM not found in any active cluster\n")
		os.Exit(1)
	}
}

func streamTrace(socketPath, vmID string, streamAll bool) error {
	client := http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
	}
	url := "http://unix/trace"
	if !streamAll && vmID != "" {
		url += "?id=" + vmID
	}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return errors.New(strings.TrimSpace(string(body)))
	}
	_, _ = io.Copy(os.Stdout, resp.Body)
	return nil
}

func startAdminServer(h *funxy.Hypervisor, socketPath string) {
	// remove old socket if exists
	os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to start admin socket at %s: %v\n", socketPath, err)
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ps", func(w http.ResponseWriter, r *http.Request) {
		vms := h.ListVMs()
		json.NewEncoder(w).Encode(vms)
	})
	mux.HandleFunc("/kill", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "missing id parameter", http.StatusBadRequest)
			return
		}
		_, err := h.KillVM(id, false, 1000)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		fmt.Fprintf(w, "Killed VM '%s'\n", id)
	})
	mux.HandleFunc("/stop", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "missing id parameter", http.StatusBadRequest)
			return
		}
		_, err := h.KillVM(id, true, 5000)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		fmt.Fprintf(w, "Stopped VM '%s'\n", id)
	})
	mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "missing id parameter", http.StatusBadRequest)
			return
		}
		stats, err := h.GetStats(id)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		json.NewEncoder(w).Encode(stats)
	})

	mux.HandleFunc("/inspect", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "missing id parameter", http.StatusBadRequest)
			return
		}

		info, err := h.InspectVM(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(info)
	})
	mux.HandleFunc("/circuit", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "missing id parameter", http.StatusBadRequest)
			return
		}

		stats, err := h.GetRPCCircuitStats(id)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		json.NewEncoder(w).Encode(stats)
	})
	mux.HandleFunc("/trace", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id != "" {
			if _, err := h.InspectVM(id); err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		ch, unsubscribe := h.SubscribeRPCTrace()
		defer unsubscribe()
		for {
			select {
			case <-r.Context().Done():
				return
			case evt, ok := <-ch:
				if !ok {
					return
				}
				if id != "" && evt.FromVM != id && evt.ToVM != id {
					continue
				}
				fmt.Fprintln(w, formatTraceLine(evt, id))
				flusher.Flush()
			}
		}
	})
	mux.HandleFunc("/trace_on", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" {
			h.TraceOnAll()
			fmt.Fprintln(w, "trace enabled for all VMs")
			return
		}
		if err := h.TraceOn(id); err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		fmt.Fprintf(w, "trace enabled for VM '%s'\n", id)
	})
	mux.HandleFunc("/trace_off", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" {
			h.TraceOffAll()
			fmt.Fprintln(w, "trace disabled for all VMs")
			return
		}
		if err := h.TraceOff(id); err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		fmt.Fprintf(w, "trace disabled for VM '%s'\n", id)
	})
	mux.HandleFunc("/trace_recent", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "missing id parameter", http.StatusBadRequest)
			return
		}
		limit := 50
		if ls := r.URL.Query().Get("limit"); ls != "" {
			if n, err := strconv.Atoi(ls); err == nil && n > 0 && n <= 1000 {
				limit = n
			}
		}
		if _, err := h.InspectVM(id); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(h.GetRPCTraceRecent(id, limit))
	})
	mux.HandleFunc("/uptime", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "missing id parameter", http.StatusBadRequest)
			return
		}
		info, err := h.InspectVM(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		out := map[string]interface{}{}
		if s, ok := info["uptime_seconds"]; ok {
			out["uptime_seconds"] = s
		}
		if s, ok := info["started_at"]; ok {
			out["started_at"] = s
		}
		json.NewEncoder(w).Encode(out)
	})

	mux.HandleFunc("/reload", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		evt := map[string]interface{}{
			"type": "hot_reload",
		}
		if id != "" {
			evt["vmId"] = id
		}
		h.BroadcastEvent(evt)
		if id != "" {
			fmt.Fprintf(w, "Hot reload triggered for VM '%s'\n", id)
		} else {
			fmt.Fprintf(w, "Hot reload triggered for all VMs\n")
		}
	})

	go http.Serve(listener, mux)
}

func formatTraceLine(evt funxy.RPCTraceEvent, vmID string) string {
	arg := evt.ArgPreview
	if arg == "" {
		arg = "nil"
	}
	if vmID == "" {
		return fmt.Sprintf("[TRACE] %s -> %s rpc:%s(%s) trace=%s status=%s dur=%dms",
			evt.FromVM, evt.ToVM, evt.Method, arg, evt.TraceID, evt.Status, evt.DurationMs)
	}
	if evt.ToVM == vmID {
		return fmt.Sprintf("[TRACE] %s <- rpc:%s(%s) from %s trace=%s status=%s dur=%dms",
			vmID, evt.Method, arg, evt.FromVM, evt.TraceID, evt.Status, evt.DurationMs)
	}
	return fmt.Sprintf("[TRACE] %s -> rpc:%s(%s) to %s trace=%s status=%s dur=%dms",
		vmID, evt.Method, arg, evt.ToVM, evt.TraceID, evt.Status, evt.DurationMs)
}

func startMetricsServer(h *funxy.Hypervisor, port string) {
	// Start HTTP server
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", NewMetricsHandler(h))

		// JSON stats endpoint for specific VM
		// Usage: /stats?id=<vm_id>
		mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
			id := r.URL.Query().Get("id")
			if id == "" {
				http.Error(w, "missing id parameter", http.StatusBadRequest)
				return
			}
			stats, err := h.GetStats(id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(stats)
		})

		// JSON inspect endpoint for specific VM
		mux.HandleFunc("/inspect", func(w http.ResponseWriter, r *http.Request) {
			id := r.URL.Query().Get("id")
			if id == "" {
				http.Error(w, "missing id parameter", http.StatusBadRequest)
				return
			}
			info, err := h.InspectVM(id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(info)
		})
		mux.HandleFunc("/circuit", func(w http.ResponseWriter, r *http.Request) {
			id := r.URL.Query().Get("id")
			if id == "" {
				http.Error(w, "missing id parameter", http.StatusBadRequest)
				return
			}
			stats, err := h.GetRPCCircuitStats(id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(stats)
		})
		mux.HandleFunc("/trace_recent", func(w http.ResponseWriter, r *http.Request) {
			id := r.URL.Query().Get("id")
			if id == "" {
				http.Error(w, "missing id parameter", http.StatusBadRequest)
				return
			}
			limit := 50
			if ls := r.URL.Query().Get("limit"); ls != "" {
				if n, err := strconv.Atoi(ls); err == nil && n > 0 && n <= 1000 {
					limit = n
				}
			}
			if _, err := h.InspectVM(id); err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(h.GetRPCTraceRecent(id, limit))
		})
		mux.HandleFunc("/trace_on", func(w http.ResponseWriter, r *http.Request) {
			id := r.URL.Query().Get("id")
			if id == "" {
				h.TraceOnAll()
				fmt.Fprintln(w, "trace enabled for all VMs")
				return
			}
			if err := h.TraceOn(id); err != nil {
				if strings.Contains(err.Error(), "not found") {
					http.Error(w, err.Error(), http.StatusNotFound)
				} else {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
				return
			}
			fmt.Fprintf(w, "trace enabled for VM '%s'\n", id)
		})
		mux.HandleFunc("/trace_off", func(w http.ResponseWriter, r *http.Request) {
			id := r.URL.Query().Get("id")
			if id == "" {
				h.TraceOffAll()
				fmt.Fprintln(w, "trace disabled for all VMs")
				return
			}
			if err := h.TraceOff(id); err != nil {
				if strings.Contains(err.Error(), "not found") {
					http.Error(w, err.Error(), http.StatusNotFound)
				} else {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
				return
			}
			fmt.Fprintf(w, "trace disabled for VM '%s'\n", id)
		})

		addr := ":" + port
		fmt.Printf("Metrics server listening on %s/metrics\n", addr)
		if err := http.ListenAndServe(addr, mux); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to start metrics server: %v\n", err)
		}
	}()
}
