// +build linux

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/intelrdt"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// event struct for encoding the event data to json.
type event struct {
	Type string      `json:"type"`
	ID   string      `json:"id"`
	Data interface{} `json:"data,omitempty"`
}

// stats is the runc specific stats structure for stability when encoding and decoding stats.
type stats struct {
	CPU      cpu                `json:"cpu"`
	Memory   memory             `json:"memory"`
	Pids     pids               `json:"pids"`
	Blkio    blkio              `json:"blkio"`
	Hugetlb  map[string]hugetlb `json:"hugetlb"`
	IntelRdt intelRdt           `json:"intel_rdt"`
}

type hugetlb struct {
	Usage   uint64 `json:"usage,omitempty"`
	Max     uint64 `json:"max,omitempty"`
	Failcnt uint64 `json:"failcnt"`
}

type blkioEntry struct {
	Major uint64 `json:"major,omitempty"`
	Minor uint64 `json:"minor,omitempty"`
	Op    string `json:"op,omitempty"`
	Value uint64 `json:"value,omitempty"`
}

type blkio struct {
	IoServiceBytesRecursive []blkioEntry `json:"ioServiceBytesRecursive,omitempty"`
	IoServicedRecursive     []blkioEntry `json:"ioServicedRecursive,omitempty"`
	IoQueuedRecursive       []blkioEntry `json:"ioQueueRecursive,omitempty"`
	IoServiceTimeRecursive  []blkioEntry `json:"ioServiceTimeRecursive,omitempty"`
	IoWaitTimeRecursive     []blkioEntry `json:"ioWaitTimeRecursive,omitempty"`
	IoMergedRecursive       []blkioEntry `json:"ioMergedRecursive,omitempty"`
	IoTimeRecursive         []blkioEntry `json:"ioTimeRecursive,omitempty"`
	SectorsRecursive        []blkioEntry `json:"sectorsRecursive,omitempty"`
}

type pids struct {
	Current uint64 `json:"current,omitempty"`
	Limit   uint64 `json:"limit,omitempty"`
}

type throttling struct {
	Periods          uint64 `json:"periods,omitempty"`
	ThrottledPeriods uint64 `json:"throttledPeriods,omitempty"`
	ThrottledTime    uint64 `json:"throttledTime,omitempty"`
}

type cpuUsage struct {
	// Units: nanoseconds.
	Total  uint64   `json:"total,omitempty"`
	Percpu []uint64 `json:"percpu,omitempty"`
	Kernel uint64   `json:"kernel"`
	User   uint64   `json:"user"`
}

type cpu struct {
	Usage      cpuUsage   `json:"usage,omitempty"`
	Throttling throttling `json:"throttling,omitempty"`
}

type memoryEntry struct {
	Limit   uint64 `json:"limit"`
	Usage   uint64 `json:"usage,omitempty"`
	Max     uint64 `json:"max,omitempty"`
	Failcnt uint64 `json:"failcnt"`
}

type memory struct {
	Cache     uint64            `json:"cache,omitempty"`
	Usage     memoryEntry       `json:"usage,omitempty"`
	Swap      memoryEntry       `json:"swap,omitempty"`
	Kernel    memoryEntry       `json:"kernel,omitempty"`
	KernelTCP memoryEntry       `json:"kernelTCP,omitempty"`
	Raw       map[string]uint64 `json:"raw,omitempty"`
}

type l3CacheInfo struct {
	CbmMask    string `json:"cbm_mask,omitempty"`
	MinCbmBits uint64 `json:"min_cbm_bits,omitempty"`
	NumClosids uint64 `json:"num_closids,omitempty"`
}

type memBwInfo struct {
	BandwidthGran uint64 `json:"bandwidth_gran,omitempty"`
	DelayLinear   uint64 `json:"delay_linear,omitempty"`
	MinBandwidth  uint64 `json:"min_bandwidth,omitempty"`
	NumClosids    uint64 `json:"num_closids,omitempty"`
}

type intelRdt struct {
	// The read-only L3 cache information
	L3CacheInfo *l3CacheInfo `json:"l3_cache_info,omitempty"`

	// The read-only L3 cache schema in root
	L3CacheSchemaRoot string `json:"l3_cache_schema_root,omitempty"`

	// The L3 cache schema in 'container_id' group
	L3CacheSchema string `json:"l3_cache_schema,omitempty"`

	// The read-only memory bandwidth information
	MemBwInfo *memBwInfo `json:"mem_bw_info,omitempty"`

	// The read-only memory bandwidth schema in root
	MemBwSchemaRoot string `json:"mem_bw_schema_root,omitempty"`

	// The memory bandwidth schema in 'container_id' group
	MemBwSchema string `json:"mem_bw_schema,omitempty"`
}

var eventsCommand = cli.Command{
	Name:  "events",
	Usage: "display container events such as OOM notifications, cpu, memory, and IO usage statistics",
	ArgsUsage: `<container-id>

Where "<container-id>" is the name for the instance of the container.`,
	Description: `The events command displays information about the container. By default the
information is displayed once every 5 seconds.`,
	Flags: []cli.Flag{
		cli.DurationFlag{Name: "interval", Value: 5 * time.Second, Usage: "set the stats collection interval"},
		cli.BoolFlag{Name: "stats", Usage: "display the container's stats then exit"},
	},
	Action: func(context *cli.Context) error {
		if err := checkArgs(context, 1, exactArgs); err != nil {
			return err
		}
		container, err := getContainer(context)
		if err != nil {
			return err
		}
		duration := context.Duration("interval")
		if duration <= 0 {
			return fmt.Errorf("duration interval must be greater than 0")
		}
		status, err := container.Status()
		if err != nil {
			return err
		}
		if status == libcontainer.Stopped {
			return fmt.Errorf("container with id %s is not running", container.ID())
		}
		var (
			stats  = make(chan *libcontainer.Stats, 1)
			events = make(chan *event, 1024)
			group  = &sync.WaitGroup{}
		)
		group.Add(1)
		go func() {
			defer group.Done()
			enc := json.NewEncoder(os.Stdout)
			for e := range events {
				if err := enc.Encode(e); err != nil {
					logrus.Error(err)
				}
			}
		}()
		if context.Bool("stats") {
			s, err := container.Stats()
			if err != nil {
				return err
			}
			events <- &event{Type: "stats", ID: container.ID(), Data: convertLibcontainerStats(s)}
			close(events)
			group.Wait()
			return nil
		}
		go func() {
			for range time.Tick(context.Duration("interval")) {
				s, err := container.Stats()
				if err != nil {
					logrus.Error(err)
					continue
				}
				stats <- s
			}
		}()
		n, err := container.NotifyOOM()
		if err != nil {
			return err
		}
		for {
			select {
			case _, ok := <-n:
				if ok {
					// this means an oom event was received, if it is !ok then
					// the channel was closed because the container stopped and
					// the cgroups no longer exist.
					events <- &event{Type: "oom", ID: container.ID()}
				} else {
					n = nil
				}
			case s := <-stats:
				events <- &event{Type: "stats", ID: container.ID(), Data: convertLibcontainerStats(s)}
			}
			if n == nil {
				close(events)
				break
			}
		}
		group.Wait()
		return nil
	},
}

func convertLibcontainerStats(ls *libcontainer.Stats) *stats {
	cg := ls.CgroupStats
	if cg == nil {
		return nil
	}
	var s stats
	s.Pids.Current = cg.PidsStats.Current
	s.Pids.Limit = cg.PidsStats.Limit

	s.CPU.Usage.Kernel = cg.CpuStats.CpuUsage.UsageInKernelmode
	s.CPU.Usage.User = cg.CpuStats.CpuUsage.UsageInUsermode
	s.CPU.Usage.Total = cg.CpuStats.CpuUsage.TotalUsage
	s.CPU.Usage.Percpu = cg.CpuStats.CpuUsage.PercpuUsage
	s.CPU.Throttling.Periods = cg.CpuStats.ThrottlingData.Periods
	s.CPU.Throttling.ThrottledPeriods = cg.CpuStats.ThrottlingData.ThrottledPeriods
	s.CPU.Throttling.ThrottledTime = cg.CpuStats.ThrottlingData.ThrottledTime

	s.Memory.Cache = cg.MemoryStats.Cache
	s.Memory.Kernel = convertMemoryEntry(cg.MemoryStats.KernelUsage)
	s.Memory.KernelTCP = convertMemoryEntry(cg.MemoryStats.KernelTCPUsage)
	s.Memory.Swap = convertMemoryEntry(cg.MemoryStats.SwapUsage)
	s.Memory.Usage = convertMemoryEntry(cg.MemoryStats.Usage)
	s.Memory.Raw = cg.MemoryStats.Stats

	s.Blkio.IoServiceBytesRecursive = convertBlkioEntry(cg.BlkioStats.IoServiceBytesRecursive)
	s.Blkio.IoServicedRecursive = convertBlkioEntry(cg.BlkioStats.IoServicedRecursive)
	s.Blkio.IoQueuedRecursive = convertBlkioEntry(cg.BlkioStats.IoQueuedRecursive)
	s.Blkio.IoServiceTimeRecursive = convertBlkioEntry(cg.BlkioStats.IoServiceTimeRecursive)
	s.Blkio.IoWaitTimeRecursive = convertBlkioEntry(cg.BlkioStats.IoWaitTimeRecursive)
	s.Blkio.IoMergedRecursive = convertBlkioEntry(cg.BlkioStats.IoMergedRecursive)
	s.Blkio.IoTimeRecursive = convertBlkioEntry(cg.BlkioStats.IoTimeRecursive)
	s.Blkio.SectorsRecursive = convertBlkioEntry(cg.BlkioStats.SectorsRecursive)

	s.Hugetlb = make(map[string]hugetlb)
	for k, v := range cg.HugetlbStats {
		s.Hugetlb[k] = convertHugtlb(v)
	}

	if is := ls.IntelRdtStats; is != nil {
		if intelrdt.IsCatEnabled() {
			s.IntelRdt.L3CacheInfo = convertL3CacheInfo(is.L3CacheInfo)
			s.IntelRdt.L3CacheSchemaRoot = is.L3CacheSchemaRoot
			s.IntelRdt.L3CacheSchema = is.L3CacheSchema
		}
		if intelrdt.IsMbaEnabled() {
			s.IntelRdt.MemBwInfo = convertMemBwInfo(is.MemBwInfo)
			s.IntelRdt.MemBwSchemaRoot = is.MemBwSchemaRoot
			s.IntelRdt.MemBwSchema = is.MemBwSchema
		}
	}

	return &s
}

func convertHugtlb(c cgroups.HugetlbStats) hugetlb {
	return hugetlb{
		Usage:   c.Usage,
		Max:     c.MaxUsage,
		Failcnt: c.Failcnt,
	}
}

func convertMemoryEntry(c cgroups.MemoryData) memoryEntry {
	return memoryEntry{
		Limit:   c.Limit,
		Usage:   c.Usage,
		Max:     c.MaxUsage,
		Failcnt: c.Failcnt,
	}
}

func convertBlkioEntry(c []cgroups.BlkioStatEntry) []blkioEntry {
	var out []blkioEntry
	for _, e := range c {
		out = append(out, blkioEntry{
			Major: e.Major,
			Minor: e.Minor,
			Op:    e.Op,
			Value: e.Value,
		})
	}
	return out
}

func convertL3CacheInfo(i *intelrdt.L3CacheInfo) *l3CacheInfo {
	return &l3CacheInfo{
		CbmMask:    i.CbmMask,
		MinCbmBits: i.MinCbmBits,
		NumClosids: i.NumClosids,
	}
}

func convertMemBwInfo(i *intelrdt.MemBwInfo) *memBwInfo {
	return &memBwInfo{
		BandwidthGran: i.BandwidthGran,
		DelayLinear:   i.DelayLinear,
		MinBandwidth:  i.MinBandwidth,
		NumClosids:    i.NumClosids,
	}
}
