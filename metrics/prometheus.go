package metrics

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

// PrometheusInterface allows for mocking out the functionality of Prometheus when testing the full process of an apply run.
type PrometheusInterface interface {
	UpdateKubectlExitCodeCount(string, int)
	UpdateNamespaceSuccess(string, bool)
	UpdateRunLatency(float64, bool)
	UpdateResultSummary(map[string]string)
}

// Prometheus implements instrumentation of metrics for kube-applier.
// kubectlExitCodeCount is a Counter vector to increment the number of exit codes for each kubectl execution
// fileApplyCount is a Counter vector to increment the number of successful and failed apply attempts for each file in the repo.
// runLatency is a Summary vector that keeps track of the duration for apply runs.
type Prometheus struct {
	kubectlExitCodeCount *prometheus.CounterVec
	namespaceApplyCount  *prometheus.CounterVec
	runLatency           *prometheus.HistogramVec
	resultSummary        *prometheus.GaugeVec
}

// Init creates and registers the custom metrics for kube-applier.
func (p *Prometheus) Init() {
	p.kubectlExitCodeCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "kubectl_exit_code_count",
		Help: "Count of kubectl exit codes",
	},
		[]string{
			// Path of the file that was applied
			"namespace",
			// Exit code
			"exit_code",
		},
	)
	p.namespaceApplyCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "namespace_apply_count",
		Help: "Success metric for every namespace applied",
	},
		[]string{
			// Path of the file that was applied
			"namespace",
			// Result: true if the apply was successful, false otherwise
			"success",
		},
	)
	p.runLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "run_latency_seconds",
		Help: "Latency for completed apply runs",
	},
		[]string{
			// Result: true if the run was successful, false otherwise
			"success",
		},
	)
	p.resultSummary = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "result_summary",
		Help: "Result summary for every manifest",
	},
		[]string{
			// The object namespace
			"namespace",
			// The object type
			"type",
			// The object name
			"name",
			// The applied action
			"action",
		},
	)
	prometheus.MustRegister(p.kubectlExitCodeCount)
	prometheus.MustRegister(p.resultSummary)
	prometheus.MustRegister(p.namespaceApplyCount)
	prometheus.MustRegister(p.runLatency)
}

// UpdateKubectlExitCodeCount increments for each exit code returned by kubectl
func (p *Prometheus) UpdateKubectlExitCodeCount(file string, code int) {
	p.kubectlExitCodeCount.With(prometheus.Labels{
		"namespace": filepath.Base(file),
		"exit_code": strconv.Itoa(code),
	}).Inc()
}

// UpdateNamespaceSuccess increments the given namespace's Counter for either successful apply attempts or failed apply attempts.
func (p *Prometheus) UpdateNamespaceSuccess(file string, success bool) {
	p.namespaceApplyCount.With(prometheus.Labels{
		"namespace": filepath.Base(file), "success": strconv.FormatBool(success),
	}).Inc()
}

// UpdateRunLatency adds a data point (latency of the most recent run) to the run_latency_seconds Summary metric, with a tag indicating whether or not the run was successful.
func (p *Prometheus) UpdateRunLatency(runLatency float64, success bool) {
	p.runLatency.With(prometheus.Labels{
		"success": strconv.FormatBool(success),
	}).Observe(runLatency)
}

// UpdateResultSummary sets gauges for each deployment
func (p *Prometheus) UpdateResultSummary(failures map[string]string) {
	p.resultSummary.Reset()

	for filePath, output := range failures {
		res := parseKubectlOutput(output)
		for _, r := range res {
			p.resultSummary.With(prometheus.Labels{
				"namespace": filepath.Base(filePath),
				"type":      r.Type,
				"name":      r.Name,
				"action":    r.Action,
			}).Set(1)
		}
	}
}

// Result struct containing Type, Name and Action
type Result struct {
	Type, Name, Action string
}

func parseKubectlOutput(output string) []Result {
	lines := strings.Split(output, "\n")

	var results []Result
	for _, line := range lines {
		o := strings.Split(line, " ")
		if len(o) < 2 {
			continue
		}

		os := strings.Split(o[0], "/")
		results = append(results, Result{
			Type:   os[0],
			Name:   os[1],
			Action: o[1],
		})
	}

	return results
}
