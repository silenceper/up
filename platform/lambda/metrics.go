package lambda

import (
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/golang/sync/errgroup"

	"github.com/apex/up/internal/metrics"
	"github.com/apex/up/platform/event"
)

// TODO: write a higher level pkg in tj/aws
// TODO: move the metrics pkg to tj/aws

type stat struct {
	Namespace string
	Name      string
	Metric    string
	Stat      string
	point     *cloudwatch.Datapoint
}

// Value returns the metric value.
func (s *stat) Value() int {
	if s.point == nil {
		return 0
	}

	switch s.Stat {
	case "Sum":
		return int(*s.point.Sum)
	case "Average":
		return int(*s.point.Average)
	case "Minimum":
		return int(*s.point.Minimum)
	case "Maximum":
		return int(*s.point.Maximum)
	default:
		return 0
	}
}

// stats to fetch.
var stats = []*stat{
	{"AWS/ApiGateway", "Requests", "Count", "Sum", nil},
	{"AWS/ApiGateway", "Min Latency", "Latency", "Minimum", nil},
	{"AWS/ApiGateway", "Avg Latency", "Latency", "Average", nil},
	{"AWS/ApiGateway", "Max Latency", "Latency", "Maximum", nil},
	{"AWS/ApiGateway", "Client Errors", "4XXError", "Sum", nil},
	{"AWS/ApiGateway", "Server Errors", "5XXError", "Sum", nil},
	{"AWS/Lambda", "Lambda Errors", "Errors", "Sum", nil},
	{"AWS/Lambda", "Lambda Throttles", "Throttles", "Sum", nil},
}

// ShowMetrics implementation.
func (p *Platform) ShowMetrics(region, stage string, start time.Time) error {
	s := session.New(aws.NewConfig().WithRegion(region))
	c := cloudwatch.New(s)
	var g errgroup.Group

	d := time.Now().Sub(start)
	period := int(d / time.Second)

	for _, s := range stats {
		s := s
		g.Go(func() error {
			m := metrics.New().
				Namespace(s.Namespace).
				TimeRange(time.Now().Add(-d), time.Now()).
				Period(period).
				Stat(s.Stat).
				Metric(s.Metric)

			switch s.Namespace {
			case "AWS/ApiGateway":
				m = m.Dimension("ApiName", p.config.Name).Dimension("Stage", stage)
			case "AWS/Lambda":
				m = m.Dimension("FunctionName", p.config.Name).Dimension("Alias", stage)
			}

			res, err := c.GetMetricStatistics(m.Params())
			if err != nil {
				return err
			}

			if len(res.Datapoints) > 0 {
				s.point = res.Datapoints[0]
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	for _, s := range stats {
		p.events.Emit("metrics.value", event.Fields{
			"name":  s.Name,
			"value": s.Value(),
		})
	}

	return nil
}
