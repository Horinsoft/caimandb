// Package metrics defines and registers CaimanDB's Prometheus metrics.
// Every other package (including the root caimandb package) increments/
// observes these directly; this package owns only their definition and
// one-time registration (InitMetrics).
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sync"
)

var (
	MetricOpsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "caimandb_operations_total",
			Help: "Total number of operations",
		},
		[]string{"type", "db", "block", "shard", "node", "status"},
	)

	MetricOpsDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "caimandb_operation_duration_seconds",
			Help:    "Duration of operations",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 20),
		},
		[]string{"type", "db", "block", "shard", "node"},
	)

	MetricCacheHits = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "caimandb_cache_hits_total",
			Help: "Total cache hits",
		},
	)

	MetricCacheMisses = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "caimandb_cache_misses_total",
			Help: "Total cache misses",
		},
	)

	MetricActiveQueries = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "caimandb_active_queries",
			Help: "Number of active queries",
		},
	)

	MetricIndexSize = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "caimandb_index_size_bytes",
			Help: "Size of secondary indexes",
		},
		[]string{"db", "block", "field", "shard", "node"},
	)

	MetricDocumentsTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "caimandb_documents_total",
			Help: "Total number of documents",
		},
		[]string{"db", "block", "shard", "node"},
	)

	MetricShardDistribution = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "caimandb_shard_distribution",
			Help: "Distribution of documents across shards",
		},
		[]string{"db", "block", "shard", "node"},
	)

	MetricClusterStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "caimandb_cluster_status",
			Help: "Cluster node status",
		},
		[]string{"node", "role"},
	)

	MetricRaftCommits = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "caimandb_raft_commits_total",
			Help: "Total Raft commits",
		},
	)

	MetricAutoScaleEvents = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "caimandb_autoscale_events_total",
			Help: "Total auto-scale events",
		},
	)

	MetricWALWrites = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "caimandb_wal_writes_total",
			Help: "Total WAL writes",
		},
	)

	MetricCompactions = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "caimandb_compactions_total",
			Help: "Total compactions performed",
		},
	)

	MetricTransactionCommits = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "caimandb_transaction_commits_total",
			Help: "Total transaction commits",
		},
	)

	MetricTransactionRollbacks = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "caimandb_transaction_rollbacks_total",
			Help: "Total transaction rollbacks",
		},
	)

	MetricStartupTime = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "caimandb_startup_duration_seconds",
			Help: "Time taken to start up",
		},
	)

	MetricDocSize = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "caimandb_document_size_bytes",
			Help:    "Size of documents inserted",
			Buckets: []float64{1024, 10240, 102400, 1048576, 5242880, 10485760, 52428800},
		},
	)

	MetricQueryLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "caimandb_query_latency_seconds",
			Help:    "Query latency",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 15),
		},
		[]string{"db", "block", "type"},
	)

	MetricMemoryUsage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "caimandb_memory_usage_bytes",
			Help: "Memory usage in bytes",
		},
	)

	MetricDiskUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "caimandb_disk_usage_bytes",
			Help: "Disk usage in bytes",
		},
		[]string{"db", "type"},
	)

	MetricAuthFailures = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "caimandb_auth_failures_total",
			Help: "Total authentication failures",
		},
	)

	MetricWALSize = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "caimandb_wal_size_bytes",
			Help: "WAL size in bytes",
		},
	)

	MetricRiskBlocks = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "caimandb_risk_blocks_total",
			Help: "Total operations blocked by risk engine",
		},
	)

	MetricChangeEventsDropped = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "caimandb_change_events_dropped_total",
			Help: "Total real-time change-stream events dropped due to a slow/full subscriber",
		},
	)

	metricsOnce sync.Once
)

func InitMetrics() {
	metricsOnce.Do(func() {
		prometheus.MustRegister(
			MetricOpsTotal,
			MetricOpsDuration,
			MetricCacheHits,
			MetricCacheMisses,
			MetricActiveQueries,
			MetricIndexSize,
			MetricDocumentsTotal,
			MetricShardDistribution,
			MetricClusterStatus,
			MetricRaftCommits,
			MetricAutoScaleEvents,
			MetricWALWrites,
			MetricCompactions,
			MetricTransactionCommits,
			MetricTransactionRollbacks,
			MetricStartupTime,
			MetricDocSize,
			MetricQueryLatency,
			MetricMemoryUsage,
			MetricDiskUsage,
			MetricAuthFailures,
			MetricWALSize,
			MetricRiskBlocks,
			MetricChangeEventsDropped,
		)
	})
}
