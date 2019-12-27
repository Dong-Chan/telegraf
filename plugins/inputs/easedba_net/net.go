package easedba_net

import (
	"fmt"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/filter"
	"github.com/influxdata/telegraf/plugins/inputs"
	"github.com/influxdata/telegraf/plugins/inputs/system"
	"net"
	"strings"
	"time"
)

var statusMap = make(map[string]*Status)

type NetIOStats struct {
	filter filter.Filter
	ps     system.PS

	skipChecks          bool
	IgnoreProtocolStats bool
	Interfaces          []string
}

func (_ *NetIOStats) Description() string {
	return "Read metrics about network interface usage"
}

var netSampleConfig = `
  ## By default, telegraf gathers stats from any up interface (excluding loopback)
  ## Setting interfaces will tell it to gather these explicit interfaces,
  ## regardless of status.
  ##
  # interfaces = ["eth0"]
  ##
  ## On linux systems telegraf also collects protocol stats.
  ## Setting ignore_protocol_stats to true will skip reporting of protocol metrics.
  ##
  # ignore_protocol_stats = false
  ##
`

func (_ *NetIOStats) SampleConfig() string {
	return netSampleConfig
}

func (s *NetIOStats) Gather(acc telegraf.Accumulator) error {
	netio, err := s.ps.NetIO()
	if err != nil {
		return fmt.Errorf("error getting net io info: %s", err)
	}

	if s.filter == nil {
		if s.filter, err = filter.Compile(s.Interfaces); err != nil {
			return fmt.Errorf("error compiling filter: %s", err)
		}
	}

	interfaces, err := net.Interfaces()
	if err != nil {
		return fmt.Errorf("error getting list of interfaces: %s", err)
	}
	interfacesByName := map[string]net.Interface{}
	for _, iface := range interfaces {
		interfacesByName[iface.Name] = iface
	}

	for _, io := range netio {
		if len(s.Interfaces) != 0 {
			var found bool

			if s.filter.Match(io.Name) {
				found = true
			}

			if !found {
				continue
			}
		} else if !s.skipChecks {
			iface, ok := interfacesByName[io.Name]
			if !ok {
				continue
			}

			if iface.Flags&net.FlagLoopback == net.FlagLoopback {
				continue
			}

			if iface.Flags&net.FlagUp == 0 {
				continue
			}
		}

		tags := map[string]string{
			"interface": io.Name,
		}

		fields := map[string]interface{}{
			"bytes_sent":   fmt.Sprintf("%d",io.BytesSent),
			"bytes_recv":   fmt.Sprintf("%d",io.BytesRecv),
			"packets_sent": fmt.Sprintf("%d",io.PacketsSent),
			"packets_recv": fmt.Sprintf("%d",io.PacketsRecv),
			"err_in":       fmt.Sprintf("%d",io.Errin),
			"err_out":      fmt.Sprintf("%d",io.Errout),
			"drop_in":      fmt.Sprintf("%d",io.Dropin),
			"drop_out":     fmt.Sprintf("%d",io.Dropout),
		}

		s, ok := statusMap[io.Name]
		if ! ok {
			s = New(io.Name)
			statusMap[io.Name] = s
		}

		s.Fill(fields)

		adaptedFields := make(map[string]interface{})

		for k := range fields {
			v, err := s.GetPropertyDelta(k)
			if err != nil {
				continue
			}
			switch k {
			case "err_in", "err_out", "drop_in", "drop_out":
				adaptedFields[k] = v
			case "bytes_sent", "bytes_recv", "packets_sent", "packets_recv":
				adaptedFields[k] = v / (s.CurrTime.Sub(s.LastTime).Nanoseconds() / int64(time.Second))
			}
		}

		acc.AddCounter("net", adaptedFields, tags)
	}

	// Get system wide stats for different network protocols
	// (ignore these stats if the call fails)
	if !s.IgnoreProtocolStats {
		netprotos, _ := s.ps.NetProto()
		fields := make(map[string]interface{})
		for _, proto := range netprotos {
			for stat, value := range proto.Stats {
				name := fmt.Sprintf("%s_%s", strings.ToLower(proto.Protocol),
					strings.ToLower(stat))
				fields[name] = value
			}
		}
		tags := map[string]string{
			"interface": "all",
		}
		acc.AddFields("net", fields, tags)
	}

	return nil
}

func init() {
	inputs.Add("easedba_net", func() telegraf.Input {
		return &NetIOStats{ps: system.NewSystemPS()}
	})
}
