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
	strBandwidth  = "bandwidth"
	strPassword   = "password"
	strReadBytes  = "readbytes"
	strWriteBytes = "writebytes"
)

func NewInfluxDB(
	ctx context.Context,
	bufferSize int,
	org string,
	bucket string,
	client influxdb2.Client,
	period time.Duration,
	logger *zap.Logger,
) (*InfluxDB, error) {
	var data = make(chan Metric, bufferSize)
	go func() {
		ticker := time.NewTicker(period)
		defer ticker.Stop()

		buf := []Metric{}

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
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
			}
		}
	}()

	return &InfluxDB{
		data: data,
	}, nil
}

func (i *InfluxDB) IncReadBytes(password string, bytes int64) error {
	tags := map[string]string{
		strPassword: password,
	}
	fields := map[string]interface{}{
		strReadBytes: bytes,
	}

	metric := Metric{
		measurement: strBandwidth,
		tags:        tags,
		fields:      fields,
		ts:          time.Now(),
	}

	i.data <- metric
	return nil
}

func (i *InfluxDB) IncWriteBytes(password string, bytes int64) error {
	tags := map[string]string{
		strPassword: password,
	}
	fields := map[string]interface{}{
		strWriteBytes: bytes,
	}

	metric := Metric{
		measurement: strBandwidth,
		tags:        tags,
		fields:      fields,
		ts:          time.Now(),
	}

	i.data <- metric
	return nil
}
