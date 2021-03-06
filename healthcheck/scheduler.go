package healthcheck

import (
	"context"
	"fmt"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"

	"github.com/robfig/cron/v3"

	"github.com/SotirisAlfonsos/chaos-bot/proto"

	"github.com/SotirisAlfonsos/chaos-master/network"
)

type HealthChecker struct {
	DetailsMap map[string]*Details
	logger     log.Logger
}

type Details struct {
	Status     proto.HealthCheckResponse_ServingStatus
	connection network.Connection
}

func Register(connections *network.Connections, logger log.Logger) *HealthChecker {
	healthChecker := &HealthChecker{DetailsMap: make(map[string]*Details), logger: logger}
	for target, connection := range connections.Pool {
		healthChecker.DetailsMap[target] = &Details{
			Status:     proto.HealthCheckResponse_UNKNOWN,
			connection: connection,
		}
	}

	return healthChecker
}

func (hch *HealthChecker) Start(report bool) {
	c := cron.New()
	id, err := c.AddFunc("@every 1m", func() {
		for target, details := range hch.DetailsMap {
			client, err := details.connection.GetHealthClient(target)
			if err != nil {
				_ = level.Error(hch.logger).Log(
					"msg", fmt.Sprintf("Can not get healthcheck connection for target {%s}", target),
					"err", err)
				continue
			}

			resp, err := client.Check(context.Background(), &proto.HealthCheckRequest{})
			if err != nil {
				_ = level.Error(hch.logger).Log(
					"msg", fmt.Sprintf("Failed to get valid response when health-checking target %s", target),
					"err", err)
				hch.DetailsMap[target].Status = proto.HealthCheckResponse_NOT_SERVING
			} else {
				hch.DetailsMap[target].Status = resp.Status
			}
		}

		_ = level.Debug(hch.logger).Log("msg", "checking status of bots")

		if report {
			for target, details := range hch.DetailsMap {
				_ = level.Info(hch.logger).Log("msg", fmt.Sprintf("Status of bot %s is %s", target, details.Status))
			}
		}
	})

	if err != nil {
		_ = level.Error(hch.logger).Log("msg", "could not create scheduling task for automated health-checks")
	}

	_ = level.Info(hch.logger).Log("msg", fmt.Sprintf("starting automated health-check scheduler with id %d", id))

	c.Start()
}
