package cli

import (
	"fmt"
	"net/http"
	funxy "github.com/funvibe/funxy/pkg/embed"
	"sort"
)

// MetricsHandler implements http.Handler to expose VM stats in Prometheus format
type MetricsHandler struct {
	hypervisor *funxy.Hypervisor
}

// NewMetricsHandler creates a new handler for the given hypervisor
func NewMetricsHandler(h *funxy.Hypervisor) *MetricsHandler {
	return &MetricsHandler{
		hypervisor: h,
	}
}

// ServeHTTP implements http.Handler
func (h *MetricsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	vms := h.hypervisor.ListVMs()
	// Sort for consistent output
	sort.Strings(vms)

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")

	// Instructions
	fmt.Fprintln(w, "# HELP funxy_vm_instructions_total Total instructions executed by the VM")
	fmt.Fprintln(w, "# TYPE funxy_vm_instructions_total counter")
	for _, id := range vms {
		stats, err := h.hypervisor.GetStats(id)
		if err != nil {
			continue
		}
		if inst, ok := stats["instructions"]; ok {
			fmt.Fprintf(w, "funxy_vm_instructions_total{vm_id=\"%s\"} %d\n", id, inst)
		}
	}
	fmt.Fprintln(w)

	// Allocations
	fmt.Fprintln(w, "# HELP funxy_vm_allocations_total Total memory allocations by the VM")
	fmt.Fprintln(w, "# TYPE funxy_vm_allocations_total counter")
	for _, id := range vms {
		stats, err := h.hypervisor.GetStats(id)
		if err != nil {
			continue
		}
		if allocs, ok := stats["allocations"]; ok {
			fmt.Fprintf(w, "funxy_vm_allocations_total{vm_id=\"%s\"} %d\n", id, allocs)
		}
	}
	fmt.Fprintln(w)

	// Rate metrics
	fmt.Fprintln(w, "# HELP funxy_vm_instructions_per_sec Current instructions per second rate")
	fmt.Fprintln(w, "# TYPE funxy_vm_instructions_per_sec gauge")
	for _, id := range vms {
		if stats, err := h.hypervisor.GetStats(id); err == nil {
			if val, ok := stats["current_instructions_per_sec"]; ok {
				fmt.Fprintf(w, "funxy_vm_instructions_per_sec{vm_id=\"%s\"} %d\n", id, val)
			}
		}
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "# HELP funxy_vm_allocations_bytes_per_sec Current memory allocations per second rate")
	fmt.Fprintln(w, "# TYPE funxy_vm_allocations_bytes_per_sec gauge")
	for _, id := range vms {
		if stats, err := h.hypervisor.GetStats(id); err == nil {
			if val, ok := stats["current_allocations_bytes_per_sec"]; ok {
				fmt.Fprintf(w, "funxy_vm_allocations_bytes_per_sec{vm_id=\"%s\"} %d\n", id, val)
			}
		}
	}
	fmt.Fprintln(w)

	// Limits
	fmt.Fprintln(w, "# HELP funxy_vm_limit_instructions Configured maximum instructions (gas limit)")
	fmt.Fprintln(w, "# TYPE funxy_vm_limit_instructions gauge")
	for _, id := range vms {
		if stats, err := h.hypervisor.GetStats(id); err == nil {
			if val, ok := stats["limit_instructions"]; ok {
				fmt.Fprintf(w, "funxy_vm_limit_instructions{vm_id=\"%s\"} %d\n", id, val)
			}
		}
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "# HELP funxy_vm_limit_instructions_per_sec Configured rate limit for instructions")
	fmt.Fprintln(w, "# TYPE funxy_vm_limit_instructions_per_sec gauge")
	for _, id := range vms {
		if stats, err := h.hypervisor.GetStats(id); err == nil {
			if val, ok := stats["limit_instructions_per_sec"]; ok {
				fmt.Fprintf(w, "funxy_vm_limit_instructions_per_sec{vm_id=\"%s\"} %d\n", id, val)
			}
		}
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "# HELP funxy_vm_limit_allocations_bytes Configured maximum memory allocations in bytes")
	fmt.Fprintln(w, "# TYPE funxy_vm_limit_allocations_bytes gauge")
	for _, id := range vms {
		if stats, err := h.hypervisor.GetStats(id); err == nil {
			if val, ok := stats["limit_allocations_bytes"]; ok {
				fmt.Fprintf(w, "funxy_vm_limit_allocations_bytes{vm_id=\"%s\"} %d\n", id, val)
			}
		}
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "# HELP funxy_vm_limit_allocations_bytes_per_sec Configured rate limit for memory allocations in bytes")
	fmt.Fprintln(w, "# TYPE funxy_vm_limit_allocations_bytes_per_sec gauge")
	for _, id := range vms {
		if stats, err := h.hypervisor.GetStats(id); err == nil {
			if val, ok := stats["limit_allocations_bytes_per_sec"]; ok {
				fmt.Fprintf(w, "funxy_vm_limit_allocations_bytes_per_sec{vm_id=\"%s\"} %d\n", id, val)
			}
		}
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "# HELP funxy_vm_limit_stack_depth Configured maximum call stack depth")
	fmt.Fprintln(w, "# TYPE funxy_vm_limit_stack_depth gauge")
	for _, id := range vms {
		if stats, err := h.hypervisor.GetStats(id); err == nil {
			if val, ok := stats["limit_stack_depth"]; ok {
				fmt.Fprintf(w, "funxy_vm_limit_stack_depth{vm_id=\"%s\"} %d\n", id, val)
			}
		}
	}
	fmt.Fprintln(w)

	// Mailbox Metrics
	fmt.Fprintln(w, "# HELP funxy_mailbox_messages_received_total Total mailbox messages received")
	fmt.Fprintln(w, "# TYPE funxy_mailbox_messages_received_total counter")
	for _, id := range vms {
		if stats, err := h.hypervisor.GetStats(id); err == nil {
			if val, ok := stats["mailbox_messages_received_total"]; ok {
				fmt.Fprintf(w, "funxy_mailbox_messages_received_total{vm_id=\"%s\"} %d\n", id, val)
			}
		}
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "# HELP funxy_mailbox_queue_size Current mailbox queue size")
	fmt.Fprintln(w, "# TYPE funxy_mailbox_queue_size gauge")
	for _, id := range vms {
		if stats, err := h.hypervisor.GetStats(id); err == nil {
			if val, ok := stats["mailbox_queue_size"]; ok {
				fmt.Fprintf(w, "funxy_mailbox_queue_size{vm_id=\"%s\"} %d\n", id, val)
			}
		}
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "# HELP funxy_mailbox_queue_capacity Mailbox queue capacity")
	fmt.Fprintln(w, "# TYPE funxy_mailbox_queue_capacity gauge")
	for _, id := range vms {
		if stats, err := h.hypervisor.GetStats(id); err == nil {
			if val, ok := stats["mailbox_queue_capacity"]; ok {
				fmt.Fprintf(w, "funxy_mailbox_queue_capacity{vm_id=\"%s\"} %d\n", id, val)
			}
		}
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "# HELP funxy_mailbox_drops_total Total mailbox messages dropped")
	fmt.Fprintln(w, "# TYPE funxy_mailbox_drops_total counter")
	for _, id := range vms {
		if stats, err := h.hypervisor.GetStats(id); err == nil {
			if val, ok := stats["mailbox_drops_total_full_old"]; ok {
				fmt.Fprintf(w, "funxy_mailbox_drops_total{vm_id=\"%s\",reason=\"full_old\"} %d\n", id, val)
			}
			if val, ok := stats["mailbox_drops_total_full_new"]; ok {
				fmt.Fprintf(w, "funxy_mailbox_drops_total{vm_id=\"%s\",reason=\"full_new\"} %d\n", id, val)
			}
			if val, ok := stats["mailbox_drops_total_importance"]; ok {
				fmt.Fprintf(w, "funxy_mailbox_drops_total{vm_id=\"%s\",reason=\"importance\"} %d\n", id, val)
			}
		}
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "# HELP funxy_mailbox_block_duration_ms_total Total block duration in ms for sendWait")
	fmt.Fprintln(w, "# TYPE funxy_mailbox_block_duration_ms_total counter")
	for _, id := range vms {
		if stats, err := h.hypervisor.GetStats(id); err == nil {
			if val, ok := stats["mailbox_block_duration_ms_total"]; ok {
				fmt.Fprintf(w, "funxy_mailbox_block_duration_ms_total{vm_id=\"%s\"} %d\n", id, val)
			}
		}
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "# HELP funxy_mailbox_selective_scan_depth_total Total depth of selective receive scans")
	fmt.Fprintln(w, "# TYPE funxy_mailbox_selective_scan_depth_total counter")
	for _, id := range vms {
		if stats, err := h.hypervisor.GetStats(id); err == nil {
			if val, ok := stats["mailbox_selective_scan_depth_total"]; ok {
				fmt.Fprintf(w, "funxy_mailbox_selective_scan_depth_total{vm_id=\"%s\"} %d\n", id, val)
			}
		}
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "# HELP funxy_mailbox_skipped_messages Total messages skipped during selective receive")
	fmt.Fprintln(w, "# TYPE funxy_mailbox_skipped_messages counter")
	for _, id := range vms {
		if stats, err := h.hypervisor.GetStats(id); err == nil {
			if val, ok := stats["mailbox_skipped_messages"]; ok {
				fmt.Fprintf(w, "funxy_mailbox_skipped_messages{vm_id=\"%s\"} %d\n", id, val)
			}
		}
	}
	fmt.Fprintln(w)

	// Hypervisor event queue metrics
	fmt.Fprintln(w, "# HELP funxy_vmm_event_queue_size Current hypervisor event queue size")
	fmt.Fprintln(w, "# TYPE funxy_vmm_event_queue_size gauge")
	fmt.Fprintf(w, "funxy_vmm_event_queue_size %d\n", h.hypervisor.EventQueueSize())
	fmt.Fprintln(w)

	fmt.Fprintln(w, "# HELP funxy_vmm_event_queue_capacity Hypervisor event queue capacity")
	fmt.Fprintln(w, "# TYPE funxy_vmm_event_queue_capacity gauge")
	fmt.Fprintf(w, "funxy_vmm_event_queue_capacity %d\n", h.hypervisor.EventQueueCapacity())
	fmt.Fprintln(w)

	fmt.Fprintln(w, "# HELP funxy_vmm_event_queue_dropped_total Total events dropped due to full queue")
	fmt.Fprintln(w, "# TYPE funxy_vmm_event_queue_dropped_total counter")
	fmt.Fprintf(w, "funxy_vmm_event_queue_dropped_total %d\n", h.hypervisor.EventDroppedTotal())
	fmt.Fprintln(w)

	// RPC Circuit breaker diagnostics
	fmt.Fprintln(w, "# HELP funxy_vmm_rpc_circuit_state RPC circuit state per VM (one-hot by state label)")
	fmt.Fprintln(w, "# TYPE funxy_vmm_rpc_circuit_state gauge")
	for _, id := range vms {
		cs, err := h.hypervisor.GetRPCCircuitStats(id)
		if err != nil {
			continue
		}
		closed := 0
		open := 0
		halfOpen := 0
		switch cs.State {
		case "open":
			open = 1
		case "half_open":
			halfOpen = 1
		default:
			closed = 1
		}
		fmt.Fprintf(w, "funxy_vmm_rpc_circuit_state{vm_id=\"%s\",state=\"closed\"} %d\n", id, closed)
		fmt.Fprintf(w, "funxy_vmm_rpc_circuit_state{vm_id=\"%s\",state=\"open\"} %d\n", id, open)
		fmt.Fprintf(w, "funxy_vmm_rpc_circuit_state{vm_id=\"%s\",state=\"half_open\"} %d\n", id, halfOpen)
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "# HELP funxy_vmm_rpc_circuit_failures_window Current failure count in breaker window")
	fmt.Fprintln(w, "# TYPE funxy_vmm_rpc_circuit_failures_window gauge")
	for _, id := range vms {
		cs, err := h.hypervisor.GetRPCCircuitStats(id)
		if err != nil {
			continue
		}
		fmt.Fprintf(w, "funxy_vmm_rpc_circuit_failures_window{vm_id=\"%s\"} %d\n", id, cs.FailureCount)
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "# HELP funxy_vmm_rpc_circuit_fast_fail_total Total fast-fail rejections due to open/half-open breaker")
	fmt.Fprintln(w, "# TYPE funxy_vmm_rpc_circuit_fast_fail_total counter")
	for _, id := range vms {
		cs, err := h.hypervisor.GetRPCCircuitStats(id)
		if err != nil {
			continue
		}
		fmt.Fprintf(w, "funxy_vmm_rpc_circuit_fast_fail_total{vm_id=\"%s\"} %d\n", id, cs.FastFailTotal)
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "# HELP funxy_vmm_rpc_circuit_transitions_total Total circuit state transitions by destination state")
	fmt.Fprintln(w, "# TYPE funxy_vmm_rpc_circuit_transitions_total counter")
	for _, id := range vms {
		cs, err := h.hypervisor.GetRPCCircuitStats(id)
		if err != nil {
			continue
		}
		fmt.Fprintf(w, "funxy_vmm_rpc_circuit_transitions_total{vm_id=\"%s\",to=\"open\"} %d\n", id, cs.TransitionsOpenTotal)
		fmt.Fprintf(w, "funxy_vmm_rpc_circuit_transitions_total{vm_id=\"%s\",to=\"half_open\"} %d\n", id, cs.TransitionsHalfOpenTotal)
		fmt.Fprintf(w, "funxy_vmm_rpc_circuit_transitions_total{vm_id=\"%s\",to=\"closed\"} %d\n", id, cs.TransitionsClosedTotal)
	}
	fmt.Fprintln(w)
}
