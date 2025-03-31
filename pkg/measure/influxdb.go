package measure

import (
	"context"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"go.uber.org/zap"
)

type (
	InfluxDB struct {
		data chan Metric
	}

	Metric struct {
		measurement string
		tags        map[string]string
		fields      map[string]interface{}
		ts          time.Time
	}
)

const (
	strUsage       = "usage"
	strPassword    = "password"
	strReadBytes   = "readbytes"
	strWriteBytes  = "writebytes"
	strRequests    = "requests"
	strThreads     = "threads"
	strOperation   = "operation"
	strUptime      = "uptime"
	strHealthCheck = "healthcheck"
	strMinute      = "minute"
)

var (
	Errors400BadRequest      = "proxy_errors_400"
	Errors403Forbidden       = "proxy_errors_403"
	Errors407AuthRequired    = "proxy_errors_407"
	Errors402PaymentRequired = "proxy_errors_402"
	Errors429TooManyRequests = "proxy_errors_429"
	Errors500Internal        = "proxy_errors_500"
	Errors502Internal        = "proxy_errors_502"
	Errors504GatewayTimeout  = "proxy_errors_504"
)

func NewInfluxDB(
	ctx context.Context,
	bufferSize int,
	org string,
	bucket string,
	client influxdb2.Client,
	metricPeriod time.Duration,
	healthCheckPeriod time.Duration,
	logger *zap.Logger,
) (*InfluxDB, error) {
	var data = make(chan Metric, bufferSize)
	go func() {
		metricTicker := time.NewTicker(metricPeriod)
		defer metricTicker.Stop()

		healthCheckTicker := time.NewTicker(healthCheckPeriod)
		defer healthCheckTicker.Stop()

		buf := []Metric{}
		for {
			select {
			case <-ctx.Done():
				return
			case <-metricTicker.C:
				if len(buf) == 0 {
					continue
				}

				writeAPI := client.WriteAPIBlocking(org, bucket)
				for _, metric := range buf {
					point := influxdb2.NewPoint(metric.measurement, metric.tags, metric.fields, metric.ts)
					if err := writeAPI.WritePoint(context.Background(), point); err != nil {
						logger.Error("failed to write data", zap.Error(err))
					}
				}
				buf = []Metric{}
			case d := <-data:
				buf = append(buf, d)
			case <-healthCheckTicker.C:
				tags := map[string]string{
					strUptime: strMinute,
				}
				fields := map[string]interface{}{
					strHealthCheck: 1,
				}

				writeAPI := client.WriteAPIBlocking(org, bucket)
				point := influxdb2.NewPoint(strOperation, tags, fields, time.Now())
				if err := writeAPI.WritePoint(context.Background(), point); err != nil {
					logger.Error("failed to write data", zap.Error(err))
				}
			}
		}
	}()

	return &InfluxDB{
		data: data,
	}, nil
}

func (i *InfluxDB) IncReadBytes(password string, bytes int64) error {
	return i.composeMetric(password, strReadBytes, bytes)
}

func (i *InfluxDB) IncWriteBytes(password string, bytes int64) error {
	return i.composeMetric(password, strWriteBytes, bytes)
}

func (i *InfluxDB) IncRequest(password string) error {
	return i.composeMetric(password, strRequests, 1)
}

func (i *InfluxDB) LogThreads(password string, threads int64) error {
	return i.composeMetric(password, strThreads, threads)
}

func (i *InfluxDB) CountError(password, err string) error {
	return i.composeMetric(password, err, 1)
}

func (i *InfluxDB) LogAdoptedFeature(password, feature string) error {
	return i.composeMetric(password, feature, 1)
}

func (i *InfluxDB) composeMetric(password string, field string, value int64) error {
	tags := map[string]string{
		strPassword: password,
	}
	fields := map[string]interface{}{
		field: value,
	}

	metric := Metric{
		measurement: strUsage,
		tags:        tags,
		fields:      fields,
		ts:          time.Now(),
	}

	i.data <- metric
	return nil
}
