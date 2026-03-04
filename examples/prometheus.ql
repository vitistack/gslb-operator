# worker-pool size
worker_pool_size_total

# number of service groups
service_groups_total

# number of registered services
sum(service_group_members)

# number of health checks in the last <interval>
sum(increase(healthcheck_total[$__rate_interval]))

# average health check duration towards each datacenter
sum by(datacenter) (rate(healthcheck_duration_ms_sum[5m])) / sum by(datacenter) (rate(healthcheck_duration_ms_count[5m]))

# health check success rate percentage towards each datacenter
(sum by(datacenter) (rate(healthcheck_total{status="success"}[$__rate_interval]))) * 100 / (sum by(datacenter) (rate(healthcheck_total[$__rate_interval])))
